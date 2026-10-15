package sessionmanager

/**
Manages the sessions for the admin user or to access password-protected files
*/

import (
	"net/http"
	"time"

	"github.com/forceu/gokapi/internal/configuration/database"
	"github.com/forceu/gokapi/internal/helper"
	"github.com/forceu/gokapi/internal/models"
)

// If no login occurred during this time, the admin session will be deleted. Default 30 days
const cookieLifeAdmin = 30 * 24 * time.Hour
const lengthSessionId = 60

// IsValidSession checks if the user is submitting a valid session token
// If a valid session is found, useSession will be called
// Returns true if authenticated, otherwise false
func IsValidSession(w http.ResponseWriter, r *http.Request, isOauth bool, OAuthRecheckInterval int) (models.User, bool) {
	cookie, err := r.Cookie("session_token")
	if err == nil {
		sessionString := cookie.Value
		if sessionString != "" {
			session, ok := database.GetSession(sessionString)
			if !ok {
				// The session may just have been replaced by a concurrent renewal.
				// Follow the single successor instead of creating another session,
				// so that concurrent requests share the same rotation result.
				return useSuccessorSession(w, sessionString)
			}
			user, userExists := database.GetUser(session.UserId)
			if !userExists {
				return user, false
			}
			return user, useSession(w, sessionString, session, isOauth, OAuthRecheckInterval)
		}
	}
	return models.User{}, false
}

// useSuccessorSession authenticates a request that presented an already
// rotated session id by following its successor pointer. It never creates a
// new session and returns false if no valid successor exists.
func useSuccessorSession(w http.ResponseWriter, oldSessionId string) (models.User, bool) {
	successorId, successor, ok := database.GetSessionSuccessor(oldSessionId)
	if !ok {
		return models.User{}, false
	}
	user, userExists := database.GetUser(successor.UserId)
	if !userExists {
		return user, false
	}
	writeSessionCookie(w, successorId, time.Unix(successor.ValidUntil, 0))
	go database.UpdateUserLastOnline(successor.UserId)
	return user, true
}

// useSession checks if a session is still valid. It Changes the session string
// if it has been used for more than an hour to limit session hijacking
// Returns true if session is still valid
// Returns false if session is invalid (and deletes it)
func useSession(w http.ResponseWriter, id string, session models.Session, isOauth bool, OAuthRecheckInterval int) bool {
	if session.ValidUntil < time.Now().Unix() {
		database.DeleteSession(id)
		return false
	}
	if session.RenewAt < time.Now().Unix() {
		return renewSession(w, id, session, isOauth, OAuthRecheckInterval)
	}
	go database.UpdateUserLastOnline(session.UserId)
	return true
}

// renewSession performs the single-successor rotation of a session that has
// reached its RenewAt. Concurrent requests either receive the exact same
// successor or are forced to re-authenticate, but they can never create
// multiple long-lived sessions. If a revocation (logout, password reset,
// user deletion or revoke-all-sessions) became effective after the caller
// read the old session state, no valid session is recreated and no session
// cookie is issued.
func renewSession(w http.ResponseWriter, oldId string, oldSession models.Session, isOauth bool, oAuthRecheckInterval int) bool {
	validUntil := time.Now().Add(cookieLifeAdmin)
	if isOauth {
		validUntil = time.Now().Add(time.Duration(oAuthRecheckInterval) * time.Hour)
	}
	newSession := models.Session{
		RenewAt:    time.Now().Add(12 * time.Hour).Unix(),
		ValidUntil: validUntil.Unix(),
		UserId:     oldSession.UserId,
	}
	successorId, rotated := database.RotateSession(oldId, helper.GenerateRandomString(lengthSessionId), newSession)
	if !rotated {
		// Predecessor was revoked or expired between reading and rotation
		return false
	}
	// A revocation may have become effective concurrently, in which case the
	// successor was deleted as well - do not hand out a cookie for a dead session
	successor, exists := database.GetSession(successorId)
	if !exists || successor.ValidUntil < time.Now().Unix() {
		return false
	}
	writeSessionCookie(w, successorId, time.Unix(successor.ValidUntil, 0))
	go database.UpdateUserLastOnline(successor.UserId)
	return true
}

// CreateSession creates a new session - called after login with correct username / password
// If sessions parameter is nil, it will be loaded from config
func CreateSession(w http.ResponseWriter, isOauth bool, OAuthRecheckInterval int, userId int) {
	timeExpiry := time.Now().Add(cookieLifeAdmin)
	if isOauth {
		timeExpiry = time.Now().Add(time.Duration(OAuthRecheckInterval) * time.Hour)
	}

	sessionString := helper.GenerateRandomString(lengthSessionId)
	database.SaveSession(sessionString, models.Session{
		RenewAt:    time.Now().Add(12 * time.Hour).Unix(),
		ValidUntil: timeExpiry.Unix(),
		UserId:     userId,
	})
	writeSessionCookie(w, sessionString, timeExpiry)
}

// LogoutSession logs out user and deletes session
func LogoutSession(w http.ResponseWriter, r *http.Request) {
	cookie, err := r.Cookie("session_token")
	if err == nil {
		database.DeleteSession(cookie.Value)
	}
	writeSessionCookie(w, "", time.Now())
}

// Writes session cookie to browser
func writeSessionCookie(w http.ResponseWriter, sessionString string, expiry time.Time) {
	c := &http.Cookie{
		Name:     "session_token",
		Value:    sessionString,
		Expires:  expiry,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	}
	http.SetCookie(w, c)
}
