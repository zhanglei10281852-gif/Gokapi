package redis

import (
	"strconv"

	"github.com/forceu/gokapi/internal/helper"
	redigo "github.com/gomodule/redigo/redis"
)

const (
	prefixStatisticsTraffic      = "stats:traffic"
	prefixStatisticsTrafficSince = "stats:traffic_since"
)

// trafficAddScript atomically adds ARGV[1] bytes to the traffic counter
// (KEYS[2]), but only if the stored beginning of traffic counting (KEYS[1])
// equals the epoch supplied in ARGV[2]. If the statistics have been reset and
// the epoch differs, the amount is discarded. It always returns the stored
// total and the stored epoch
var trafficAddScript = `
local since = redis.call('GET', KEYS[1])
if since == false then
	redis.call('SET', KEYS[1], ARGV[2])
	since = ARGV[2]
end
local total
if tonumber(since) == tonumber(ARGV[2]) then
	total = redis.call('INCRBY', KEYS[2], ARGV[1])
else
	total = redis.call('GET', KEYS[2])
	if total == false then
		total = 0
	end
end
return {total, since}`

// GetStatTraffic returns the total traffic from statistics
func (p DatabaseProvider) GetStatTraffic() uint64 {
	result, _ := p.getKeyUInt64(prefixStatisticsTraffic)
	return result
}

// AddStatTraffic atomically adds the given amount of traffic, but only if the
// stored beginning of traffic counting equals expectedSince. If it differs
// (because the statistics were reset by another node), the amount is discarded.
// It returns the stored total and the stored beginning of counting
func (p DatabaseProvider) AddStatTraffic(addedTraffic uint64, expectedSince int64) (uint64, int64) {
	conn := p.pool.Get()
	defer conn.Close()
	values, err := redigo.Values(conn.Do("EVAL", trafficAddScript, 2,
		p.dbPrefix+prefixStatisticsTrafficSince,
		p.dbPrefix+prefixStatisticsTraffic,
		strconv.FormatUint(addedTraffic, 10), expectedSince))
	if err != nil {
		helper.Check(err)
		return 0, 0
	}
	total, err := redigo.Uint64(values[0], nil)
	if err != nil {
		helper.Check(err)
		return 0, 0
	}
	since, err := redigo.Int64(values[1], nil)
	if err != nil {
		helper.Check(err)
		return 0, 0
	}
	return total, since
}

// ResetStatTraffic atomically sets the total traffic to zero and the beginning
// of traffic counting to the given value
func (p DatabaseProvider) ResetStatTraffic(since int64) {
	conn := p.pool.Get()
	defer conn.Close()
	err := conn.Send("MULTI")
	if err != nil {
		helper.Check(err)
		return
	}
	err = conn.Send("SET", p.dbPrefix+prefixStatisticsTraffic, 0)
	if err != nil {
		helper.Check(err)
		return
	}
	err = conn.Send("SET", p.dbPrefix+prefixStatisticsTrafficSince, since)
	if err != nil {
		helper.Check(err)
		return
	}
	_, err = conn.Do("EXEC")
	helper.Check(err)
}

// InitTrafficSince stores the beginning of traffic counting if it has not been
// stored yet and returns the value that is currently stored
func (p DatabaseProvider) InitTrafficSince(since int64) int64 {
	conn := p.pool.Get()
	defer conn.Close()
	_, err := conn.Do("SET", p.dbPrefix+prefixStatisticsTrafficSince, since, "NX")
	helper.Check(err)
	result, ok := p.GetTrafficSince()
	if !ok {
		return since
	}
	return result
}

// GetTrafficSince gets the beginning of traffic counting
func (p DatabaseProvider) GetTrafficSince() (int64, bool) {
	return p.getKeyInt64(prefixStatisticsTrafficSince)
}
