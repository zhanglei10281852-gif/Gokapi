package database

import (
	"time"

	"github.com/forceu/gokapi/internal/helper"
	"github.com/forceu/gokapi/internal/storage/chunking/chunkreservation"
)

// CreateChunkReservation stores a reservation for an upcoming chunk upload.
// If maxFiles is greater than zero, the creation is atomic and fails when the
// number of completed uploads and valid reservations has reached maxFiles.
func CreateChunkReservation(requestId, uuid string, expiry int64, maxFiles int) bool {
	return db.CreateChunkReservation(requestId, uuid, expiry, maxFiles)
}

// SetChunkReservationUploading marks a reservation as uploading and extends its expiry.
func SetChunkReservationUploading(requestId, uuid string, newExpiry int64) bool {
	return db.SetChunkReservationUploading(requestId, uuid, newExpiry)
}

// ReleaseChunkReservation removes a reservation. The operation is idempotent.
func ReleaseChunkReservation(requestId, uuid string) {
	db.ReleaseChunkReservation(requestId, uuid)
}

// FinaliseChunkReservation marks a reservation as completed and stores the
// created file ID. If the reservation was already finalised, the existing file
// ID is returned.
func FinaliseChunkReservation(requestId, uuid, fileId string, expiry int64) (string, bool) {
	return db.FinaliseChunkReservation(requestId, uuid, fileId, expiry)
}

// GetFinalisedChunkReservation returns the file ID of a finalised, unexpired reservation
func GetFinalisedChunkReservation(requestId, uuid string) (string, bool) {
	return db.GetFinalisedChunkReservation(requestId, uuid)
}

// CountActiveChunkReservations returns the number of active, unexpired reservations
func CountActiveChunkReservations(requestId string) int {
	return db.CountActiveChunkReservations(requestId)
}

// DeleteAllChunkReservations removes all reservations associated with a file request
func DeleteAllChunkReservations(requestId string) {
	db.DeleteAllChunkReservations(requestId)
}

// DeleteExpiredChunkReservations reclaims all reservations that have expired
func DeleteExpiredChunkReservations() {
	db.DeleteExpiredChunkReservations()
}

// InitChunkReservationStore connects the chunk reservation lifecycle to the
// shared database. Reservations are then visible across instances, survive
// process restarts and expired entries are reclaimed immediately. Call this
// once after the database connection has been established.
func InitChunkReservationStore() {
	chunkreservation.SetStore(dbReservationStore{})
	db.DeleteExpiredChunkReservations()
}

// dbReservationStore implements chunkreservation.Store with the shared database,
// so that reservations are recognised on every instance and after a restart
type dbReservationStore struct{}

// New creates a reservation without a quota check (unlimited file requests)
func (dbReservationStore) New(requestId string) string {
	uuid := helper.GenerateRandomString(32)
	db.CreateChunkReservation(requestId, uuid,
		time.Now().Unix()+chunkreservation.TimeReservationWithoutUpload, 0)
	return uuid
}

// TryNew atomically creates a reservation if a quota slot is available
func (dbReservationStore) TryNew(requestId string, maxFiles, uploadedFiles int) (string, bool) {
	uuid := helper.GenerateRandomString(32)
	if !db.CreateChunkReservation(requestId, uuid,
		time.Now().Unix()+chunkreservation.TimeReservationWithoutUpload, maxFiles) {
		return "", false
	}
	return uuid, true
}

// SetUploading validates an active, unexpired reservation and extends its lifetime
func (dbReservationStore) SetUploading(requestId, uuid string) bool {
	return db.SetChunkReservationUploading(requestId, uuid,
		time.Now().Unix()+chunkreservation.TimeReservationWithUpload)
}

// Release removes a reservation exactly once, regardless of which terminal
// event (success, cancel, failure or timeout) triggers it
func (dbReservationStore) Release(requestId, uuid string) {
	db.ReleaseChunkReservation(requestId, uuid)
}

// Finalise marks a reservation as successfully completed
func (dbReservationStore) Finalise(requestId, uuid, fileId string) (string, bool) {
	return db.FinaliseChunkReservation(requestId, uuid, fileId,
		time.Now().Unix()+chunkreservation.TimeReservationWithUpload)
}

// GetFinalised returns the file ID of a reservation that was already finalised
func (dbReservationStore) GetFinalised(requestId, uuid string) (string, bool) {
	return db.GetFinalisedChunkReservation(requestId, uuid)
}

// CountActive returns the number of active, unexpired reservations
func (dbReservationStore) CountActive(requestId string) int {
	return db.CountActiveChunkReservations(requestId)
}

// CleanExpired reclaims expired reservations
func (dbReservationStore) CleanExpired() {
	db.DeleteExpiredChunkReservations()
}
