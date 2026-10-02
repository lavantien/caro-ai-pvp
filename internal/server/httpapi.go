package server

// The HTTP transport of the M6a backend over the landed domain: the auth
// store and the room manager behind one stdlib ServeMux. Bodies are JSON,
// errors ride the stable envelope below, acting routes require the session
// cookie, and guests read rooms per first-cause.md Implication 1.1.

import (
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"time"
)

// Transport-local knobs config does not carry yet (config gap, reported
// with the milestone): the auth cookie key and the ceiling on one request
// body read, so a hostile client cannot buffer unbounded memory.
const (
	sessionCookieName  = "caro_session"
	httpBodyLimitBytes = 64 << 10
)

// Stable wire error codes of the {error, message} envelope.
const (
	codeBadCredentials  = "bad_credentials"
	codeInvalidUsername = "invalid_username"
	codeUnauthorized    = "unauthorized"
	codeBadRequest      = "bad_request"
	codeNotParticipant  = "not_participant"
	codeRoomNotFound    = "room_not_found"
	codeRoomFull        = "room_full"
	codeRoomClosed      = "room_closed"
	codeNotReady        = "not_ready"
	codeNotYourTurn     = "not_your_turn"
	codeIllegalMove     = "illegal_move"
	codeSeriesFinished  = "series_finished"
	codeInternal        = "internal"
)

// errorResponse maps the package sentinels onto wire code and status.
// Classes: 401 the caller must log in, 400 the request itself is unusable,
// 403 the caller is real but not a party to the room, 404 the target does
// not exist, 409 the target exists but the transition is refused now, 500
// the server failed.
//
//	ErrBadCredentials        bad_credentials   401  wrong password, login only
//	missing or dead token    unauthorized      401  protected route, no live session
//	ErrInvalidUsername       invalid_username  400
//	ErrBadTimeControl        bad_request       400  settings the config hubs reject
//	ErrBadSeriesLength       bad_request       400
//	ErrSamePlayer            bad_request       400  the host joining its own room
//	malformed body or cell   bad_request       400  decode and rules.ParseCell failures
//	ErrNotParticipant        not_participant   403
//	ErrRoomNotFound          room_not_found    404
//	ErrRoomFull              room_full         409
//	ErrRoomClosed            room_closed       409
//	ErrNotReady              not_ready         409
//	ErrNotYourTurn           not_your_turn     409
//	ErrIllegalMove           illegal_move      409
//	ErrSeriesFinished        series_finished   409  defensive: rooms retire at finish
//	anything else            internal          500  store and queue failures
func errorResponse(err error) (code string, status int) {
	switch {
	case errors.Is(err, ErrBadCredentials):
		return codeBadCredentials, http.StatusUnauthorized
	case errors.Is(err, ErrInvalidUsername):
		return codeInvalidUsername, http.StatusBadRequest
	case errors.Is(err, ErrRoomNotFound):
		return codeRoomNotFound, http.StatusNotFound
	case errors.Is(err, ErrRoomFull):
		return codeRoomFull, http.StatusConflict
	case errors.Is(err, ErrRoomClosed):
		return codeRoomClosed, http.StatusConflict
	case errors.Is(err, ErrNotYourTurn):
		return codeNotYourTurn, http.StatusConflict
	case errors.Is(err, ErrIllegalMove):
		return codeIllegalMove, http.StatusConflict
	case errors.Is(err, ErrNotParticipant):
		return codeNotParticipant, http.StatusForbidden
	case errors.Is(err, ErrNotReady):
		return codeNotReady, http.StatusConflict
	case errors.Is(err, ErrSeriesFinished):
		return codeSeriesFinished, http.StatusConflict
	case errors.Is(err, ErrBadTimeControl),
		errors.Is(err, ErrBadSeriesLength),
		errors.Is(err, ErrSamePlayer),
		errors.Is(err, ErrBadOwner):
		return codeBadRequest, http.StatusBadRequest
	default:
		return codeInternal, http.StatusInternalServerError
	}
}

type apiError struct {
	Error   string `json:"error"`
	Message string `json:"message"`
}

func writeError(w http.ResponseWriter, status int, code, message string) {
	writeJSON(w, status, apiError{Error: code, Message: message})
}

// writeDomainError renders an error through the mapping; internal failures
// keep a generic message so nothing about the store leaks.
func writeDomainError(w http.ResponseWriter, err error) {
	code, status := errorResponse(err)
	message := err.Error()
	if status == http.StatusInternalServerError {
		message = "internal error"
	}
	writeError(w, status, code, message)
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

// decodeJSON reads one JSON value from the request body within the limit.
func decodeJSON(r *http.Request, dst any) error {
	return json.NewDecoder(io.LimitReader(r.Body, httpBodyLimitBytes)).Decode(dst)
}

// userSummary is the logged-in home line: the login and /api/me body.
type userSummary struct {
	Username string `json:"username"`
	Rating   int    `json:"rating"`
	Wins     int    `json:"wins"`
	Losses   int    `json:"losses"`
	Draws    int    `json:"draws"`
	Level    int    `json:"level"`
}

// apiServer carries the transport's two dependencies.
type apiServer struct {
	store *Store
	rooms *RoomManager
}

// NewHTTPAPI builds the M6a JSON surface over store and rooms.
func NewHTTPAPI(store *Store, rooms *RoomManager) http.Handler {
	a := &apiServer{store: store, rooms: rooms}
	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/login", a.handleLogin)
	mux.HandleFunc("POST /api/logout", a.handleLogout)
	mux.HandleFunc("GET /api/me", a.requireSession(a.handleMe))
	return mux
}

// handleLogin is the one-form auth of the spec: an unknown username creates
// the account, a known one verifies. Success sets the HttpOnly session
// cookie and returns the summary line.
func (a *apiServer) handleLogin(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, codeBadRequest, "malformed login body")
		return
	}
	u, sess, err := LoginOrCreate(a.store, req.Username, req.Password, time.Now().Unix())
	if err != nil {
		writeDomainError(w, err)
		return
	}
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieName,
		Value:    hex.EncodeToString(sess.Token),
		Path:     "/",
		MaxAge:   int(sessionTTLSec),
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})
	a.writeUserSummary(w, http.StatusOK, u)
}

// handleLogout drops the token behind the cookie and expires the cookie.
// Without a usable cookie it is still 204: logout races natural expiry.
func (a *apiServer) handleLogout(w http.ResponseWriter, r *http.Request) {
	if token, err := sessionToken(r); err == nil {
		if err := Logout(a.store, token); err != nil {
			writeDomainError(w, err)
			return
		}
	}
	http.SetCookie(w, &http.Cookie{
		Name: sessionCookieName, Value: "", Path: "/", MaxAge: -1, HttpOnly: true,
	})
	w.WriteHeader(http.StatusNoContent)
}

// handleMe answers the session's own summary line.
func (a *apiServer) handleMe(w http.ResponseWriter, _ *http.Request, u User) {
	a.writeUserSummary(w, http.StatusOK, u)
}

// writeUserSummary reads the rating chain tail and the W-L-D plus level
// totals of one user and writes the line.
func (a *apiServer) writeUserSummary(w http.ResponseWriter, status int, u User) {
	rating, err := currentRating(a.store, u.ID)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	stats, err := a.store.UserStats(u.ID)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	writeJSON(w, status, userSummary{
		Username: u.Username, Rating: rating,
		Wins: stats.Wins, Losses: stats.Losses, Draws: stats.Draws, Level: stats.SeriesWon,
	})
}

// requireSession is the middleware of acting routes: parse the cookie,
// authenticate the token against the store with one clock read, hand the
// resolved user down. Everything else gets the 401 unauthorized envelope.
func (a *apiServer) requireSession(h func(http.ResponseWriter, *http.Request, User)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		token, err := sessionToken(r)
		if err != nil {
			writeError(w, http.StatusUnauthorized, codeUnauthorized, "login required")
			return
		}
		u, err := Authenticate(a.store, token, time.Now().Unix())
		if err != nil {
			if errors.Is(err, ErrBadCredentials) {
				writeError(w, http.StatusUnauthorized, codeUnauthorized, "login required")
				return
			}
			writeError(w, http.StatusInternalServerError, codeInternal, "authentication failed")
			return
		}
		h(w, r, u)
	}
}

// sessionToken hex-decodes the session cookie value.
func sessionToken(r *http.Request) ([]byte, error) {
	c, err := r.Cookie(sessionCookieName)
	if err != nil {
		return nil, err
	}
	return hex.DecodeString(c.Value)
}
