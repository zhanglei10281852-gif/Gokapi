package sqlite

import (
	"database/sql"
	"errors"
	"time"

	"github.com/forceu/gokapi/internal/helper"
)

const (
	statusReservationActive     = 0
	statusReservationFinalised  = 1
)

// CreateChunkReservation stores a reservation for an upcoming chunk upload.
// If maxFiles is greater than zero, the creation is atomic and fails when the
// number of completed uploads and valid reservations has reached maxFiles.
func (p DatabaseProvider) CreateChunkReservation(requestId, uuid string, expiry int64, maxFiles int) bool {
	if requestId == "" || uuid == "" {
		return false
	}
	var (
		result sql.Result
		err    error
	)
	if maxFiles <= 0 {
		result, err = p.sqliteDb.Exec(`INSERT OR IGNORE INTO ChunkReservations
			(UploadRequestId, Uuid, Expiry, Status, FileId) VALUES (?, ?, ?, ?, '')`,
			requestId, uuid, expiry, statusReservationActive)
	} else {
		// The quota check and the insert are executed as a single atomic
		// statement, so concurrent reservations cannot both claim the last
		// available slot.
		now := time.Now().Unix()
		result, err = p.sqliteDb.Exec(`INSERT INTO ChunkReservations
			(UploadRequestId, Uuid, Expiry, Status, FileId)
			SELECT ?, ?, ?, ?, ''
			WHERE (
				SELECT COUNT(*) FROM FileMetaData
					WHERE UploadRequestId = ? AND PendingDeletion = 0
			) + (
				SELECT COUNT(*) FROM ChunkReservations
					WHERE UploadRequestId = ? AND Status = ? AND Expiry >= ?
			) < ?`,
			requestId, uuid, expiry, statusReservationActive,
			requestId,
			requestId, statusReservationActive, now, maxFiles)
	}
	if err != nil {
		helper.Check(err)
		return false
	}
	rows, err := result.RowsAffected()
	helper.Check(err)
	return rows == 1
}

// SetChunkReservationUploading marks a reservation as uploading and extends its expiry.
func (p DatabaseProvider) SetChunkReservationUploading(requestId, uuid string, newExpiry int64) bool {
	result, err := p.sqliteDb.Exec(`UPDATE ChunkReservations SET Expiry = ?
		WHERE UploadRequestId = ? AND Uuid = ? AND Status = ? AND Expiry >= ?`,
		newExpiry, requestId, uuid, statusReservationActive, time.Now().Unix())
	if err != nil {
		helper.Check(err)
		return false
	}
	rows, err := result.RowsAffected()
	helper.Check(err)
	return rows == 1
}

// ReleaseChunkReservation removes a reservation. The operation is idempotent.
func (p DatabaseProvider) ReleaseChunkReservation(requestId, uuid string) {
	if requestId == "" || uuid == "" {
		return
	}
	_, err := p.sqliteDb.Exec(`DELETE FROM ChunkReservations
		WHERE UploadRequestId = ? AND Uuid = ?`, requestId, uuid)
	helper.Check(err)
}

// FinaliseChunkReservation marks a reservation as completed and stores the
// created file ID. If the reservation was already finalised, the existing file
// ID is returned.
func (p DatabaseProvider) FinaliseChunkReservation(requestId, uuid, fileId string, expiry int64) (string, bool) {
	now := time.Now().Unix()
	result, err := p.sqliteDb.Exec(`UPDATE ChunkReservations
		SET Status = ?, FileId = ?, Expiry = ?
		WHERE UploadRequestId = ? AND Uuid = ? AND Status = ? AND Expiry >= ?`,
		statusReservationFinalised, fileId, expiry, requestId, uuid, statusReservationActive, now)
	if err != nil {
		helper.Check(err)
		return "", false
	}
	rows, err := result.RowsAffected()
	helper.Check(err)
	if rows == 1 {
		return "", true
	}
	return p.GetFinalisedChunkReservation(requestId, uuid)
}

// GetFinalisedChunkReservation returns the file ID of a finalised, unexpired reservation
func (p DatabaseProvider) GetFinalisedChunkReservation(requestId, uuid string) (string, bool) {
	var fileId string
	row := p.sqliteDb.QueryRow(`SELECT FileId FROM ChunkReservations
		WHERE UploadRequestId = ? AND Uuid = ? AND Status = ? AND Expiry >= ?`,
		requestId, uuid, statusReservationFinalised, time.Now().Unix())
	err := row.Scan(&fileId)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", false
		}
		helper.Check(err)
		return "", false
	}
	return fileId, true
}

// CountActiveChunkReservations returns the number of active, unexpired reservations
func (p DatabaseProvider) CountActiveChunkReservations(requestId string) int {
	var count int
	row := p.sqliteDb.QueryRow(`SELECT COUNT(*) FROM ChunkReservations
		WHERE UploadRequestId = ? AND Status = ? AND Expiry >= ?`,
		requestId, statusReservationActive, time.Now().Unix())
	err := row.Scan(&count)
	helper.Check(err)
	return count
}

// DeleteAllChunkReservations removes all reservations associated with a file request
func (p DatabaseProvider) DeleteAllChunkReservations(requestId string) {
	if requestId == "" {
		return
	}
	_, err := p.sqliteDb.Exec(`DELETE FROM ChunkReservations WHERE UploadRequestId = ?`, requestId)
	helper.Check(err)
}

// DeleteExpiredChunkReservations reclaims all reservations that have expired
func (p DatabaseProvider) DeleteExpiredChunkReservations() {
	_, err := p.sqliteDb.Exec(`DELETE FROM ChunkReservations WHERE Expiry < ?`, time.Now().Unix())
	helper.Check(err)
}
