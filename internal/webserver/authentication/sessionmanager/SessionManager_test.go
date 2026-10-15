package sessionmanager

import (
	"sync"
	"testing"
	"time"

	"net/http"
	"net/http/httptest"
	"os"

	"github.com/forceu/gokapi/internal/configuration"
	"github.com/forceu/gokapi/internal/configuration/database"
	"github.com/forceu/gokapi/internal/models"
	"github.com/forceu/gokapi/internal/test"
	"github.com/forceu/gokapi/internal/test/testconfiguration"
)

var newSession string

func TestMain(m *testing.M) {
	testconfiguration.Create(false)
	configuration.Load()
	configuration.ConnectDatabase()
	exitVal := m.Run()
	testconfiguration.Delete()
	os.Exit(exitVal)
}

func getRecorder(cookies []test.Cookie) (*httptest.ResponseRecorder, *http.Request, bool, int) {
	w, r := test.GetRecorder("GET", "/", cookies, nil, nil)
	return w, r, false, 1
}

func TestIsValidSession(t *testing.T) {
	user, ok := IsValidSession(getRecorder(nil))
	test.IsEqualBool(t, ok, false)
	user, ok = IsValidSession(getRecorder([]test.Cookie{{
		Name:  "session_token",
		Value: "invalid"},
	}))
	test.IsEqualBool(t, ok, false)
	user, ok = IsValidSession(getRecorder([]test.Cookie{{
		Name:  "session_token",
		Value: ""},
	}))
	test.IsEqualBool(t, ok, false)
	user, ok = IsValidSession(getRecorder([]test.Cookie{{
		Name:  "session_token",
		Value: "expiredsession"},
	}))
	test.IsEqualBool(t, ok, false)
	user, ok = IsValidSession(getRecorder([]test.Cookie{{
		Name:  "session_token",
		Value: "validsession"},
	}))
	test.IsEqualBool(t, ok, true)
	_, ok = IsValidSession(getRecorder([]test.Cookie{{
		Name:  "session_token",
		Value: "validSessionInvalidUser"},
	}))
	test.IsEqualBool(t, ok, false)
	test.IsEqualInt(t, user.Id, 7)
	w, r, _, _ := getRecorder([]test.Cookie{{
		Name:  "session_token",
		Value: "needsRenewal"},
	})
	user, ok = IsValidSession(w, r, false, 1)
	cookies := w.Result().Cookies()
	test.IsEqualInt(t, len(cookies), 1)
	test.IsEqualString(t, cookies[0].Name, "session_token")
	session := cookies[0].Value
	test.IsEqualInt(t, len(session), 60)
	test.IsNotEqualString(t, session, "needsRenewal")
}

func TestCreateSession(t *testing.T) {
	w, _, _, _ := getRecorder(nil)
	CreateSession(w, false, 1, 5)
	cookies := w.Result().Cookies()
	test.IsEqualInt(t, len(cookies), 1)
	test.IsEqualString(t, cookies[0].Name, "session_token")
	newSession = cookies[0].Value
	test.IsEqualInt(t, len(newSession), 60)

	user, ok := IsValidSession(getRecorder([]test.Cookie{{
		Name:  "session_token",
		Value: newSession},
	}))
	test.IsEqualBool(t, ok, true)
	test.IsEqualInt(t, user.Id, 5)

	w, _, _, _ = getRecorder(nil)
	CreateSession(w, true, 20, 50)
	cookies = w.Result().Cookies()
	newOauthSession := cookies[0].Value

	var session models.Session
	session, ok = database.GetSession(newOauthSession)
	test.IsEqualBool(t, ok, true)
	isEqual := time.Now().Add(20*time.Hour).Unix()-session.ValidUntil < 10 &&
		time.Now().Add(20*time.Hour).Unix()-session.ValidUntil > -1
	test.IsEqualBool(t, isEqual, true)
}

func TestLogoutSession(t *testing.T) {
	user, ok := IsValidSession(getRecorder([]test.Cookie{{
		Name:  "session_token",
		Value: newSession},
	}))
	test.IsEqualBool(t, ok, true)
	test.IsEqualInt(t, user.Id, 5)
	w, r, _, _ := getRecorder([]test.Cookie{{
		Name:  "session_token",
		Value: newSession},
	})
	LogoutSession(w, r)
	_, ok = IsValidSession(getRecorder([]test.Cookie{{
		Name:  "session_token",
		Value: newSession},
	}))
	test.IsEqualBool(t, ok, false)
}

// TestConcurrentRenewal verifies that many concurrent requests hitting the
// RenewAt boundary all receive the exact same successor session instead of
// each creating their own long-lived session.
func TestConcurrentRenewal(t *testing.T) {
	// Clear leftovers from a previous run, as open database files may prevent
	// test directory cleanup on some platforms
	database.DeleteSession("concRenewal")
	database.SaveSession("concRenewal", models.Session{
		RenewAt:    0,
		ValidUntil: 2147483646,
		UserId:     7,
	})
	const count = 25
	cookies := make([]string, count)
	oks := make([]bool, count)
	var wg sync.WaitGroup
	start := make(chan struct{})
	for i := 0; i < count; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			w, r, _, _ := getRecorder([]test.Cookie{{
				Name:  "session_token",
				Value: "concRenewal"},
			})
			<-start
			_, oks[i] = IsValidSession(w, r, false, 1)
			resultCookies := w.Result().Cookies()
			if len(resultCookies) == 1 && resultCookies[0].Name == "session_token" {
				cookies[i] = resultCookies[0].Value
			}
		}(i)
	}
	close(start)
	wg.Wait()

	successor := ""
	for i := 0; i < count; i++ {
		test.IsEqualBool(t, oks[i], true)
		test.IsEqualInt(t, len(cookies[i]), lengthSessionId)
		if successor == "" {
			successor = cookies[i]
		}
		test.IsEqualString(t, cookies[i], successor)
	}
	_, ok := database.GetSession("concRenewal")
	test.IsEqualBool(t, ok, false)
	successorSession, ok := database.GetSession(successor)
	test.IsEqualBool(t, ok, true)
	test.IsEqualInt(t, successorSession.UserId, 7)
	// The successor is a regular long-lived session that does not require
	// another immediate renewal
	test.IsEqualBool(t, successorSession.RenewAt > time.Now().Unix(), true)
	database.DeleteSession(successor)
}

// TestRenewalAfterRevocation verifies that once a local revocation became
// effective, a late request that read the old session state cannot recreate
// a valid session.
func TestRenewalAfterRevocation(t *testing.T) {
	// Clear leftovers from a previous run, as open database files may prevent
	// test directory cleanup on some platforms
	database.DeleteSession("revokedBeforeRenew")
	database.DeleteSession("logoutDuringRenew")
	database.DeleteSession("revokeByUser")

	// Revocation (logout) effective before the renewal transition
	database.SaveSession("revokedBeforeRenew", models.Session{
		RenewAt:    0,
		ValidUntil: 2147483646,
		UserId:     7,
	})
	database.DeleteSession("revokedBeforeRenew")
	_, ok := IsValidSession(getRecorder([]test.Cookie{{
		Name:  "session_token",
		Value: "revokedBeforeRenew"},
	}))
	test.IsEqualBool(t, ok, false)

	// Logout with the old id after a rotation must invalidate the successor
	// session as well, and no late request may restore it
	database.SaveSession("logoutDuringRenew", models.Session{
		RenewAt:    0,
		ValidUntil: 2147483646,
		UserId:     7,
	})
	w, r, _, _ := getRecorder([]test.Cookie{{
		Name:  "session_token",
		Value: "logoutDuringRenew"},
	})
	_, ok = IsValidSession(w, r, false, 1)
	test.IsEqualBool(t, ok, true)
	resultCookies := w.Result().Cookies()
	test.IsEqualInt(t, len(resultCookies), 1)
	successorCookie := resultCookies[0].Value
	test.IsEqualInt(t, len(successorCookie), lengthSessionId)

	logoutW, logoutR, _, _ := getRecorder([]test.Cookie{{
		Name:  "session_token",
		Value: "logoutDuringRenew"},
	})
	LogoutSession(logoutW, logoutR)

	_, ok = database.GetSession(successorCookie)
	test.IsEqualBool(t, ok, false)
	_, _, ok = database.GetSessionSuccessor("logoutDuringRenew")
	test.IsEqualBool(t, ok, false)
	_, ok = IsValidSession(getRecorder([]test.Cookie{{
		Name:  "session_token",
		Value: successorCookie},
	}))
	test.IsEqualBool(t, ok, false)
	_, ok = IsValidSession(getRecorder([]test.Cookie{{
		Name:  "session_token",
		Value: "logoutDuringRenew"},
	}))
	test.IsEqualBool(t, ok, false)

	// Revoke-all-sessions-by-user must also defeat concurrent renewals
	database.SaveSession("revokeByUser", models.Session{
		RenewAt:    0,
		ValidUntil: 2147483646,
		UserId:     7,
	})
	w, r, _, _ = getRecorder([]test.Cookie{{
		Name:  "session_token",
		Value: "revokeByUser"},
	})
	_, ok = IsValidSession(w, r, false, 1)
	test.IsEqualBool(t, ok, true)
	database.DeleteAllSessionsByUser(7)
	// User 7 also owns the static test sessions; recreate one for other tests
	_, ok = IsValidSession(getRecorder([]test.Cookie{{
		Name:  "session_token",
		Value: w.Result().Cookies()[0].Value},
	}))
	test.IsEqualBool(t, ok, false)
	database.SaveSession("validsession", models.Session{
		RenewAt:    2147483645,
		ValidUntil: 2147483646,
		UserId:     7,
	})
}
