package sqlite

import (
	"database/sql"
	"errors"

	"github.com/forceu/gokapi/internal/helper"
)

const statIdTraffic = "1"
const statIdTrafficSince = "2"

// GetStatTraffic returns the total traffic from statistics
func (p DatabaseProvider) GetStatTraffic() uint64 {
	var result uint64
	row := p.sqliteDb.QueryRow("SELECT value FROM Statistics WHERE type = ?", statIdTraffic)
	err := row.Scan(&result)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return 0
		}
		helper.Check(err)
		return 0
	}
	return result
}

// AddStatTraffic atomically adds the given amount of traffic, but only if the
// stored beginning of traffic counting equals expectedSince. If it differs
// (because the statistics were reset by another node), the amount is discarded.
// It returns the stored total and the stored beginning of counting
func (p DatabaseProvider) AddStatTraffic(addedTraffic uint64, expectedSince int64) (uint64, int64) {
	tx, err := p.sqliteDb.Begin()
	if err != nil {
		helper.Check(err)
		return 0, 0
	}
	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback()
		}
	}()

	storedSince, exists := getTrafficSinceTx(tx)
	if !exists {
		// First time traffic is recorded, create the epoch with what the node
		// expects. If another node created it in the meantime, re-read the value
		_, err = tx.Exec(`INSERT INTO Statistics (type, value) VALUES (?, ?)
					ON CONFLICT(type) DO NOTHING`, statIdTrafficSince, expectedSince)
		if err != nil {
			helper.Check(err)
			return 0, 0
		}
		storedSince, exists = getTrafficSinceTx(tx)
		if !exists {
			storedSince = expectedSince
		}
	}

	if storedSince == expectedSince && addedTraffic > 0 {
		_, err = tx.Exec(`INSERT INTO Statistics (type, value) VALUES (?, ?)
					ON CONFLICT(type) DO UPDATE SET value = value + ?`, statIdTraffic, addedTraffic, addedTraffic)
		if err != nil {
			helper.Check(err)
			return 0, 0
		}
	}

	var total uint64
	row := tx.QueryRow("SELECT value FROM Statistics WHERE type = ?", statIdTraffic)
	err = row.Scan(&total)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		helper.Check(err)
		return 0, 0
	}

	err = tx.Commit()
	if err != nil {
		helper.Check(err)
		return 0, 0
	}
	committed = true
	return total, storedSince
}

// ResetStatTraffic atomically sets the total traffic to zero and the beginning
// of traffic counting to the given value
func (p DatabaseProvider) ResetStatTraffic(since int64) {
	tx, err := p.sqliteDb.Begin()
	if err != nil {
		helper.Check(err)
		return
	}
	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback()
		}
	}()

	_, err = tx.Exec(`INSERT INTO Statistics (type, value) VALUES (?, ?)
					ON CONFLICT(type) DO UPDATE SET value = 0`, statIdTraffic, 0)
	if err != nil {
		helper.Check(err)
		return
	}
	_, err = tx.Exec(`INSERT INTO Statistics (type, value) VALUES (?, ?)
					ON CONFLICT(type) DO UPDATE SET value = ?`, statIdTrafficSince, since, since)
	if err != nil {
		helper.Check(err)
		return
	}
	err = tx.Commit()
	if err != nil {
		helper.Check(err)
		return
	}
	committed = true
}

// InitTrafficSince stores the beginning of traffic counting if it has not been
// stored yet and returns the value that is currently stored
func (p DatabaseProvider) InitTrafficSince(since int64) int64 {
	_, err := p.sqliteDb.Exec(`INSERT INTO Statistics (type, value) VALUES (?, ?)
					ON CONFLICT(type) DO NOTHING`, statIdTrafficSince, since)
	helper.Check(err)
	result, ok := p.GetTrafficSince()
	if !ok {
		return since
	}
	return result
}

// GetTrafficSince gets the beginning of traffic counting
func (p DatabaseProvider) GetTrafficSince() (int64, bool) {
	var result int64
	row := p.sqliteDb.QueryRow("SELECT value FROM Statistics WHERE type = ?", statIdTrafficSince)
	err := row.Scan(&result)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return 0, false
		}
		helper.Check(err)
		return 0, false
	}
	return result, true
}

func getTrafficSinceTx(tx *sql.Tx) (int64, bool) {
	var result int64
	row := tx.QueryRow("SELECT value FROM Statistics WHERE type = ?", statIdTrafficSince)
	err := row.Scan(&result)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return 0, false
		}
		helper.Check(err)
		return 0, false
	}
	return result, true
}
