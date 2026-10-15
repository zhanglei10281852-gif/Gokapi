package chunkreservation

import (
	"sync"
	"time"

	"github.com/forceu/gokapi/internal/helper"
)

var reservedChunks = make(map[string]map[string]reservation)
var reservationMutex sync.RWMutex
var runGcOnce = sync.Once{}

// TimeReservationWithoutUpload is the lifetime of a reservation before the first
// chunk has been uploaded. After this time, the reservation can be reclaimed.
const TimeReservationWithoutUpload int64 = 4 * 60

// TimeReservationWithUpload is the lifetime of a reservation once the upload has
// started. Each uploaded chunk extends the expiry to this value.
const TimeReservationWithUpload int64 = 23 * 60 * 60

// Kept as unexported aliases for backwards compatibility
const timeReservationWithoutUpload = TimeReservationWithoutUpload
const timeReservationWithUpload = TimeReservationWithUpload

const (
	statusActive = iota
	statusFinalised
)

type reservation struct {
	Uuid   string
	Expiry int64
	// Status is statusActive for ongoing reservations and statusFinalised once an
	// upload completed successfully. Finalised entries are retained until their
	// expiry so that duplicate or late complete requests stay idempotent.
	Status int
	// FileId contains the ID of the created file once the reservation is finalised
	FileId string
}

// Store is the backend used to keep track of chunk reservations. The default
// implementation stores reservations in memory, production deployments use a
// database-backed implementation so that reservations are shared between
// instances and survive process restarts.
type Store interface {
	// New creates a reservation without enforcing a file quota. Used for file
	// requests with an unlimited number of files.
	New(requestId string) string
	// TryNew atomically creates a reservation if the shared quota
	// (uploaded files + valid reservations) has a free slot. It returns the
	// uuid and true on success or an empty string and false if the last slot
	// was taken by another reservation.
	TryNew(requestId string, maxFiles, uploadedFiles int) (string, bool)
	// SetUploading validates that an active, unexpired reservation exists and
	// extends its lifetime for the ongoing upload.
	SetUploading(requestId, uuid string) bool
	// Release removes a reservation (active or finalised). It is idempotent
	// and can be called for success, cancel, failure or timeout without
	// releasing a slot more than once.
	Release(requestId, uuid string)
	// Finalise marks an active reservation as successfully completed and
	// records the created file ID. If the reservation was already finalised,
	// the previously recorded file ID is returned as duplicateFileId. If no
	// valid reservation exists, ok is false.
	Finalise(requestId, uuid, fileId string) (duplicateFileId string, ok bool)
	// GetFinalised returns the file ID of a reservation that was already
	// finalised and has not expired.
	GetFinalised(requestId, uuid string) (fileId string, ok bool)
	// CountActive returns the number of active, unexpired reservations.
	CountActive(requestId string) int
	// CleanExpired reclaims all reservations that have expired.
	CleanExpired()
}

var activeStore Store = memoryStore{}

// SetStore replaces the active reservation store. It should only be called
// once during startup, after the database connection has been established.
// Passing nil resets the store to the in-memory implementation.
func SetStore(store Store) {
	reservationMutex.Lock()
	defer reservationMutex.Unlock()
	if store == nil {
		activeStore = memoryStore{}
		return
	}
	activeStore = store
}

// GetCount returns the number of active, unexpired chunks reserved for the given file request
func GetCount(id string) int {
	return activeStore.CountActive(id)
}

// New creates a new chunk reservation without a quota check (unlimited file requests)
func New(id string) string {
	return activeStore.New(id)
}

// TryNew creates a new chunk reservation if a slot is available according to
// maxFiles, counting completed uploads and valid reservations as a shared
// quota. The check and the creation are atomic, so the last available slot is
// granted to at most one reservation.
func TryNew(id string, maxFiles, uploadedFiles int) (string, bool) {
	return activeStore.TryNew(id, maxFiles, uploadedFiles)
}

// SetComplete marks a chunk as complete or cancelled. The operation is idempotent.
func SetComplete(id, uuid string) {
	activeStore.Release(id, uuid)
}

// SetUploading marks a chunk as uploading and extends the reservation time.
// It returns false if no active, unexpired reservation exists for the uuid.
func SetUploading(id string, uuid string) bool {
	return activeStore.SetUploading(id, uuid)
}

// Finalise marks a reservation as successfully completed and stores the ID of
// the resulting file. A second call for the same uuid returns the file ID of
// the first completion, allowing duplicate or late complete requests to be
// answered idempotently.
func Finalise(id, uuid, fileId string) (string, bool) {
	return activeStore.Finalise(id, uuid, fileId)
}

// GetFinalised returns the file ID if the reservation has already been finalised
func GetFinalised(id, uuid string) (string, bool) {
	return activeStore.GetFinalised(id, uuid)
}

// memoryStore is the default Store implementation. It keeps reservations in
// process memory and is used for single-instance deployments and unit tests.
type memoryStore struct{}

// New creates a new chunk reservation
func (memoryStore) New(id string) string {
	reservationMutex.Lock()
	defer reservationMutex.Unlock()

	uuid := helper.GenerateRandomString(32)
	if reservedChunks[id] == nil {
		reservedChunks[id] = make(map[string]reservation)
	}
	reservedChunks[id][uuid] = reservation{
		Uuid:   uuid,
		Expiry: time.Now().Unix() + TimeReservationWithoutUpload,
		Status: statusActive,
	}

	runGcOnce.Do(func() { go cleanUp(true) })
	return uuid
}

// TryNew creates a reservation if the shared quota has a free slot
func (memoryStore) TryNew(id string, maxFiles, uploadedFiles int) (string, bool) {
	reservationMutex.Lock()
	defer reservationMutex.Unlock()

	now := time.Now().Unix()
	if maxFiles > 0 {
		activeReservations := 0
		for _, reservedChunk := range reservedChunks[id] {
			if reservedChunk.Status == statusActive && reservedChunk.Expiry >= now {
				activeReservations++
			}
		}
		if uploadedFiles+activeReservations >= maxFiles {
			return "", false
		}
	}

	uuid := helper.GenerateRandomString(32)
	if reservedChunks[id] == nil {
		reservedChunks[id] = make(map[string]reservation)
	}
	reservedChunks[id][uuid] = reservation{
		Uuid:   uuid,
		Expiry: now + TimeReservationWithoutUpload,
		Status: statusActive,
	}

	runGcOnce.Do(func() { go cleanUp(true) })
	return uuid, true
}

// Release deletes a reservation, regardless of its state
func (memoryStore) Release(id, uuid string) {
	reservationMutex.Lock()
	delete(reservedChunks[id], uuid)
	reservationMutex.Unlock()
}

// Finalise marks an active reservation as finalised
func (memoryStore) Finalise(id, uuid, fileId string) (string, bool) {
	reservationMutex.Lock()
	defer reservationMutex.Unlock()

	chunk, ok := reservedChunks[id][uuid]
	if !ok || chunk.Expiry < time.Now().Unix() {
		return "", false
	}
	if chunk.Status == statusFinalised {
		return chunk.FileId, true
	}
	chunk.Status = statusFinalised
	chunk.FileId = fileId
	reservedChunks[id][uuid] = chunk
	return "", true
}

// GetFinalised returns the file ID of a finalised reservation
func (memoryStore) GetFinalised(id, uuid string) (string, bool) {
	reservationMutex.RLock()
	defer reservationMutex.RUnlock()

	chunk, ok := reservedChunks[id][uuid]
	if !ok || chunk.Status != statusFinalised || chunk.Expiry < time.Now().Unix() {
		return "", false
	}
	return chunk.FileId, true
}

// CountActive returns the number of active, unexpired reservations
func (memoryStore) CountActive(id string) int {
	reservationMutex.RLock()
	defer reservationMutex.RUnlock()

	now := time.Now().Unix()
	count := 0
	for _, reservedChunk := range reservedChunks[id] {
		if reservedChunk.Status == statusActive && reservedChunk.Expiry >= now {
			count++
		}
	}
	return count
}

// CleanExpired reclaims expired reservations
func (memoryStore) CleanExpired() {
	cleanUp(false)
}

func (memoryStore) SetUploading(id string, uuid string) bool {
	reservationMutex.Lock()
	defer reservationMutex.Unlock()

	if reservedChunks[id] == nil {
		return false
	}
	chunk, ok := reservedChunks[id][uuid]
	if !ok {
		return false
	}
	if chunk.Status != statusActive || chunk.Expiry < time.Now().Unix() {
		return false
	}
	chunk.Expiry = time.Now().Unix() + TimeReservationWithUpload
	reservedChunks[id][uuid] = chunk
	return true
}

func cleanUp(isPeriodic bool) {
	reservationMutex.Lock()
	for id, chunks := range reservedChunks {
		now := time.Now().Unix()
		for uuid, reservedChunk := range chunks {
			if reservedChunk.Expiry < now {
				delete(reservedChunks[id], uuid)
			}
		}
	}
	reservationMutex.Unlock()

	if isPeriodic {
		go func() {
			time.Sleep(time.Minute * 5)
			cleanUp(true)
		}()
	}
}
