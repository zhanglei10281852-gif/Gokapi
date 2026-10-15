package serverstats

import (
	"fmt"
	"math"
	"sync"
	"time"

	"github.com/forceu/gokapi/internal/configuration"
	"github.com/forceu/gokapi/internal/configuration/database"
	"github.com/shirou/gopsutil/v4/cpu"
	"github.com/shirou/gopsutil/v4/disk"
	"github.com/shirou/gopsutil/v4/mem"
)

const trafficSaveInterval = 5 * time.Minute

var startTime time.Time
var lastCpuCheck time.Time
var traffic = newTrafficTracker()

// trafficTracker contains the traffic that has been recorded on this node but
// not yet persisted to the shared database. Pending bytes are grouped by the
// epoch ("recording since" timestamp) that was valid when the download
// completed. This allows several nodes to accumulate against the same shared
// baseline without overwriting each other and makes a concurrent reset
// unambiguous: bytes attributed to the old epoch are discarded, bytes
// attributed to the new epoch continue to accumulate
type trafficTracker struct {
	// mutex guards pending and lastFlush
	mutex sync.Mutex
	// flushMutex serialises database flushes and resets performed by this node
	flushMutex sync.Mutex
	pending    map[int64]uint64
	lastFlush  time.Time
}

func newTrafficTracker() *trafficTracker {
	return &trafficTracker{
		pending:   make(map[int64]uint64),
		lastFlush: time.Now(),
	}
}

// Init initializes the server stats
func Init() {
	startTime = time.Now()
	traffic = newTrafficTracker()
	// The shared baseline is never loaded into memory, it is always read and
	// updated in the database. Only make sure an epoch exists for new deployments
	_ = getTrafficSince()
	monitorCpuUsage()
}

func getTrafficSince() int64 {
	since, ok := database.GetTrafficSince()
	if !ok {
		since = time.Now().Unix()
		since = database.InitTrafficSince(since)
	}
	return since
}

func monitorCpuUsage() {
	// continuously run, as GetCpuUsage only reports the
	// percentage since the last call
	go func() {
		if time.Since(lastCpuCheck) > 2*time.Minute {
			_ = GetCpuUsage()
			lastCpuCheck = time.Now()
		}
		time.Sleep(time.Minute)
		monitorCpuUsage()
	}()
}

// Shutdown saves statistics to the database
func Shutdown() {
	traffic.saveTraffic()
}

// ClearTraffic resets the traffic counter
func ClearTraffic() {
	traffic.clearTraffic()
}

// GetUptime returns the uptime of the server in seconds
func GetUptime() int64 {
	return time.Since(startTime).Milliseconds() / 1000
}

// GetTotalFiles returns the total number of files stored in the database
func GetTotalFiles() int {
	return len(database.GetAllMetadata())
}

// GetCurrentTraffic returns the current traffic in bytes and the time since the last recording
func GetCurrentTraffic() (uint64, int64) {
	return traffic.getCurrentTraffic()
}

// GetMemoryInfo returns information about the memory usage
func GetMemoryInfo() (uint64, uint64, uint64, int) {
	memInfo, err := mem.VirtualMemory()
	if err != nil {
		fmt.Println(err)
		return 0, 0, 0, 0
	}
	return memInfo.Free, memInfo.Used, memInfo.Total, int((float64(memInfo.Used) / float64(memInfo.Total)) * 100)
}

// GetDiskInfo returns information about the disk usage
func GetDiskInfo() (uint64, uint64, uint64, int) {
	diskInfo, err := disk.Usage(configuration.Get().DataDir)
	if err != nil {
		fmt.Println(err)
		return 0, 0, 0, 0
	}
	return diskInfo.Free, diskInfo.Used, diskInfo.Total, int((float64(diskInfo.Used) / float64(diskInfo.Total)) * 100)
}

// GetCpuUsage returns the current CPU usage in percent
func GetCpuUsage() int {
	usage, err := cpu.Percent(0, false)
	if err != nil {
		fmt.Println(err)
		return 0
	}
	return int(math.Round(usage[0]))
}

// AddTraffic adds traffic to the current traffic counter
func AddTraffic(bytes uint64) {
	traffic.addTraffic(bytes)
}

func (t *trafficTracker) addTraffic(bytes uint64) {
	if bytes == 0 {
		return
	}
	// Attribute the download to the epoch that is shared between all nodes at
	// the time it completes
	epoch := getTrafficSince()

	t.mutex.Lock()
	t.pending[epoch] += bytes
	shouldSave := time.Since(t.lastFlush) >= trafficSaveInterval
	t.mutex.Unlock()

	if shouldSave {
		t.saveTraffic()
	}
}

// saveTraffic persists all pending increments of this node to the shared
// database. Every bucket is added conditionally on its epoch, so increments
// recorded before a reset are never written back after the reset
func (t *trafficTracker) saveTraffic() {
	t.flushMutex.Lock()
	defer t.flushMutex.Unlock()

	t.mutex.Lock()
	pending := t.pending
	t.pending = make(map[int64]uint64)
	t.mutex.Unlock()

	for epoch, bytes := range pending {
		if bytes == 0 {
			continue
		}
		// The database rejects the increment if the statistics were reset since
		// the bytes were recorded; those bytes are dropped
		_, _ = database.AddStatTraffic(bytes, epoch)
	}

	t.mutex.Lock()
	t.lastFlush = time.Now()
	t.mutex.Unlock()
}

func (t *trafficTracker) clearTraffic() {
	// Serialise against concurrent periodic saves and shutdowns of this node
	t.flushMutex.Lock()
	defer t.flushMutex.Unlock()

	// Discard all increments that have not been persisted yet, regardless of
	// which epoch they belong to, so they can never reappear after the reset
	t.mutex.Lock()
	t.pending = make(map[int64]uint64)
	t.mutex.Unlock()

	newEpoch := time.Now().Unix()
	// Atomically reset the shared total and epoch
	database.ResetStatTraffic(newEpoch)

	t.mutex.Lock()
	t.pending = make(map[int64]uint64)
	t.lastFlush = time.Now()
	t.mutex.Unlock()
}

func (t *trafficTracker) getCurrentTraffic() (uint64, int64) {
	// Persist the increments of this node first, so the returned value matches
	// what every other instance reads from the shared database
	t.saveTraffic()
	epoch := getTrafficSince()
	total := database.GetStatTraffic()
	t.mutex.Lock()
	pendingForEpoch := t.pending[epoch]
	t.mutex.Unlock()
	return total + pendingForEpoch, epoch
}
