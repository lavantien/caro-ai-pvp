package server

import (
	"bytes"
	"database/sql"
	"embed"
	"errors"
	"fmt"
	"html/template"
	"io"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/lavantien/caro-ai-pvp/internal/config"
	"github.com/lavantien/caro-ai-pvp/internal/rules"
)

//go:embed web/templates
var webTemplates embed.FS

var uiTmpl = template.Must(template.ParseFS(webTemplates, "web/templates/*.html"))

const (
	roomRoutePattern     = "GET /rooms/{id}"
	playbackRoutePattern = "GET /rooms/history/{gameID}"
)

type RoomPages struct {
	rooms *RoomManager
	store *Store
}

func NewRoomPages(rooms *RoomManager, store *Store) *RoomPages {
	return &RoomPages{rooms: rooms, store: store}
}

func (p *RoomPages) Mount(mux *http.ServeMux) {
	mux.HandleFunc(roomRoutePattern, p.HandleRoom)
	mux.HandleFunc(playbackRoutePattern, p.HandlePlayback)
}

func (p *RoomPages) HandleRoom(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	room, err := p.resolveRoom(id)
	if err != nil {
		writePageNotFound(w)
		return
	}
	view, err := p.roomViewOf(r, room)
	if err != nil {
		if errors.Is(err, ErrRoomNotFound) {
			writePageNotFound(w)
			return
		}
		http.Error(w, "room page read failed", http.StatusInternalServerError)
		log.Printf("server: room %s page read: %v", id, err)
		return
	}
	p.renderRoom(w, view)
}

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
	red, err := p.store.UserByID(g.RedUser)
	if err != nil {
		http.Error(w, "playback read failed", http.StatusInternalServerError)
		return
	}
	blue, err := p.store.UserByID(g.BlueUser)
	if err != nil {
		http.Error(w, "playback read failed", http.StatusInternalServerError)
		return
	}
	redName, blueName := red.Username, blue.Username
	if g.BotName != "" {
		if isBotAccount(red) {
			redName = g.BotName
		} else if isBotAccount(blue) {
			blueName = g.BotName
		}
	}
	p.renderPlayback(w, playbackView{
		boardData: boardData{Cells: boardCells(names, nil), BoardSize: config.BoardSize},
		GameID:    g.ID, RedName: redName, BlueName: blueName,
		Outcome: outcomeWord(g.Outcome), WonBy: derefString(g.WonBy), Moves: names,
		Pairs: movePairs(names),
	})
}

func isBotAccount(u User) bool {
	return len(u.Salt) == 0 && len(u.Hash) == 0
}

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

type cellView struct {
	Name   string
	Stone  string
	Glyph  string
	Idx    int
	Latest bool
	Off    bool
}

type boardData struct {
	Cells     []cellView
	Live      bool
	MyColor   string
	BoardSize int
	Mini      bool
}

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
	CanJoin       bool
	ViewerID      int64
	ViewerName    string
	IsParticipant bool
	GameLive      bool
	Turn          string
	TurnName      string
	TurnIsMe      bool
	RedName       string
	BlueName      string
	RedTier       string
	BlueTier      string
	ClockRed      string
	ClockBlue     string
	Moves         []string
	Pairs         []MovePair
	KindsCSV      string
}

type playbackView struct {
	boardData
	GameID   int64
	RedName  string
	BlueName string
	Outcome  string
	WonBy    string
	Moves    []string
	Pairs    []MovePair
}

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
	if info.HostBotName != "" {
		view.HostName = info.HostBotName
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
		view.CanJoin = ok && info.State == SeriesCreated &&
			info.VsBotTier == "" && info.GuestUserID == 0
	} else {
		view.ViewerID, view.ViewerName = viewer.ID, viewer.Username
	}

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
		if info.VsBotTier != "" {
			if (snap.RedUserID == info.HostUserID) == (info.HostBotName != "") {
				view.RedTier = info.VsBotTier
			} else {
				view.BlueTier = info.VsBotTier
			}
		}
		view.ClockRed, view.ClockBlue = formatClockMs(snap.ClockMs[0]), formatClockMs(snap.ClockMs[1])
		view.Moves = snap.Moves
		view.Pairs = movePairs(snap.Moves)
		var playable map[string]bool
		if view.IsParticipant {
			view.MyColor = "blue"
			if viewer.ID == snap.RedUserID {
				view.MyColor = "red"
			}
			view.TurnIsMe = snap.TurnUserID == viewer.ID
			view.Live = view.TurnIsMe
			if view.Live {
				playable = make(map[string]bool, len(snap.Playable))
				for _, name := range snap.Playable {
					playable[name] = true
				}
			}
		}
		view.Cells = boardCells(snap.Moves, playable)
	} else {
		view.RedName, view.BlueName = view.HostName, view.GuestName
		view.Cells = boardCells(nil, nil)
	}
	return view, nil
}

func (p *RoomPages) viewerOf(r *http.Request) (User, bool, error) {
	token, err := sessionToken(r)
	if err != nil {
		return User{}, false, nil
	}
	u, err := Authenticate(p.store, token, time.Now().Unix())
	if err != nil {
		if errors.Is(err, ErrNotFound) || errors.Is(err, ErrBadCredentials) {
			return User{}, false, nil
		}
		return User{}, false, err
	}
	return u, true, nil
}

func (p *RoomPages) userName(id int64) (string, error) {
	u, err := p.store.UserByID(id)
	if err != nil {
		return "", err
	}
	return u.Username, nil
}

func (p *RoomPages) guestName(info RoomInfo) string {
	switch {
	case info.GuestBotName != "":
		return info.GuestBotName
	case info.GuestUserID == 0:
		return "waiting for opponent"
	}
	name, err := p.userName(info.GuestUserID)
	if err != nil {
		return "opponent"
	}
	return name
}

func (v roomView) seatNameOf(id int64) string {
	switch id {
	case v.HostUserID:
		return v.HostName
	case v.GuestUserID:
		return v.GuestName
	}
	return "opponent"
}

func (r *Room) readyFlags() (exists, hostReady, guestReady bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.series == nil {
		return false, false, false
	}
	return true, r.series.HostReady(), r.series.GuestReady()
}

func boardCells(moves []string, playable map[string]bool) []cellView {
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
		} else if playable != nil && !playable[name] {
			cv.Off = true
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

func (s *Store) gameByID(id int64) (Game, error) {
	var g Game
	var wonBy, botName sql.NullString
	err := notFound(s.db.QueryRow(
		`SELECT id, series_id, idx_in_series, red_user, blue_user, outcome, moves, full_turns, won_by, bot_name, played_at
		FROM games WHERE id = ?`, id,
	).Scan(&g.ID, &g.SeriesID, &g.IdxInSeries, &g.RedUser, &g.BlueUser, &g.Outcome,
		&g.Moves, &g.FullTurns, &wonBy, &botName, &g.PlayedAt))
	if err != nil {
		return Game{}, fmt.Errorf("server: fetch game %d: %w", id, err)
	}
	g.WonBy = nullString(wonBy)
	if botName.Valid {
		g.BotName = botName.String
	}
	return g, nil
}

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

func tapSelect(selected, tapped string) (next string, confirm bool) {
	if selected == tapped {
		return "", true
	}
	return tapped, false
}

const (
	tapSelectJSBegin = "/* caro-tap-select:begin */"
	tapSelectJSEnd   = "/* caro-tap-select:end */"
)

const tapSelectJS = tapSelectJSBegin + `
function tapSelect(sel, cell) {
	if (sel === cell) { return { select: '', confirm: true }; }
	return { select: cell, confirm: false };
}
` + tapSelectJSEnd

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

const (
	uiMLineJSBegin = "/* caro-ui-mline:begin */"
	uiMLineJSEnd   = "/* caro-ui-mline:end */"
)

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

func formatClockMs(ms int64) string {
	if ms < 0 {
		ms = 0
	}
	secs := ms / 1000
	return fmt.Sprintf("%02d:%02d.%d", secs/60, secs%60, (ms%1000)/100)
}

func tcLabel(idx int) string {
	return fmt.Sprintf("%d+%d", config.TimeControls[idx].InitialMin, config.TimeControls[idx].IncrementSec)
}

func readyWord(ready bool) string {
	if ready {
		return "ready"
	}
	return "not ready"
}

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

func writeHTML(w http.ResponseWriter, status int, body []byte) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_, _ = w.Write(body)
}

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
