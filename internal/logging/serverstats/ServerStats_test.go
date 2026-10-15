//go:build test

package serverstats

import (
	"os"
	"sync"
	"testing"

	"github.com/forceu/gokapi/internal/configuration"
	"github.com/forceu/gokapi/internal/configuration/database"
	"github.com/forceu/gokapi/internal/test"
	"github.com/forceu/gokapi/internal/test/testconfiguration"
)

func TestMain(m *testing.M) {
	testconfiguration.Create(false)
	configuration.Load()
	configuration.ConnectDatabase()
	exitVal := m.Run()
	database.Close()
	testconfiguration.Delete()
	os.Exit(exitVal)
}

const testEpoch = 1000

func resetSharedTraffic(t *testing.T, epoch int64) {
	t.Helper()
	database.ResetStatTraffic(epoch)
}

func TestZeroTraffic(t *testing.T) {
	resetSharedTraffic(t, testEpoch)
	total, since := GetCurrentTraffic()
	test.IsEqual(t, total, uint64(0))
	test.IsEqualInt64(t, since, testEpoch)

	AddTraffic(0)
	total, since = GetCurrentTraffic()
	test.IsEqual(t, total, uint64(0))
	test.IsEqualInt64(t, since, testEpoch)
	test.IsEqual(t, database.GetStatTraffic(), uint64(0))
}

func TestSingleNodeAccumulates(t *testing.T) {
	resetSharedTraffic(t, testEpoch)
	node := newTrafficTracker()

	node.addTraffic(100)
	total, since := node.getCurrentTraffic()
	test.IsEqual(t, total, uint64(100))
	test.IsEqualInt64(t, since, testEpoch)
	// Reading the stats must have persisted the increment
	test.IsEqual(t, database.GetStatTraffic(), uint64(100))

	node.addTraffic(50)
	node.saveTraffic()
	test.IsEqual(t, database.GetStatTraffic(), uint64(150))
}

func TestMultipleNodesInterleavedSaves(t *testing.T) {
	resetSharedTraffic(t, testEpoch)
	nodeA := newTrafficTracker()
	nodeB := newTrafficTracker()

	nodeA.addTraffic(100)
	nodeB.addTraffic(200)
	nodeA.saveTraffic()
	nodeB.saveTraffic()
	test.IsEqual(t, database.GetStatTraffic(), uint64(300))

	totalA, sinceA := nodeA.getCurrentTraffic()
	totalB, sinceB := nodeB.getCurrentTraffic()
	test.IsEqual(t, totalA, uint64(300))
	test.IsEqual(t, totalB, uint64(300))
	test.IsEqualInt64(t, sinceA, testEpoch)
	test.IsEqualInt64(t, sinceB, testEpoch)

	// Interleaved saves must each add to the shared total
	nodeA.addTraffic(50)
	nodeA.saveTraffic()
	nodeB.addTraffic(70)
	nodeB.saveTraffic()
	nodeA.addTraffic(10)
	nodeA.saveTraffic()

	totalA, _ = nodeA.getCurrentTraffic()
	totalB, _ = nodeB.getCurrentTraffic()
	test.IsEqual(t, totalA, uint64(430))
	test.IsEqual(t, totalB, uint64(430))
}

func TestRestartUsesSharedBaseline(t *testing.T) {
	resetSharedTraffic(t, testEpoch)
	nodeA := newTrafficTracker()
	nodeA.addTraffic(100)
	nodeA.saveTraffic()

	// Simulate a restart: the new node keeps using the shared baseline and
	// must neither duplicate nor overwrite the stored total
	nodeRestarted := newTrafficTracker()
	total, since := nodeRestarted.getCurrentTraffic()
	test.IsEqual(t, total, uint64(100))
	test.IsEqualInt64(t, since, testEpoch)
	test.IsEqual(t, database.GetStatTraffic(), uint64(100))

	nodeRestarted.addTraffic(50)
	nodeRestarted.saveTraffic()
	test.IsEqual(t, database.GetStatTraffic(), uint64(150))

	// The old node, which kept running, must also add instead of overwriting
	nodeA.addTraffic(20)
	nodeA.saveTraffic()
	test.IsEqual(t, database.GetStatTraffic(), uint64(170))
}

func TestResetOnOtherNode(t *testing.T) {
	resetSharedTraffic(t, testEpoch)
	nodeA := newTrafficTracker()
	nodeB := newTrafficTracker()

	// Node A has unflushed pre-reset traffic
	nodeA.addTraffic(1000)
	nodeB.addTraffic(2000)
	nodeB.saveTraffic()
	test.IsEqual(t, database.GetStatTraffic(), uint64(2000))

	// Reset is issued on a third instance
	resetSharedTraffic(t, testEpoch+1)

	// Node A shuts down after the reset: its pre-reset increment must not
	// reappear
	nodeA.saveTraffic()
	test.IsEqual(t, database.GetStatTraffic(), uint64(0))

	total, since := nodeA.getCurrentTraffic()
	test.IsEqual(t, total, uint64(0))
	test.IsEqualInt64(t, since, testEpoch+1)

	// Downloads completing after the reset continue to accumulate on the old
	// node
	nodeA.addTraffic(400)
	nodeA.saveTraffic()
	test.IsEqual(t, database.GetStatTraffic(), uint64(400))

	total, since = nodeB.getCurrentTraffic()
	test.IsEqual(t, total, uint64(400))
	test.IsEqualInt64(t, since, testEpoch+1)
	nodeB.addTraffic(60)
	nodeB.saveTraffic()
	test.IsEqual(t, database.GetStatTraffic(), uint64(460))
}

func TestClearTrafficDiscardsPendingOfOtherNodes(t *testing.T) {
	resetSharedTraffic(t, testEpoch)
	nodeA := newTrafficTracker()
	nodeA.addTraffic(500)

	oldTracker := traffic
	traffic = newTrafficTracker()
	defer func() { traffic = oldTracker }()

	ClearTraffic()
	total, since := GetCurrentTraffic()
	test.IsEqual(t, total, uint64(0))
	if since == testEpoch {
		t.Errorf("Expected a new recording epoch after reset, still got %d", testEpoch)
	}
	newEpoch := since

	// The pending pre-reset bytes of node A must not reappear
	nodeA.saveTraffic()
	total, since = nodeA.getCurrentTraffic()
	test.IsEqual(t, total, uint64(0))
	test.IsEqualInt64(t, since, newEpoch)

	// Post-reset downloads accumulate normally
	nodeA.addTraffic(123)
	nodeA.saveTraffic()
	total, since = nodeA.getCurrentTraffic()
	test.IsEqual(t, total, uint64(123))
	test.IsEqualInt64(t, since, newEpoch)
}

func TestShutdownPersistsPending(t *testing.T) {
	resetSharedTraffic(t, testEpoch)
	oldTracker := traffic
	traffic = newTrafficTracker()
	defer func() { traffic = oldTracker }()

	AddTraffic(999)
	test.IsEqual(t, database.GetStatTraffic(), uint64(0))
	Shutdown()
	test.IsEqual(t, database.GetStatTraffic(), uint64(999))
	total, since := GetCurrentTraffic()
	test.IsEqual(t, total, uint64(999))
	test.IsEqualInt64(t, since, testEpoch)
}

func TestConcurrentDownloadsOnMultipleNodes(t *testing.T) {
	resetSharedTraffic(t, testEpoch)
	nodeA := newTrafficTracker()
	nodeB := newTrafficTracker()

	const downloadsPerNode = 200
	var wg sync.WaitGroup
	for i := 0; i < downloadsPerNode; i++ {
		wg.Add(2)
		go func() {
			defer wg.Done()
			nodeA.addTraffic(1)
			if i%7 == 0 {
				nodeA.saveTraffic()
			}
		}()
		go func() {
			defer wg.Done()
			nodeB.addTraffic(1)
			if i%5 == 0 {
				nodeB.saveTraffic()
			}
		}()
	}
	wg.Wait()
	nodeA.saveTraffic()
	nodeB.saveTraffic()

	test.IsEqual(t, database.GetStatTraffic(), uint64(downloadsPerNode*2))
	totalA, _ := nodeA.getCurrentTraffic()
	totalB, _ := nodeB.getCurrentTraffic()
	test.IsEqual(t, totalA, uint64(downloadsPerNode*2))
	test.IsEqual(t, totalB, uint64(downloadsPerNode*2))
}
