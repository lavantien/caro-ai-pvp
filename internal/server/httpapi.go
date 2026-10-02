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
	"strings"
	"time"

	"github.com/lavantien/caro-ai-pvp/internal/config"
	"github.com/lavantien/caro-ai-pvp/internal/rules"
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

// roomSummary is one rooms-grid line.
type roomSummary struct {
	ID          string `json:"id"`
	HostUserID  int64  `json:"hostUserId"`
	GuestUserID int64  `json:"guestUserId"`
	TCIdx       int    `json:"tcIdx"`
	BOLen       int    `json:"boLen"`
	State       string `json:"state"`
	VsBotTier   string `json:"vsBotTier,omitempty"`
	HostWins    int    `json:"hostWins"`
	GuestWins   int    `json:"guestWins"`
	CreatedAt   int64  `json:"createdAt"`
}

// gameSnapshot is the live-game section of the room detail: the board as
// the ordered stone list a client replays, the side and seat to move, the
// red seat of the rotation, and both banks in milliseconds.
type gameSnapshot struct {
	Moves      []string `json:"moves"`
	Turn       string   `json:"turn"`
	TurnUserID int64    `json:"turnUserId"`
	RedUserID  int64    `json:"redUserId"`
	ClockMs    [2]int64 `json:"clockMs"`
}

// roomDetail is the public room view: the grid line plus the live game.
type roomDetail struct {
	roomSummary
	Game *gameSnapshot `json:"game"`
}

// apiServer carries the transport's two dependencies plus the SSE keepalive
// cadence, a field so tests can tighten it.
type apiServer struct {
	store     *Store
	rooms     *RoomManager
	keepalive time.Duration
}

// sseKeepalive is the default idle cadence of the event streams: a comment
// frame every 15s holds proxies and browsers on an idle connection (config
// carries no SSE constant yet, reported with the milestone).
const sseKeepalive = 15 * time.Second

// sseKeepaliveComment is the idle frame: a comment line plus the blank
// terminator, invisible to the SSE event stream.
const sseKeepaliveComment = ": keepalive\n\n"

// NewHTTPAPI builds the M6a JSON+SSE surface over store and rooms.
func NewHTTPAPI(store *Store, rooms *RoomManager) http.Handler {
	a := &apiServer{store: store, rooms: rooms, keepalive: sseKeepalive}
	return a.routes()
}

func (a *apiServer) routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/login", a.handleLogin)
	mux.HandleFunc("POST /api/logout", a.handleLogout)
	mux.HandleFunc("GET /api/me", a.requireSession(a.handleMe))
	mux.HandleFunc("GET /api/history", a.requireSession(a.handleHistory))
	mux.HandleFunc("GET /api/rooms", a.handleListRooms)
	mux.HandleFunc("POST /api/rooms", a.requireSession(a.handleCreateRoom))
	mux.HandleFunc("GET /api/rooms/{id}", a.handleRoomDetail)
	mux.HandleFunc("GET /api/rooms/{id}/events", a.handleRoomEvents)
	mux.HandleFunc("POST /api/rooms/{id}/join", a.requireSession(a.handleJoin))
	mux.HandleFunc("POST /api/rooms/{id}/ready", a.requireSession(a.handleReady))
	mux.HandleFunc("POST /api/rooms/{id}/move", a.requireSession(a.handleMove))
	mux.HandleFunc("POST /api/rooms/{id}/forfeit", a.requireSession(a.handleForfeit))
	return mux
}

// handleJoin seats the session's user as the open room's opponent.
func (a *apiServer) handleJoin(w http.ResponseWriter, r *http.Request, u User) {
	room, ok := a.roomFromRequest(w, r)
	if !ok {
		return
	}
	if err := a.rooms.Join(room.ID(), u.ID); err != nil {
		writeDomainError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// handleReady marks one participant's handshake.
func (a *apiServer) handleReady(w http.ResponseWriter, r *http.Request, u User) {
	room, ok := a.roomFromRequest(w, r)
	if !ok {
		return
	}
	if err := room.Ready(u.ID); err != nil {
		writeDomainError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// handleMove lands one stone by coordinate name through the rules codec.
func (a *apiServer) handleMove(w http.ResponseWriter, r *http.Request, u User) {
	room, ok := a.roomFromRequest(w, r)
	if !ok {
		return
	}
	var req struct {
		Cell string `json:"cell"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, codeBadRequest, "malformed move body")
		return
	}
	cell, err := rules.ParseCell(req.Cell)
	if err != nil {
		writeError(w, http.StatusBadRequest, codeBadRequest, err.Error())
		return
	}
	if err := room.PlayMove(u.ID, cell); err != nil {
		writeDomainError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// handleForfeit bills a mid-series quit or retires an open room.
func (a *apiServer) handleForfeit(w http.ResponseWriter, r *http.Request, u User) {
	room, ok := a.roomFromRequest(w, r)
	if !ok {
		return
	}
	if err := room.Forfeit(u.ID); err != nil {
		writeDomainError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// handleListRooms answers the rooms grid; public, no session needed.
func (a *apiServer) handleListRooms(w http.ResponseWriter, _ *http.Request) {
	infos := a.rooms.List()
	out := make([]roomSummary, 0, len(infos))
	for _, info := range infos {
		out = append(out, roomSummaryOf(info))
	}
	writeJSON(w, http.StatusOK, out)
}

// handleCreateRoom opens a room for the session's user under the posted
// settings, with a config-tier bot guest when a bot name is given.
func (a *apiServer) handleCreateRoom(w http.ResponseWriter, r *http.Request, u User) {
	var req struct {
		TCIdx int    `json:"tcIdx"`
		BOLen int    `json:"boLen"`
		Bot   string `json:"bot"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, codeBadRequest, "malformed room body")
		return
	}
	var tier *config.Tier
	if req.Bot != "" {
		tier = tierByName(req.Bot)
		if tier == nil {
			writeError(w, http.StatusBadRequest, codeBadRequest, "unknown bot tier "+req.Bot)
			return
		}
	}
	room, err := a.rooms.Create(u.ID, req.TCIdx, req.BOLen, tier)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	info, ok := room.Info()
	if !ok {
		writeError(w, http.StatusConflict, codeRoomClosed, ErrRoomClosed.Error())
		return
	}
	writeJSON(w, http.StatusCreated, roomSummaryOf(info))
}

// handleRoomDetail answers the public room view of {id}: grid line plus
// the live game snapshot.
func (a *apiServer) handleRoomDetail(w http.ResponseWriter, r *http.Request) {
	room, ok := a.roomFromRequest(w, r)
	if !ok {
		return
	}
	info, live := room.Info()
	if !live {
		writeError(w, http.StatusConflict, codeRoomClosed, ErrRoomClosed.Error())
		return
	}
	writeJSON(w, http.StatusOK, roomDetail{roomSummary: roomSummaryOf(info), Game: room.gameSnapshot()})
}

// handleRoomEvents is the public SSE stream of one room, guests included.
// One frame per hub event: "event: <kind>", the payload as "data:" lines,
// a blank terminator; idle periods emit the keepalive comment frame so
// intermediaries hold the connection. The stream ends cleanly when the
// series finishes (the room retires right after that event), earlier when
// the hub ends the subscription (eviction or hub close), or when the client
// goes away. A spectator of a retired room reconnects and re-syncs from
// GET /api/rooms, which is why an eviction needs no error frame.
func (a *apiServer) handleRoomEvents(w http.ResponseWriter, r *http.Request) {
	room, ok := a.roomFromRequest(w, r)
	if !ok {
		return
	}
	// A room caught mid-retirement (still in the manager map, already over)
	// would otherwise stream keepalives forever: its series event happened
	// before this subscription existed.
	if _, live := room.Info(); !live {
		writeError(w, http.StatusConflict, codeRoomClosed, ErrRoomClosed.Error())
		return
	}
	sub, err := room.Subscribe()
	if err != nil {
		writeDomainError(w, err)
		return
	}
	defer sub.Unsubscribe()
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.WriteHeader(http.StatusOK)
	rc := http.NewResponseController(w)
	_ = rc.Flush()
	ticker := time.NewTicker(a.keepalive)
	defer ticker.Stop()
	for {
		select {
		case ev, open := <-sub.Events():
			if !open {
				return
			}
			if !writeSSEFrame(w, ev) {
				return
			}
			_ = rc.Flush()
			if ev.Kind == EventKindSeries {
				return
			}
		case <-ticker.C:
			if _, err := io.WriteString(w, sseKeepaliveComment); err != nil {
				return
			}
			_ = rc.Flush()
		case <-r.Context().Done():
			return
		}
	}
}

// writeSSEFrame writes one event frame and reports whether the client is
// still reading. Payloads are single-line today; splitting on newlines
// keeps the frame spec-legal if that ever changes.
func writeSSEFrame(w io.Writer, ev Event) bool {
	var b strings.Builder
	b.WriteString("event: ")
	b.WriteString(ev.Kind)
	b.WriteByte('\n')
	for _, line := range strings.Split(ev.Payload, "\n") {
		b.WriteString("data: ")
		b.WriteString(line)
		b.WriteByte('\n')
	}
	b.WriteByte('\n')
	_, err := io.WriteString(w, b.String())
	return err == nil
}

// roomFromRequest resolves {id} against the manager and writes the 404
// envelope itself on a miss.
func (a *apiServer) roomFromRequest(w http.ResponseWriter, r *http.Request) (*Room, bool) {
	room, err := a.rooms.Get(r.PathValue("id"))
	if err != nil {
		writeDomainError(w, err)
		return nil, false
	}
	return room, true
}

// roomSummaryOf maps one manager grid line onto the wire.
func roomSummaryOf(info RoomInfo) roomSummary {
	return roomSummary{
		ID: info.ID, HostUserID: info.HostUserID, GuestUserID: info.GuestUserID,
		TCIdx: info.TCIdx, BOLen: info.BOLen, State: info.State.String(),
		VsBotTier: info.VsBotTier, HostWins: info.HostWins, GuestWins: info.GuestWins,
		CreatedAt: info.CreatedAt.Unix(),
	}
}

// tierByName resolves a wire tier name onto the config tier table.
func tierByName(name string) *config.Tier {
	for i := range config.Tiers {
		if config.Tiers[i].Name == name {
			return &config.Tiers[i]
		}
	}
	return nil
}

// gameSnapshot reads the live game under the room lock. Nil means no game
// is live: the handshake is still pending (between-terminal games never
// happens, the completion path resets in the same critical section).
func (r *Room) gameSnapshot() *gameSnapshot {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.over || r.board == nil || r.series == nil {
		return nil
	}
	turnSeat := r.seatByColorLocked(r.board.Side)
	snap := &gameSnapshot{
		Moves:      make([]string, 0, len(r.moves)),
		Turn:       colorName(r.board.Side),
		TurnUserID: turnSeat.userID, RedUserID: r.series.RedUserID(),
		ClockMs: [2]int64{
			r.clock[rules.Red].Remaining().Milliseconds(),
			r.clock[rules.Blue].Remaining().Milliseconds(),
		},
	}
	for _, m := range r.moves {
		snap.Moves = append(snap.Moves, cellName(rules.Cell(m)))
	}
	return snap
}

// colorName gives the wire names of the two stone colors; rules.Color is a
// bare uint8 with no String method. Only called with a side to move.
func colorName(c rules.Color) string {
	if c == rules.Blue {
		return "blue"
	}
	return "red"
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

// handleHistory answers the session's match history, newest first, with
// the move preview trimmed server-side and the full blob kept for the
// playback board.
func (a *apiServer) handleHistory(w http.ResponseWriter, _ *http.Request, u User) {
	rows, err := a.store.MatchHistory(u.ID)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	out := make([]historyEntry, 0, len(rows))
	for _, row := range rows {
		e, err := historyEntryOf(row)
		if err != nil {
			writeDomainError(w, err)
			return
		}
		out = append(out, e)
	}
	writeJSON(w, http.StatusOK, out)
}

// historyEntry is one match-history line: the grid fields plus the trimmed
// coordinate preview, the ellipsis flag, and the full moves blob.
type historyEntry struct {
	PlayedAt  int64    `json:"playedAt"`
	Red       string   `json:"red"`
	Blue      string   `json:"blue"`
	RedWins   int      `json:"redWins"`
	BlueWins  int      `json:"blueWins"`
	FullTurns int      `json:"fullTurns"`
	WonBy     *string  `json:"wonBy"`
	Preview   []string `json:"preview"`
	Truncated bool     `json:"truncated"`
	Moves     []byte   `json:"moves"`
}

// historyEntryOf renders one row: the first config.HistoryPreviewTurns full
// turns of coordinate names (a full turn is red's stone plus blue's), the
// truncation flag when more followed, and the untouched blob.
func historyEntryOf(row MatchHistoryRow) (historyEntry, error) {
	moves, err := decodeMoves(row.Moves)
	if err != nil {
		return historyEntry{}, err
	}
	limit := min(2*config.HistoryPreviewTurns, len(moves))
	preview := make([]string, 0, limit)
	for _, m := range moves[:limit] {
		preview = append(preview, cellName(rules.Cell(m)))
	}
	return historyEntry{
		PlayedAt: row.PlayedAt, Red: row.Red, Blue: row.Blue,
		RedWins: row.RedWins, BlueWins: row.BlueWins,
		FullTurns: row.FullTurns, WonBy: row.WonBy,
		Preview: preview, Truncated: len(moves) > limit,
		Moves: row.Moves,
	}, nil
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
