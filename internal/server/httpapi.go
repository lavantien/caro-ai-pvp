package server

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

const (
	sessionCookieName  = "caro_session"
	httpBodyLimitBytes = 64 << 10
)

const (
	codeBadCredentials  = "bad_credentials"
	codeInvalidUsername = "invalid_username"
	codeUnauthorized    = "unauthorized"
	codeBadRequest      = "bad_request"
	codeNotParticipant  = "not_participant"
	codeRoomNotFound    = "room_not_found"
	codeRoomFull        = "room_full"
	codeRoomClosed      = "room_closed"
	codeMachineBusy     = "machine_busy"
	codeNotReady        = "not_ready"
	codeNotYourTurn     = "not_your_turn"
	codeIllegalMove     = "illegal_move"
	codeSeriesFinished  = "series_finished"
	codeInternal        = "internal"
)

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
	case errors.Is(err, ErrMachineBusy):
		return codeMachineBusy, http.StatusConflict
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
		errors.Is(err, ErrBadOwner),
		errors.Is(err, ErrBadTier):
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

func decodeJSON(r *http.Request, dst any) error {
	return json.NewDecoder(io.LimitReader(r.Body, httpBodyLimitBytes)).Decode(dst)
}

type userSummary struct {
	Username string `json:"username"`
	Rating   int    `json:"rating"`
	Wins     int    `json:"wins"`
	Losses   int    `json:"losses"`
	Draws    int    `json:"draws"`
	Level    int    `json:"level"`
}

type roomSummary struct {
	ID          string `json:"id"`
	HostUserID  int64  `json:"hostUserId"`
	GuestUserID int64  `json:"guestUserId"`
	TCIdx       int    `json:"tcIdx"`
	BOLen       int    `json:"boLen"`
	State       string `json:"state"`
	HostBotTier string `json:"hostBotTier,omitempty"`
	VsBotTier   string `json:"vsBotTier,omitempty"`
	HostWins    int    `json:"hostWins"`
	GuestWins   int    `json:"guestWins"`
	CreatedAt   int64  `json:"createdAt"`
}

type gameSnapshot struct {
	Moves      []string `json:"moves"`
	Turn       string   `json:"turn"`
	TurnUserID int64    `json:"turnUserId"`
	RedUserID  int64    `json:"redUserId"`
	ClockMs    [2]int64 `json:"clockMs"`
	Playable   []string `json:"playable"`
}

type roomDetail struct {
	roomSummary
	Game *gameSnapshot `json:"game"`
}

type apiServer struct {
	store     *Store
	rooms     *RoomManager
	keepalive time.Duration
}

const sseKeepaliveComment = ": keepalive\n\n"

func NewHTTPAPI(store *Store, rooms *RoomManager) http.Handler {
	a := &apiServer{store: store, rooms: rooms,
		keepalive: time.Duration(config.SSEKeepaliveMs) * time.Millisecond}
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
	mux.HandleFunc("GET /static/", a.handleStatic)
	mux.HandleFunc("GET /spike", a.handleSpike)
	return mux
}

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

func (a *apiServer) handleListRooms(w http.ResponseWriter, _ *http.Request) {
	infos := a.rooms.List()
	out := make([]roomSummary, 0, len(infos))
	for _, info := range infos {
		out = append(out, roomSummaryOf(info))
	}
	writeJSON(w, http.StatusOK, out)
}

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

func (a *apiServer) handleRoomEvents(w http.ResponseWriter, r *http.Request) {
	room, ok := a.roomFromRequest(w, r)
	if !ok {
		return
	}
	sub, err := room.Subscribe()
	if err != nil {
		writeDomainError(w, err)
		return
	}
	defer sub.Unsubscribe()
	if _, live := room.Info(); !live {
		writeError(w, http.StatusConflict, codeRoomClosed, ErrRoomClosed.Error())
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.WriteHeader(http.StatusOK)
	rc := http.NewResponseController(w)
	_ = rc.Flush()
	ticker := time.NewTicker(a.keepalive)
	defer ticker.Stop()
	deliver := func(ev Event, open bool) bool {
		if !open || !writeSSEFrame(w, ev) {
			return true
		}
		_ = rc.Flush()
		return ev.Kind == EventKindSeries
	}
	for {
		select {
		case ev, open := <-sub.Events():
			if deliver(ev, open) {
				return
			}
		case <-ticker.C:
			if _, live := room.Info(); live {
				if _, err := io.WriteString(w, sseKeepaliveComment); err != nil {
					return
				}
				_ = rc.Flush()
				continue
			}
			for {
				select {
				case ev, open := <-sub.Events():
					if deliver(ev, open) {
						return
					}
				default:
					return
				}
			}
		case <-r.Context().Done():
			return
		}
	}
}

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

func (a *apiServer) roomFromRequest(w http.ResponseWriter, r *http.Request) (*Room, bool) {
	room, err := a.rooms.Get(r.PathValue("id"))
	if err != nil {
		writeDomainError(w, err)
		return nil, false
	}
	return room, true
}

func roomSummaryOf(info RoomInfo) roomSummary {
	return roomSummary{
		ID: info.ID, HostUserID: info.HostUserID, GuestUserID: info.GuestUserID,
		TCIdx: info.TCIdx, BOLen: info.BOLen, State: info.State.String(),
		HostBotTier: info.HostBotTier, VsBotTier: info.VsBotTier,
		HostWins: info.HostWins, GuestWins: info.GuestWins,
		CreatedAt: info.CreatedAt.Unix(),
	}
}

func tierByName(name string) *config.Tier {
	for i := range config.Tiers {
		if config.Tiers[i].Name == name {
			return &config.Tiers[i]
		}
	}
	return nil
}

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
	var legal [config.MaxBoardCells]rules.Move
	n := r.board.LegalMoves(legal[:])
	snap.Playable = make([]string, 0, n)
	for _, m := range legal[:n] {
		snap.Playable = append(snap.Playable, cellName(rules.Cell(m)))
	}
	return snap
}

func colorName(c rules.Color) string {
	if c == rules.Blue {
		return "blue"
	}
	return "red"
}

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
	sum, err := a.userSummaryOf(u)
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
	writeJSON(w, http.StatusOK, sum)
}

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

func (a *apiServer) handleMe(w http.ResponseWriter, _ *http.Request, u User) {
	a.writeUserSummary(w, http.StatusOK, u)
}

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
			continue
		}
		out = append(out, e)
	}
	writeJSON(w, http.StatusOK, out)
}

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

func historyEntryOf(row MatchHistoryRow) (historyEntry, error) {
	moves, err := DecodeMoves(row.Moves)
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

func (a *apiServer) userSummaryOf(u User) (userSummary, error) {
	rating, err := currentRating(a.store, u.ID)
	if err != nil {
		return userSummary{}, err
	}
	stats, err := a.store.UserStats(u.ID)
	if err != nil {
		return userSummary{}, err
	}
	return userSummary{
		Username: u.Username, Rating: rating,
		Wins: stats.Wins, Losses: stats.Losses, Draws: stats.Draws, Level: stats.SeriesWon,
	}, nil
}

func (a *apiServer) writeUserSummary(w http.ResponseWriter, status int, u User) {
	sum, err := a.userSummaryOf(u)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	writeJSON(w, status, sum)
}

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

func sessionToken(r *http.Request) ([]byte, error) {
	c, err := r.Cookie(sessionCookieName)
	if err != nil {
		return nil, err
	}
	return hex.DecodeString(c.Value)
}
