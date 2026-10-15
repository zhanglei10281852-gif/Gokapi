package redis

import (
	"strconv"
	"time"

	"github.com/forceu/gokapi/internal/helper"
	redigo "github.com/gomodule/redigo/redis"
)

const (
	// prefixChunkResActive contains a sorted set per file request, mapping the reservation
	// uuid (member) to its expiry timestamp (score) for active reservations
	prefixChunkResActive = "crq:"
	// prefixChunkResFinal contains finalised reservations, retained until their expiry
	// so that duplicate or late complete requests remain idempotent
	prefixChunkResFinal = "crf:"
	// prefixChunkResFiles maps a finalised reservation uuid to the id of the created file
	prefixChunkResFiles = "crfd:"
	// prefixRequestFiles contains the set of completed file IDs per file request,
	// used to atomically enforce the shared file quota
	prefixRequestFiles = "frf:"
)

// scriptCreateReservation atomically reclaims expired reservations, checks the
// shared quota (completed files + active reservations) and adds the reservation
var scriptCreateReservation = `
redis.call('ZREMRANGEBYSCORE', KEYS[1], '-inf', ARGV[3])
if tonumber(ARGV[4]) > 0 then
	local active = redis.call('ZCARD', KEYS[1])
	local uploaded = redis.call('SCARD', KEYS[2])
	if active + uploaded >= tonumber(ARGV[4]) then
		return 0
	end
end
redis.call('ZADD', KEYS[1], ARGV[2], ARGV[1])
return 1
`

// scriptSetUploading verifies that an active, unexpired reservation exists and
// extends its expiry
var scriptSetUploading = `
local score = redis.call('ZSCORE', KEYS[1], ARGV[1])
if not score or tonumber(score) <= tonumber(ARGV[3]) then
	return 0
end
redis.call('ZADD', KEYS[1], ARGV[2], ARGV[1])
return 1
`

// scriptFinaliseReservation moves an active reservation to the finalised set and
// records the file id. If the reservation was already finalised, the stored file
// id is returned instead.
var scriptFinaliseReservation = `
if redis.call('ZREM', KEYS[1], ARGV[1]) == 1 then
	redis.call('ZADD', KEYS[2], ARGV[2], ARGV[1])
	redis.call('HSET', KEYS[3], ARGV[1], ARGV[4])
	return 1
end
local existing = redis.call('HGET', KEYS[3], ARGV[1])
local score = redis.call('ZSCORE', KEYS[2], ARGV[1])
if not existing or not score or tonumber(score) <= tonumber(ARGV[3]) then
	return 0
end
return existing
`

func (p DatabaseProvider) runReservationScript(script string, keys []string, args ...any) (any, error) {
	conn := p.pool.Get()
	defer conn.Close()
	scriptArgs := redigo.Args{}.Add(script).Add(len(keys))
	for _, key := range keys {
		scriptArgs = scriptArgs.Add(p.dbPrefix + key)
	}
	scriptArgs = scriptArgs.Add(args...)
	return conn.Do("EVAL", scriptArgs...)
}

// CreateChunkReservation stores a reservation for an upcoming chunk upload.
// If maxFiles is greater than zero, the creation is atomic and fails when the
// number of completed uploads and valid reservations has reached maxFiles.
func (p DatabaseProvider) CreateChunkReservation(requestId, uuid string, expiry int64, maxFiles int) bool {
	if requestId == "" || uuid == "" {
		return false
	}
	reply, err := p.runReservationScript(scriptCreateReservation,
		[]string{prefixChunkResActive + requestId, prefixRequestFiles + requestId},
		uuid, expiry, time.Now().Unix()-1, maxFiles)
	if err != nil {
		helper.Check(err)
		return false
	}
	result, err := redigo.Int(reply, nil)
	if err != nil {
		helper.Check(err)
		return false
	}
	return result == 1
}

// SetChunkReservationUploading marks a reservation as uploading and extends its expiry.
func (p DatabaseProvider) SetChunkReservationUploading(requestId, uuid string, newExpiry int64) bool {
	reply, err := p.runReservationScript(scriptSetUploading,
		[]string{prefixChunkResActive + requestId},
		uuid, newExpiry, time.Now().Unix()-1)
	if err != nil {
		helper.Check(err)
		return false
	}
	result, err := redigo.Int(reply, nil)
	if err != nil {
		helper.Check(err)
		return false
	}
	return result == 1
}

// ReleaseChunkReservation removes a reservation. The operation is idempotent.
func (p DatabaseProvider) ReleaseChunkReservation(requestId, uuid string) {
	if requestId == "" || uuid == "" {
		return
	}
	conn := p.pool.Get()
	defer conn.Close()
	_, err := conn.Do("ZREM", p.dbPrefix+prefixChunkResActive+requestId, uuid)
	helper.Check(err)
	_, err = conn.Do("ZREM", p.dbPrefix+prefixChunkResFinal+requestId, uuid)
	helper.Check(err)
	_, err = conn.Do("HDEL", p.dbPrefix+prefixChunkResFiles+requestId, uuid)
	helper.Check(err)
}

// FinaliseChunkReservation marks a reservation as completed and stores the
// created file ID. If the reservation was already finalised, the existing file
// ID is returned.
func (p DatabaseProvider) FinaliseChunkReservation(requestId, uuid, fileId string, expiry int64) (string, bool) {
	reply, err := p.runReservationScript(scriptFinaliseReservation,
		[]string{
			prefixChunkResActive + requestId,
			prefixChunkResFinal + requestId,
			prefixChunkResFiles + requestId,
		},
		uuid, expiry, time.Now().Unix()-1, fileId)
	if err != nil {
		helper.Check(err)
		return "", false
	}
	switch result := reply.(type) {
	case int64:
		return "", result == 1
	case string:
		return result, true
	case []byte:
		return string(result), true
	default:
		return "", false
	}
}

// GetFinalisedChunkReservation returns the file ID of a finalised, unexpired reservation
func (p DatabaseProvider) GetFinalisedChunkReservation(requestId, uuid string) (string, bool) {
	conn := p.pool.Get()
	defer conn.Close()
	score, err := redigo.Int64(conn.Do("ZSCORE", p.dbPrefix+prefixChunkResFinal+requestId, uuid))
	if err != nil || score < time.Now().Unix() {
		return "", false
	}
	fileId, err := redigo.String(conn.Do("HGET", p.dbPrefix+prefixChunkResFiles+requestId, uuid))
	if err != nil || fileId == "" {
		return "", false
	}
	return fileId, true
}

// CountActiveChunkReservations returns the number of active, unexpired reservations
func (p DatabaseProvider) CountActiveChunkReservations(requestId string) int {
	conn := p.pool.Get()
	defer conn.Close()
	key := p.dbPrefix + prefixChunkResActive + requestId
	_, err := conn.Do("ZREMRANGEBYSCORE", key, "-inf", strconv.FormatInt(time.Now().Unix()-1, 10))
	helper.Check(err)
	count, err := redigo.Int(conn.Do("ZCARD", key))
	if err != nil {
		helper.Check(err)
		return 0
	}
	return count
}

// DeleteAllChunkReservations removes all reservations associated with a file request
func (p DatabaseProvider) DeleteAllChunkReservations(requestId string) {
	if requestId == "" {
		return
	}
	conn := p.pool.Get()
	defer conn.Close()
	_, err := conn.Do("DEL",
		p.dbPrefix+prefixChunkResActive+requestId,
		p.dbPrefix+prefixChunkResFinal+requestId,
		p.dbPrefix+prefixChunkResFiles+requestId,
		p.dbPrefix+prefixRequestFiles+requestId)
	helper.Check(err)
}

// DeleteExpiredChunkReservations reclaims all reservations that have expired
func (p DatabaseProvider) DeleteExpiredChunkReservations() {
	nowMinusOne := strconv.FormatInt(time.Now().Unix()-1, 10)
	p.cleanExpiredReservationSet(prefixChunkResActive, nowMinusOne, false)
	p.cleanExpiredReservationSet(prefixChunkResFinal, nowMinusOne, true)
}

// cleanExpiredReservationSet removes expired members from all reservation sets
// with the given prefix. Empty sets are deleted. If removeFileMap is true, the
// corresponding finalised-uuid entries and the file-id hash are cleaned as well.
func (p DatabaseProvider) cleanExpiredReservationSet(prefix, nowMinusOne string, removeFileMap bool) {
	conn := p.pool.Get()
	defer conn.Close()
	for _, key := range p.getAllKeysWithPrefix(prefix) {
		fullKey := p.dbPrefix + key
		expiredMembers, err := redigo.Strings(conn.Do("ZRANGEBYSCORE", fullKey, "-inf", nowMinusOne))
		helper.Check(err)
		_, err = conn.Do("ZREMRANGEBYSCORE", fullKey, "-inf", nowMinusOne)
		helper.Check(err)
		fileMapKey := p.dbPrefix + prefixChunkResFiles + key[len(prefix):]
		if removeFileMap {
			for _, member := range expiredMembers {
				_, _ = conn.Do("HDEL", fileMapKey, member)
			}
		}
		count, err := redigo.Int(conn.Do("ZCARD", fullKey))
		helper.Check(err)
		if count == 0 {
			if removeFileMap {
				_, _ = conn.Do("DEL", fullKey, fileMapKey)
			} else {
				_, _ = conn.Do("DEL", fullKey)
			}
		}
	}
}

// addRequestFile indexes a completed upload for the shared file quota
func (p DatabaseProvider) addRequestFile(requestId, fileId string) {
	if requestId == "" || fileId == "" {
		return
	}
	conn := p.pool.Get()
	defer conn.Close()
	_, err := conn.Do("SADD", p.dbPrefix+prefixRequestFiles+requestId, fileId)
	helper.Check(err)
}

// removeRequestFile removes a completed upload from the shared file quota index
func (p DatabaseProvider) removeRequestFile(requestId, fileId string) {
	if requestId == "" || fileId == "" {
		return
	}
	conn := p.pool.Get()
	defer conn.Close()
	_, err := conn.Do("SREM", p.dbPrefix+prefixRequestFiles+requestId, fileId)
	helper.Check(err)
}
