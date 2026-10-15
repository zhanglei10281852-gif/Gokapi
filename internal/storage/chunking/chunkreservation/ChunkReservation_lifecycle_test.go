package chunkreservation

import (
	"sync"
	"testing"
	"time"
)

// TestTryNew_ReservesWhileQuotaAvailable covers the shared quota of completed
// uploads and valid reservations
func TestTryNew_ReservesWhileQuotaAvailable(t *testing.T) {
	resetStateBlockCleanup()
	uuid1, ok := TryNew("quota", 2, 0)
	if !ok || uuid1 == "" {
		t.Fatal("expected first reservation to succeed")
	}
	uuid2, ok := TryNew("quota", 2, 0)
	if !ok || uuid2 == "" {
		t.Fatal("expected second reservation to succeed")
	}
	if uuid1 == uuid2 {
		t.Error("expected unique uuids")
	}
	if _, ok := TryNew("quota", 2, 0); ok {
		t.Error("expected third reservation to be rejected, quota is exhausted")
	}
	if count := GetCount("quota"); count != 2 {
		t.Errorf("expected 2 active reservations, got %d", count)
	}
}

// TestTryNew_ConsidersCompletedUploads ensures completed files count against the quota
func TestTryNew_ConsidersCompletedUploads(t *testing.T) {
	resetStateBlockCleanup()
	if _, ok := TryNew("quota2", 1, 1); ok {
		t.Error("expected reservation to fail when one file is already uploaded with max one")
	}
	uuid, ok := TryNew("quota2", 3, 2)
	if !ok {
		t.Fatal("expected reservation to succeed when one slot remains")
	}
	if _, ok := TryNew("quota2", 3, 2); ok {
		t.Error("expected reservation to fail when last slot is taken")
	}
	SetComplete("quota2", uuid)
	if _, ok := TryNew("quota2", 3, 2); !ok {
		t.Error("expected reservation to succeed after slot was released")
	}
}

// TestTryNew_UnlimitedAlwaysSucceeds checks that maxFiles of 0 keeps the old unlimited behaviour
func TestTryNew_UnlimitedAlwaysSucceeds(t *testing.T) {
	resetStateBlockCleanup()
	for i := 0; i < 10; i++ {
		if _, ok := TryNew("unlimited", 0, 0); !ok {
			t.Fatalf("expected reservation %d to succeed for unlimited request", i)
		}
	}
}

// TestTryNew_LastSlotAtomic makes sure that concurrent reservations can only
// claim the last available slot once
func TestTryNew_LastSlotAtomic(t *testing.T) {
	resetStateBlockCleanup()
	const goroutines = 50
	var wg sync.WaitGroup
	var successCounter int64
	var counterMutex sync.Mutex
	wg.Add(goroutines)
	for i := 0; i < goroutines; i++ {
		go func() {
			defer wg.Done()
			_, ok := TryNew("race", 1, 0)
			if ok {
				counterMutex.Lock()
				successCounter++
				counterMutex.Unlock()
			}
		}()
	}
	wg.Wait()
	if successCounter != 1 {
		t.Errorf("expected exactly one successful last-slot reservation, got %d", successCounter)
	}
}

// TestFinalise_Idempotent verifies that a duplicate completion returns the file
// id of the first completion and that finalised reservations no longer count
// against the active quota
func TestFinalise_Idempotent(t *testing.T) {
	resetStateBlockCleanup()
	uuid := New("finalise")

	duplicate, ok := Finalise("finalise", uuid, "file-1")
	if !ok {
		t.Fatal("expected first Finalise to succeed")
	}
	if duplicate != "" {
		t.Errorf("expected no duplicate file id on first finalise, got %q", duplicate)
	}

	fileId, ok := GetFinalised("finalise", uuid)
	if !ok || fileId != "file-1" {
		t.Errorf("expected finalised file id file-1, got %q (ok=%v)", fileId, ok)
	}

	duplicate, ok = Finalise("finalise", uuid, "file-2")
	if !ok {
		t.Error("expected duplicate Finalise to report ok")
	}
	if duplicate != "file-1" {
		t.Errorf("expected duplicate Finalise to return original file id, got %q", duplicate)
	}

	if count := GetCount("finalise"); count != 0 {
		t.Errorf("expected finalised reservation not to count as active, got %d", count)
	}
	if SetUploading("finalise", uuid) {
		t.Error("expected SetUploading to be rejected for finalised reservation")
	}
}

// TestFinalise_MissingOrExpired checks unknown and expired reservations
func TestFinalise_MissingOrExpired(t *testing.T) {
	resetStateBlockCleanup()
	if _, ok := Finalise("nope", "uuid", "file"); ok {
		t.Error("expected Finalise to fail for unknown reservation")
	}
	uuid := "expired"
	reservationMutex.Lock()
	reservedChunks["exp"] = map[string]reservation{
		uuid: {Uuid: uuid, Expiry: time.Now().Unix() - 1, Status: statusActive},
	}
	reservationMutex.Unlock()
	if _, ok := Finalise("exp", uuid, "file"); ok {
		t.Error("expected Finalise to fail for expired reservation")
	}
}

// TestSetComplete_ReleasesExactlyOnce verifies that repeated release calls do
// not resurrect or double-release a reservation
func TestSetComplete_ReleasesExactlyOnce(t *testing.T) {
	resetStateBlockCleanup()
	uuid, _ := TryNew("release", 1, 0)
	SetComplete("release", uuid)
	SetComplete("release", uuid)
	SetComplete("release", "other-uuid")
	if count := GetCount("release"); count != 0 {
		t.Errorf("expected no active reservations after release, got %d", count)
	}
	// The freed slot can be claimed again exactly once
	if _, ok := TryNew("release", 1, 0); !ok {
		t.Error("expected released slot to become available again")
	}
}

// TestSetUploading_RejectedForFinalisedReservation is covered above, but this
// also ensures a finalised then released reservation is fully gone
func TestReleaseFinalised(t *testing.T) {
	resetStateBlockCleanup()
	uuid := New("relfin")
	_, _ = Finalise("relfin", uuid, "file-x")
	SetComplete("relfin", uuid)
	if _, ok := GetFinalised("relfin", uuid); ok {
		t.Error("expected finalised reservation to be removed after release")
	}
}

// stubStore is used to verify store injection
type stubStore struct {
	newCount    int
	tryNewCount int
}

func (s *stubStore) New(string) string                         { s.newCount++; return "stub" }
func (s *stubStore) TryNew(string, int, int) (string, bool)    { s.tryNewCount++; return "stub", true }
func (s *stubStore) SetUploading(string, string) bool          { return true }
func (s *stubStore) Release(string, string)                    {}
func (s *stubStore) Finalise(string, string, string) (string, bool) { return "", true }
func (s *stubStore) GetFinalised(string, string) (string, bool)     { return "", false }
func (s *stubStore) CountActive(string) int                      { return s.newCount + s.tryNewCount }
func (s *stubStore) CleanExpired()                               {}

// TestSetStore_Delegates ensures that production can replace the memory store
// and that it can be reset to nil afterwards
func TestSetStore_Delegates(t *testing.T) {
	resetStateBlockCleanup()
	stub := &stubStore{}
	SetStore(stub)
	defer SetStore(nil)
	if uuid := New("any"); uuid != "stub" {
		t.Errorf("expected delegation to stub store, got %q", uuid)
	}
	if _, ok := TryNew("any", 1, 0); !ok {
		t.Error("expected stub to answer TryNew")
	}
	if count := GetCount("any"); count != 2 {
		t.Errorf("expected stub count 2, got %d", count)
	}
	SetStore(nil)
	if uuid := New("reset"); uuid == "stub" {
		t.Error("expected nil SetStore to reset to the memory store")
	}
}
