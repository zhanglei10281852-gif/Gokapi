package sqlite

import (
	"database/sql"
	"errors"
	"time"

	"github.com/forceu/gokapi/internal/helper"
	"github.com/forceu/gokapi/internal/models"
)

// maxRotationHops limits how many rotation pointers are followed when looking
// up a successor or deleting a rotation chain. Successor IDs are freshly
// generated random values, so cycles cannot occur in practice; this is a
// defensive bound only.
const maxRotationHops = 5

type schemaSessions struct {
	Id         string
	RenewAt    int64
	ValidUntil int64
	UserId     int
}

// GetSession returns the session with the given ID or false if not a valid ID
func (p DatabaseProvider) GetSession(id string) (models.Session, bool) {
	var rowResult schemaSessions
	row := p.sqliteDb.QueryRow("SELECT * FROM Sessions WHERE Id = ?", id)
	err := row.Scan(&rowResult.Id, &rowResult.RenewAt, &rowResult.ValidUntil, &rowResult.UserId)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return models.Session{}, false
		}
		helper.Check(err)
		return models.Session{}, false
	}
	result := models.Session{
		RenewAt:    rowResult.RenewAt,
		ValidUntil: rowResult.ValidUntil,
		UserId:     rowResult.UserId,
	}
	return result, true
}

// SaveSession stores the given session. After the expiry passed, it will be deleted automatically
func (p DatabaseProvider) SaveSession(id string, session models.Session) {
	newData := schemaSessions{
		Id:         id,
		RenewAt:    session.RenewAt,
		ValidUntil: session.ValidUntil,
		UserId:     session.UserId,
	}

	_, err := p.sqliteDb.Exec("INSERT OR REPLACE INTO Sessions (Id, RenewAt, ValidUntil, UserId) VALUES (?, ?, ?, ?)",
		newData.Id, newData.RenewAt, newData.ValidUntil, newData.UserId)
	helper.Check(err)
}

// RotateSession atomically replaces the session oldId with the single successor
// newId. The transaction is started in IMMEDIATE mode, so concurrent invocations
// from multiple connections or instances are serialized:
//   - the first invocation creates the successor and records the rotation,
//   - all following invocations for the same oldId receive that same successor,
//   - if oldId was deleted (logout, password reset, user deletion) before the
//     transition starts, no successor is created and false is returned.
func (p DatabaseProvider) RotateSession(oldId string, newId string, newSession models.Session) (string, bool) {
	txn, err := p.sqliteDb.Begin()
	if err != nil {
		helper.Check(err)
		return "", false
	}
	committed := false
	defer func() {
		if !committed {
			_ = txn.Rollback()
		}
	}()

	// oldId was already rotated: return the existing single successor
	var successorId string
	err = txn.QueryRow("SELECT NewId FROM SessionRotations WHERE OldId = ?", oldId).Scan(&successorId)
	if err == nil {
		helper.Check(txn.Commit())
		committed = true
		return successorId, true
	}
	if !errors.Is(err, sql.ErrNoRows) {
		helper.Check(err)
		return "", false
	}

	// A rotation is only valid if the predecessor session still exists.
	// If it was revoked or expired between the read and this transition,
	// the login state must not be recreated.
	var existingId string
	err = txn.QueryRow("SELECT Id FROM Sessions WHERE Id = ?", oldId).Scan(&existingId)
	if errors.Is(err, sql.ErrNoRows) {
		_ = txn.Rollback()
		return "", false
	}
	if err != nil {
		helper.Check(err)
		return "", false
	}

	_, err = txn.Exec("INSERT INTO Sessions (Id, RenewAt, ValidUntil, UserId) VALUES (?, ?, ?, ?)",
		newId, newSession.RenewAt, newSession.ValidUntil, newSession.UserId)
	if err != nil {
		helper.Check(err)
		return "", false
	}
	_, err = txn.Exec("INSERT INTO SessionRotations (OldId, NewId, ValidUntil) VALUES (?, ?, ?)",
		oldId, newId, newSession.ValidUntil)
	if err != nil {
		helper.Check(err)
		return "", false
	}
	_, err = txn.Exec("DELETE FROM Sessions WHERE Id = ?", oldId)
	helper.Check(err)
	helper.Check(txn.Commit())
	committed = true
	return newId, true
}

// GetSessionSuccessor follows the rotation pointer of an ID that has already
// been replaced and returns its currently valid successor. Expired successors
// or pointers to revoked sessions result in false.
func (p DatabaseProvider) GetSessionSuccessor(oldId string) (string, models.Session, bool) {
	currentId := oldId
	visited := map[string]bool{oldId: true}
	for i := 0; i < maxRotationHops; i++ {
		var nextId string
		err := p.sqliteDb.QueryRow("SELECT NewId FROM SessionRotations WHERE OldId = ?", currentId).Scan(&nextId)
		if errors.Is(err, sql.ErrNoRows) {
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

// DeleteSession deletes a session with the given ID. When the session was
// rotated, every successor of the chain and all related rotation records are
// deleted within the same transaction, so that a revocation always wins
// against a concurrent renewal - regardless of their execution order.
func (p DatabaseProvider) DeleteSession(id string) {
	txn, err := p.sqliteDb.Begin()
	if err != nil {
		helper.Check(err)
		return
	}
	committed := false
	defer func() {
		if !committed {
			_ = txn.Rollback()
		}
	}()

	currentId := id
	visited := map[string]bool{}
	for i := 0; i < maxRotationHops; i++ {
		if visited[currentId] {
			break
		}
		visited[currentId] = true
		_, err := txn.Exec("DELETE FROM Sessions WHERE Id = ?", currentId)
		helper.Check(err)
		// Remove pointers of earlier rotations that target this id
		_, err = txn.Exec("DELETE FROM SessionRotations WHERE NewId = ?", currentId)
		helper.Check(err)

		var nextId string
		err = txn.QueryRow("SELECT NewId FROM SessionRotations WHERE OldId = ?", currentId).Scan(&nextId)
		_, _ = txn.Exec("DELETE FROM SessionRotations WHERE OldId = ?", currentId)
		if errors.Is(err, sql.ErrNoRows) {
			break
		}
		if err != nil {
			helper.Check(err)
			return
		}
		currentId = nextId
	}
	helper.Check(txn.Commit())
	committed = true
}

// DeleteAllSessions logs all users out
func (p DatabaseProvider) DeleteAllSessions() {
	txn, err := p.sqliteDb.Begin()
	if err != nil {
		helper.Check(err)
		return
	}
	committed := false
	defer func() {
		if !committed {
			_ = txn.Rollback()
		}
	}()
	//goland:noinspection SqlWithoutWhere
	_, err = txn.Exec("DELETE FROM SessionRotations")
	helper.Check(err)
	//goland:noinspection SqlWithoutWhere
	_, err = txn.Exec("DELETE FROM Sessions")
	helper.Check(err)
	helper.Check(txn.Commit())
	committed = true
}

// DeleteAllSessionsByUser logs the specific users out. The rotation records
// and the sessions are removed in one transaction, so a rotation that ran
// before this call loses its successor rows, while a rotation running after
// it finds the predecessor missing and cannot recreate a session.
func (p DatabaseProvider) DeleteAllSessionsByUser(userId int) {
	txn, err := p.sqliteDb.Begin()
	if err != nil {
		helper.Check(err)
		return
	}
	committed := false
	defer func() {
		if !committed {
			_ = txn.Rollback()
		}
	}()
	_, err = txn.Exec(`DELETE FROM SessionRotations
		WHERE OldId IN (SELECT Id FROM Sessions WHERE UserId = ?)
		   OR NewId IN (SELECT Id FROM Sessions WHERE UserId = ?)`, userId, userId)
	helper.Check(err)
	_, err = txn.Exec("DELETE FROM Sessions WHERE UserId = ?", userId)
	helper.Check(err)
	helper.Check(txn.Commit())
	committed = true
}

func (p DatabaseProvider) cleanExpiredSessions() {
	now := time.Now().Unix()
	_, err := p.sqliteDb.Exec("DELETE FROM Sessions WHERE Sessions.ValidUntil < ?", now)
	helper.Check(err)
	_, err = p.sqliteDb.Exec("DELETE FROM SessionRotations WHERE SessionRotations.ValidUntil < ?", now)
	helper.Check(err)
}
