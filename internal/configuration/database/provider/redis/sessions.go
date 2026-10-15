package redis

import (
	"errors"
	"time"

	"github.com/forceu/gokapi/internal/helper"
	"github.com/forceu/gokapi/internal/models"
	redigo "github.com/gomodule/redigo/redis"
)

const (
	prefixSessions  = "se:"
	prefixRotations = "ro:"
)

// maxRotationHops limits how many rotation pointers are followed when looking
// up a successor or deleting a rotation chain
const maxRotationHops = 5

// luaRotateSession atomically replaces a session with a single successor.
// KEYS[1] = rotation record (ro:<oldId>), KEYS[2] = old session, KEYS[3] = new session
// ARGV[1] = new id, ARGV[2] = renew_at, ARGV[3] = valid_until, ARGV[4] = user_id
// Returns {1, <successor id>} on success (existing successor if already rotated)
// or {0} if the old session does not exist anymore (revoked/expired).
const luaRotateSession = `
local existing = redis.call('GET', KEYS[1])
if existing then
	return {1, existing}
end
if redis.call('EXISTS', KEYS[2]) == 0 then
	return {0}
end
redis.call('HSET', KEYS[3], 'renew_at', ARGV[2], 'valid_until', ARGV[3], 'user_id', ARGV[4])
redis.call('EXPIREAT', KEYS[3], ARGV[3])
redis.call('SET', KEYS[1], ARGV[1])
redis.call('EXPIREAT', KEYS[1], ARGV[3])
redis.call('DEL', KEYS[2])
return {1, ARGV[1]}
`

// luaDeleteSessionChain deletes a session together with every successor of its
// rotation chain, then removes rotation records pointing to deleted sessions.
// ARGV[1] = key prefix, ARGV[2] = session id
const luaDeleteSessionChain = `
local prefix, startId = ARGV[1], ARGV[2]
local current = startId
for i = 1, 6 do
	redis.call('DEL', prefix .. 'se:' .. current)
	local nextId = redis.call('GET', prefix .. 'ro:' .. current)
	redis.call('DEL', prefix .. 'ro:' .. current)
	if not nextId then
		break
	end
	current = nextId
end
for pass = 1, 6 do
	local deletedAny = false
	for _, k in ipairs(redis.call('KEYS', prefix .. 'ro:*')) do
		local target = redis.call('GET', k)
		if (not target) or (redis.call('EXISTS', prefix .. 'se:' .. target) == 0) then
			redis.call('DEL', k)
			deletedAny = true
		end
	end
	if not deletedAny then
		break
	end
end
return 1
`

// luaDeleteSessionsByUser atomically deletes all sessions of a user, including
// any rotation successors, and prunes the affected rotation records. Because
// the script runs atomically, a concurrent rotation either committed fully
// before it (its successor is deleted) or runs after it (the predecessor is
// missing, so the rotation returns {0} without creating a successor).
// ARGV[1] = key prefix, ARGV[2] = user id
const luaDeleteSessionsByUser = `
local prefix, userId = ARGV[1], ARGV[2]
for _, key in ipairs(redis.call('KEYS', prefix .. 'se:*')) do
	if redis.call('HGET', key, 'user_id') == userId then
		local id = string.sub(key, string.len(prefix .. 'se:') + 1)
		local current = id
		for hop = 1, 6 do
			redis.call('DEL', prefix .. 'se:' .. current)
			local nextId = redis.call('GET', prefix .. 'ro:' .. current)
			redis.call('DEL', prefix .. 'ro:' .. current)
			if not nextId then
				break
			end
			current = nextId
		end
	end
end
for pass = 1, 6 do
	local deletedAny = false
	for _, k in ipairs(redis.call('KEYS', prefix .. 'ro:*')) do
		local target = redis.call('GET', k)
		if (not target) or (redis.call('EXISTS', prefix .. 'se:' .. target) == 0) then
			redis.call('DEL', k)
			deletedAny = true
		end
	end
	if not deletedAny then
		break
	end
end
return 1
`

// GetSession returns the session with the given ID or false if not a valid ID
func (p DatabaseProvider) GetSession(id string) (models.Session, bool) {
	hashmapEntry, ok := p.getHashMap(prefixSessions + id)
	if !ok {
		return models.Session{}, false
	}
	var result models.Session
	err := redigo.ScanStruct(hashmapEntry, &result)
	helper.Check(err)
	return result, true
}

// SaveSession stores the given session. After the expiry passed, it will be deleted automatically
func (p DatabaseProvider) SaveSession(id string, session models.Session) {
	p.setHashMap(p.buildArgs(prefixSessions + id).AddFlat(session))
	p.setExpiryAt(prefixSessions+id, session.ValidUntil)
}

// RotateSession atomically replaces oldId with the single successor newId.
// Concurrent invocations from different instances are serialized by the Redis
// server executing the Lua script.
func (p DatabaseProvider) RotateSession(oldId string, newId string, newSession models.Session) (string, bool) {
	conn := p.pool.Get()
	defer conn.Close()
	reply, err := conn.Do("EVAL", luaRotateSession, 3,
		p.dbPrefix+prefixRotations+oldId,
		p.dbPrefix+prefixSessions+oldId,
		p.dbPrefix+prefixSessions+newId,
		newId, newSession.RenewAt, newSession.ValidUntil, newSession.UserId)
	if err != nil {
		helper.Check(err)
		return "", false
	}
	values, err := redigo.Values(reply, nil)
	if err != nil {
		helper.Check(err)
		return "", false
	}
	status, _ := redigo.Int(values[0], nil)
	if status == 0 {
		return "", false
	}
	successor, err := redigo.String(values[1], nil)
	if err != nil {
		helper.Check(err)
		return "", false
	}
	return successor, true
}

// GetSessionSuccessor follows the rotation pointer of an ID that has already
// been replaced and returns its currently valid successor.
func (p DatabaseProvider) GetSessionSuccessor(oldId string) (string, models.Session, bool) {
	conn := p.pool.Get()
	defer conn.Close()
	currentId := oldId
	visited := map[string]bool{oldId: true}
	for i := 0; i < maxRotationHops; i++ {
		nextId, err := redigo.String(conn.Do("GET", p.dbPrefix+prefixRotations+currentId))
		if errors.Is(err, redigo.ErrNil) {
			return "", models.Session{}, false
		}
		if err != nil {
			helper.Check(err)
			return "", models.Session{}, false
		}
		session, ok := p.GetSession(nextId)
		if ok && session.ValidUntil >= time.Now().Unix() {
			return nextId, session, true
		}
		// Successor may have been rotated again - follow the chain
		if visited[nextId] {
			return "", models.Session{}, false
		}
		visited[nextId] = true
		currentId = nextId
	}
	return "", models.Session{}, false
}

// DeleteSession deletes a session with the given ID, along with the entire
// successor chain created by rotations. This ensures that revocations also
// invalidate successor sessions that concurrent renewals may have created.
func (p DatabaseProvider) DeleteSession(id string) {
	conn := p.pool.Get()
	defer conn.Close()
	_, err := conn.Do("EVAL", luaDeleteSessionChain, 0, p.dbPrefix, id)
	helper.Check(err)
}

// DeleteAllSessions logs all users out
func (p DatabaseProvider) DeleteAllSessions() {
	p.deleteAllWithPrefix(prefixSessions)
	p.deleteAllWithPrefix(prefixRotations)
}

// DeleteAllSessionsByUser logs the specific users out
func (p DatabaseProvider) DeleteAllSessionsByUser(userId int) {
	conn := p.pool.Get()
	defer conn.Close()
	_, err := conn.Do("EVAL", luaDeleteSessionsByUser, 0, p.dbPrefix, userId)
	helper.Check(err)
}
