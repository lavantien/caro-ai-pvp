package server

// The M6b htmx room page and playback board: server-rendered boards over the
// landed room detail, live updates through the spike's proven SSE wiring
// (hx-sse with hx-on handlers per event kind, a plain EventSource as the
// fallback once htmx gives up reconnecting), and the playback route reading
// one finished game straight from the store. Page routes mount beside the
// JSON API; nothing here touches the API mux.

import (
	"bytes"
	"database/sql"
	"embed"
	"errors"
	"fmt"
	"html/template"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/lavantien/caro-ai-pvp/internal/config"
	"github.com/lavantien/caro-ai-pvp/internal/rules"
)

//go:embed web/templates
var webTemplates embed.FS

// uiTmpl parses the page set once; every template in the directory joins one
// set so the shared board partials stay single-source.
var uiTmpl = template.Must(template.ParseFS(webTemplates, "web/templates/*.html"))

// Page route patterns the serve composition mounts.
const (
	roomRoutePattern     = "GET /rooms/{id}"
	playbackRoutePattern = "GET /rooms/history/{gameID}"
)

// RoomPages serves the room and playback pages over the same room manager
// and store the JSON API reads.
type RoomPages struct {
	rooms *RoomManager
	store *Store
}

// NewRoomPages builds the page handlers for the lead to mount.
func NewRoomPages(rooms *RoomManager, store *Store) *RoomPages {
	return &RoomPages{rooms: rooms, store: store}
}

// Mount registers both page routes on mux.
func (p *RoomPages) Mount(mux *http.ServeMux) {
	mux.HandleFunc(roomRoutePattern, p.HandleRoom)
	mux.HandleFunc(playbackRoutePattern, p.HandlePlayback)
}

// HandleRoom renders the live game view of GET /rooms/{id}: the full board
// rehydrated server-side from the room detail (a mid-stream reload never
// misses stones), both clocks, the ready handshake and the acting controls
// for participants only, and the SSE feed attachments. Guests and strangers
// get the same page read-only.
func (p *RoomPages) HandleRoom(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	room, err := p.resolveRoom(id)
	if err != nil {
		writePageNotFound(w)
		return
	}
	view, err := p.roomViewOf(r, room)
	if err != nil {
		// The room may retire between the liveness resolve above and the
		// view's own read; that race is the 404 page, never a 500.
		if errors.Is(err, ErrRoomNotFound) {
			writePageNotFound(w)
			return
		}
		http.Error(w, "room page read failed", http.StatusInternalServerError)
		return
	}
	p.renderRoom(w, view)
}

// HandlePlayback renders GET /rooms/history/{gameID}: one finished game read
// straight from the games table (the same rows /api/history lists, without a
// refetch round trip), decoded through the shared moves codec, on the same
// board component with step controls. Playback follows the history listing's
// visibility: only a session that played the game may open it.
func (p *RoomPages) HandlePlayback(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("gameID"), 10, 64)
	if err != nil || id <= 0 {
		writePageNotFound(w)
		return
	}
	viewer, ok, err := p.viewerOf(r)
	if err != nil {
		http.Error(w, "playback read failed", http.StatusInternalServerError)
		return
	}
	if !ok {
		writePageNotFound(w)
		return
	}
	g, err := p.store.gameByID(id)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			writePageNotFound(w)
			return
		}
		http.Error(w, "playback read failed", http.StatusInternalServerError)
		return
	}
	if viewer.ID != g.RedUser && viewer.ID != g.BlueUser {
		writePageNotFound(w)
		return
	}
	names, err := playbackMoves(g.Moves)
	if err != nil {
		http.Error(w, "playback decode failed", http.StatusInternalServerError)
		return
	}
	redName, err := p.userName(g.RedUser)
	if err != nil {
		http.Error(w, "playback read failed", http.StatusInternalServerError)
		return
	}
	blueName, err := p.userName(g.BlueUser)
	if err != nil {
		http.Error(w, "playback read failed", http.StatusInternalServerError)
		return
	}
	p.renderPlayback(w, playbackView{
		boardData: boardData{Cells: boardCells(names), BoardSize: config.BoardSize},
		GameID:    g.ID, RedName: redName, BlueName: blueName,
		Outcome: outcomeWord(g.Outcome), WonBy: derefString(g.WonBy), Moves: names,
	})
}

// resolveRoom maps the path id onto a live room; anything but a live room id
// is the 404 page.
func (p *RoomPages) resolveRoom(id string) (*Room, error) {
	if !isRoomID(id) {
		return nil, ErrRoomNotFound
	}
	room, err := p.rooms.Get(id)
	if err != nil {
		return nil, err
	}
	if _, live := room.Info(); !live {
		return nil, ErrRoomNotFound
	}
	return room, nil
}

// cellView is one board square: the rules coordinate name and the stone it
// carries, with the move index and the latest flag for rendering.
type cellView struct {
	Name   string
	Stone  string // "", "red", "blue"
	Glyph  string // "", "O", "X"
	Idx    int
	Latest bool
}

// moveLine is one move-history entry.
type moveLine struct {
	Name   string
	Latest bool
}

// boardData is the shared board component state both pages render.
type boardData struct {
	Cells     []cellView
	Live      bool   // the room board accepts the mover's input
	MyColor   string // "", "red", "blue": the viewer's stones when playing
	BoardSize int
}

// roomView is the room page's whole render state.
type roomView struct {
	boardData
	RoomID        string
	State         string
	TCLabel       string
	BOLen         int
	VsBotTier     string
	HostUserID    int64
	HostName      string
	GuestUserID   int64
	GuestName     string
	HostWins      int
	GuestWins     int
	HostReady     string
	GuestReady    string
	CanReady      bool
	CanForfeit    bool
	CanJoin       bool // a logged-in stranger may take the open guest seat
	ViewerID      int64
	ViewerName    string
	IsParticipant bool
	GameLive      bool
	Turn          string
	TurnName      string
	TurnIsMe      bool
	RedName       string
	BlueName      string
	ClockRed      string
	ClockBlue     string
	Moves         []moveLine
	KindsCSV      string
}

// playbackView is the playback page's whole render state.
type playbackView struct {
	boardData
	GameID   int64
	RedName  string
	BlueName string
	Outcome  string
	WonBy    string
	Moves    []string
}

// roomViewOf snapshots one live room into the page state: grid line,
// handshake, live game, seat names, and the viewer's role. The viewer is the
// session's user when a live session cookie rides the request, the guest
// view otherwise; a dead session is a guest, never an error page.
func (p *RoomPages) roomViewOf(r *http.Request, room *Room) (roomView, error) {
	info, live := room.Info()
	if !live {
		return roomView{}, ErrRoomNotFound
	}
	view := roomView{
		RoomID: info.ID, State: info.State.String(), TCLabel: tcLabel(info.TCIdx),
		BOLen: info.BOLen, VsBotTier: info.VsBotTier,
		HostUserID: info.HostUserID, GuestUserID: info.GuestUserID,
		HostWins: info.HostWins, GuestWins: info.GuestWins,
		CanForfeit: true, BoardSize: config.BoardSize,
		KindsCSV: strings.Join(spikeEventKinds, ","),
	}
	// A bot host has no user row: the tier is the name, and a store lookup
	// of the synthetic id would read as a missing account and fail the
	// page. Bot guests render the same way through guestName.
	if info.HostBotTier != "" {
		view.HostName = "AI " + info.HostBotTier
	} else {
		hostName, err := p.userName(info.HostUserID)
		if err != nil {
			return roomView{}, err
		}
		view.HostName = hostName
	}
	view.GuestName = p.guestName(info)

	viewer, ok, err := p.viewerOf(r)
	if err != nil {
		return view, err
	}
	view.IsParticipant = ok && (viewer.ID == info.HostUserID || viewer.ID == info.GuestUserID)
	if !view.IsParticipant {
		view.CanForfeit = false
		// The seat-taking affordance: a logged-in stranger on a created
		// human room whose guest seat is still open.
		view.CanJoin = ok && info.State == SeriesCreated &&
			info.VsBotTier == "" && info.GuestUserID == 0
	} else {
		view.ViewerID, view.ViewerName = viewer.ID, viewer.Username
	}

	// The handshake section rides the created state only: once both ready,
	// game 1 is live and the ready button would only answer 409.
	seriesOK, hostReady, guestReady := room.readyFlags()
	if view.IsParticipant && seriesOK && info.State == SeriesCreated {
		view.CanReady = true
		view.HostReady, view.GuestReady = readyWord(hostReady), readyWord(guestReady)
	}

	if snap := room.gameSnapshot(); snap != nil {
		view.GameLive = true
		view.Turn, view.TurnName = snap.Turn, view.seatNameOf(snap.TurnUserID)
		blueUserID := info.HostUserID
		if snap.RedUserID == info.HostUserID {
			blueUserID = info.GuestUserID
		}
		view.RedName, view.BlueName = view.seatNameOf(snap.RedUserID), view.seatNameOf(blueUserID)
		view.ClockRed, view.ClockBlue = formatClockMs(snap.ClockMs[0]), formatClockMs(snap.ClockMs[1])
		view.Moves = moveLines(snap.Moves)
		view.Cells = boardCells(snap.Moves)
		if view.IsParticipant {
			view.MyColor = "blue"
			if viewer.ID == snap.RedUserID {
				view.MyColor = "red"
			}
			view.TurnIsMe = snap.TurnUserID == viewer.ID
			view.Live = view.TurnIsMe
		}
	} else {
		// The grid renders pre-match too: a spectator arriving before the
		// handshake sees the full empty board, not a blank square.
		view.Cells = boardCells(nil)
	}
	return view, nil
}

// viewerOf resolves the session's user. A missing cookie and a dead session
// are the guest view; a live-shaped session over a failing store is an
// outage the caller must surface, not a silent guest render.
func (p *RoomPages) viewerOf(r *http.Request) (User, bool, error) {
	token, err := sessionToken(r)
	if err != nil {
		return User{}, false, nil
	}
	u, err := Authenticate(p.store, token, time.Now().Unix())
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return User{}, false, nil
		}
		return User{}, false, err
	}
	return u, true, nil
}

// userName resolves one user id to its display name.
func (p *RoomPages) userName(id int64) (string, error) {
	u, err := p.store.UserByID(id)
	if err != nil {
		return "", err
	}
	return u.Username, nil
}

// guestName renders the opponent seat: the bot tier, the waiting state, or
// the seated user's name.
func (p *RoomPages) guestName(info RoomInfo) string {
	switch {
	case info.VsBotTier != "":
		return "AI " + info.VsBotTier
	case info.GuestUserID == 0:
		return "waiting for opponent"
	}
	name, err := p.userName(info.GuestUserID)
	if err != nil {
		return "opponent"
	}
	return name
}

// seatNameOf maps one seat id of this room onto its display name without
// another store read: host, guest, or the synthetic bot seat.
func (v roomView) seatNameOf(id int64) string {
	switch id {
	case v.HostUserID:
		return v.HostName
	case v.GuestUserID:
		return v.GuestName
	}
	return "opponent"
}

// readyFlags snapshots the handshake under the room lock (the series pointer
// itself swaps at join), the series lock nested inside per the lock order.
func (r *Room) readyFlags() (exists, hostReady, guestReady bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.series == nil {
		return false, false, false
	}
	return true, r.series.HostReady(), r.series.GuestReady()
}

// boardCells lays out every coordinate of the config-sized board in row
// order, stones placed by move parity: plies 0, 2, 4... are red, the first
// mover holding red per the rules.
func boardCells(moves []string) []cellView {
	at := make(map[string]int, len(moves))
	for i, name := range moves {
		at[name] = i
	}
	cells := make([]cellView, 0, config.BoardCells)
	for cell := range config.BoardCells {
		name := cellName(rules.Cell(cell))
		cv := cellView{Name: name}
		if i, ok := at[name]; ok {
			cv.Idx = i
			cv.Stone, cv.Glyph = stoneOf(i)
			cv.Latest = i == len(moves)-1
		}
		cells = append(cells, cv)
	}
	return cells
}

func stoneOf(i int) (stone, glyph string) {
	if i%2 == 0 {
		return "red", "O"
	}
	return "blue", "X"
}

func moveLines(names []string) []moveLine {
	out := make([]moveLine, len(names))
	for i, name := range names {
		out[i] = moveLine{Name: name, Latest: i == len(names)-1}
	}
	return out
}

// gameByID reads one finished game row, the playback board's whole data
// path; a missing id maps to ErrNotFound like every single-row accessor.
func (s *Store) gameByID(id int64) (Game, error) {
	var g Game
	var wonBy sql.NullString
	err := notFound(s.db.QueryRow(
		`SELECT id, series_id, idx_in_series, red_user, blue_user, outcome, moves, full_turns, won_by, played_at
		FROM games WHERE id = ?`, id,
	).Scan(&g.ID, &g.SeriesID, &g.IdxInSeries, &g.RedUser, &g.BlueUser, &g.Outcome,
		&g.Moves, &g.FullTurns, &wonBy, &g.PlayedAt))
	if err != nil {
		return Game{}, fmt.Errorf("server: fetch game %d: %w", id, err)
	}
	g.WonBy = nullString(wonBy)
	return g, nil
}

// playbackMoves decodes a history blob into ordered coordinate names through
// the shared codec.
func playbackMoves(blob []byte) ([]string, error) {
	moves, err := DecodeMoves(blob)
	if err != nil {
		return nil, err
	}
	names := make([]string, len(moves))
	for i, m := range moves {
		names[i] = cellName(rules.Cell(m))
	}
	return names, nil
}

// tapSelect is the tap-to-preview state machine of the coarse-pointer
// surface (Scenario 1's mobile clause): the first tap on a cell selects it,
// a second tap on the same cell confirms the move, a tap on another cell
// moves the selection. room.js ships the browser twin of these semantics,
// pinned byte-exact by test against the block below.
func tapSelect(selected, tapped string) (next string, confirm bool) {
	if selected == tapped {
		return "", true
	}
	return tapped, false
}

// The tapSelect twin markers inside web/static/room.js.
const (
	tapSelectJSBegin = "/* caro-tap-select:begin */"
	tapSelectJSEnd   = "/* caro-tap-select:end */"
)

// tapSelectJS is the canonical browser twin; room.js carries it verbatim
// between the markers so the two cannot drift silently.
const tapSelectJS = tapSelectJSBegin + `
function tapSelect(sel, cell) {
	if (sel === cell) { return { select: '', confirm: true }; }
	return { select: cell, confirm: false };
}
` + tapSelectJSEnd

// uiMLine hides the Implication 1.5 fields the spec bars from the UI ("on
// UI, ebf, hf, fh1 are hidden"): the wire line stays verbatim for the
// analytics pipeline, the room page's bot log drops those three tokens.
func uiMLine(line string) string {
	fields := strings.Split(line, ", ")
	kept := fields[:0]
	for _, f := range fields {
		if strings.HasPrefix(f, "ebf=") || strings.HasPrefix(f, "hf=") || strings.HasPrefix(f, "fh1=") {
			continue
		}
		kept = append(kept, f)
	}
	return strings.Join(kept, ", ")
}

// The uiMLine twin markers inside web/static/room.js.
const (
	uiMLineJSBegin = "/* caro-ui-mline:begin */"
	uiMLineJSEnd   = "/* caro-ui-mline:end */"
)

// uiMLineJS is uiMLine's browser twin, pinned like tapSelect's.
const uiMLineJS = uiMLineJSBegin + `
function uiMLine(line) {
	var fields = line.split(', ');
	var kept = [];
	for (var i = 0; i < fields.length; i++) {
		var f = fields[i];
		if (f.indexOf('ebf=') === 0 || f.indexOf('hf=') === 0 || f.indexOf('fh1=') === 0) { continue; }
		kept.push(f);
	}
	return kept.join(', ');
}
` + uiMLineJSEnd

// formatClockMs renders one clock bank as mm:ss.t; a negative reading
// (impossible under the clock law) clamps to zero.
func formatClockMs(ms int64) string {
	if ms < 0 {
		ms = 0
	}
	secs := ms / 1000
	return fmt.Sprintf("%02d:%02d.%d", secs/60, secs%60, (ms%1000)/100)
}

// tcLabel renders one config time control in the conventional m+i notation.
func tcLabel(idx int) string {
	return fmt.Sprintf("%d+%d", config.TimeControls[idx].InitialSec, config.TimeControls[idx].IncrementSec)
}

func readyWord(ready bool) string {
	if ready {
		return "ready"
	}
	return "not ready"
}

// outcomeWord renders a stored outcome for display; the schema's outcome
// strings are the same wire words the gameend events carry.
func outcomeWord(outcome string) string {
	switch outcome {
	case OutcomeRed:
		return "red wins"
	case OutcomeBlue:
		return "blue wins"
	case OutcomeDraw:
		return "draw"
	}
	return outcome
}

func derefString(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}

func executeRoom(w io.Writer, v roomView) error { return uiTmpl.ExecuteTemplate(w, "room.html", v) }
func executePlayback(w io.Writer, v playbackView) error {
	return uiTmpl.ExecuteTemplate(w, "playback.html", v)
}

func (p *RoomPages) renderRoom(w http.ResponseWriter, v roomView) {
	var buf bytes.Buffer
	if err := executeRoom(&buf, v); err != nil {
		http.Error(w, "room page render failed", http.StatusInternalServerError)
		return
	}
	writeHTML(w, http.StatusOK, buf.Bytes())
}

func (p *RoomPages) renderPlayback(w http.ResponseWriter, v playbackView) {
	var buf bytes.Buffer
	if err := executePlayback(&buf, v); err != nil {
		http.Error(w, "playback render failed", http.StatusInternalServerError)
		return
	}
	writeHTML(w, http.StatusOK, buf.Bytes())
}

// writeHTML ships one rendered page; live views never cache.
func writeHTML(w http.ResponseWriter, status int, body []byte) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_, _ = w.Write(body)
}

// writePageNotFound answers every unusable page route identically: a bad id,
// an unknown room, a retired room, a playback row the viewer never played.
const pageNotFoundHTML = `<!doctype html>
<html lang="en"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width, initial-scale=1">
<title>caro: not found</title></head>
<body><h1>not found</h1><p>no such live room or playable game.<br><a href="/">home</a></p></body></html>
`

func writePageNotFound(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(http.StatusNotFound)
	_, _ = io.WriteString(w, pageNotFoundHTML)
}
