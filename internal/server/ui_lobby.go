package server

import (
	"html/template"
	"net/http"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/lavantien/caro-ai-pvp/internal/config"
)

const (
	lobbyModeCasual   = "casual"
	lobbyModeVsBot    = "vs bot"
	lobbyModeBotVsBot = "bot vs bot"
	openSeatLabel     = "open seat"
)

type LobbySeat struct {
	Name string
	Tier string
}

type LobbyRoom struct {
	ID         string
	Mode       string
	Host       LobbySeat
	Guest      LobbySeat
	HostUserID int64
	TCIdx      int
	BOLen      int
	Spectators int
	Status     string
	Joinable   bool
	CreatedAt  time.Time
	Live       bool
	Moves      []string
	MoveCount  int
	RedIsHost  bool
}

func (h *Hub) watchers(room string) int {
	h.mu.Lock()
	defer h.mu.Unlock()
	return len(h.rooms[room])
}

func (rm *RoomManager) LobbySnapshot() []LobbyRoom {
	rm.mu.Lock()
	rooms := make([]*Room, 0, len(rm.rooms))
	for _, r := range rm.rooms {
		rooms = append(rooms, r)
	}
	hub, store := rm.hub, rm.store
	rm.mu.Unlock()
	out := make([]LobbyRoom, 0, len(rooms))
	for _, r := range rooms {
		lr, ok := lobbyRoomOf(r, hub, store)
		if !ok {
			continue
		}
		out = append(out, lr)
	}
	slices.SortFunc(out, func(a, b LobbyRoom) int {
		if c := a.CreatedAt.Compare(b.CreatedAt); c != 0 {
			return c
		}
		return strings.Compare(a.ID, b.ID)
	})
	return out
}

func lobbyRoomOf(r *Room, hub *Hub, store *Store) (LobbyRoom, bool) {
	info, ok := r.Info()
	if !ok {
		return LobbyRoom{}, false
	}
	lr := LobbyRoom{
		ID:         info.ID,
		Mode:       lobbyMode(info.HostBotTier, info.VsBotTier),
		Host:       lobbySeatOf(info.HostBotName, info.HostBotTier, info.HostUserID, store),
		Guest:      lobbySeatOf(info.GuestBotName, info.VsBotTier, info.GuestUserID, store),
		HostUserID: info.HostUserID,
		TCIdx:      info.TCIdx,
		BOLen:      info.BOLen,
		Spectators: hub.watchers(info.ID),
		Status:     info.State.String(),
		Joinable:   info.HostBotTier == "" && info.VsBotTier == "" && info.GuestUserID == 0 && info.State == SeriesCreated,
		CreatedAt:  info.CreatedAt,
	}
	if snap := r.gameSnapshot(); snap != nil {
		lr.Live = true
		lr.Moves = snap.Moves
		lr.MoveCount = len(snap.Moves)
		lr.RedIsHost = snap.RedUserID == info.HostUserID
	}
	return lr, true
}

func lobbySeatOf(botName, tier string, userID int64, store *Store) LobbySeat {
	if tier != "" {
		return LobbySeat{Name: botName, Tier: tier}
	}
	if userID == 0 {
		return LobbySeat{Name: openSeatLabel}
	}
	u, err := store.UserByID(userID)
	if err != nil {
		return LobbySeat{Name: "#" + strconv.FormatInt(userID, 10)}
	}
	return LobbySeat{Name: u.Username}
}

func lobbyMode(hostTier, guestTier string) string {
	switch {
	case hostTier != "" && guestTier != "":
		return lobbyModeBotVsBot
	case guestTier != "":
		return lobbyModeVsBot
	default:
		return lobbyModeCasual
	}
}

type LobbyPages struct {
	store *Store
	rooms *RoomManager
}

func NewLobbyPages(store *Store, rooms *RoomManager) *LobbyPages {
	return &LobbyPages{store: store, rooms: rooms}
}

func (p *LobbyPages) Mount(mux *http.ServeMux) {
	mux.HandleFunc("GET /lobby", p.handleLobby)
	mux.HandleFunc("GET /partials/lobby", p.handleLobbyPartial)
	mux.HandleFunc("POST /lobby/{id}/join", p.handleJoin)
}

func lobbyPage() *template.Template {
	return template.Must(template.ParseFS(shellTmplFS,
		"web/templates/base.tmpl", "web/templates/rooms.tmpl",
		"web/templates/board.html", "web/templates/lobby.tmpl"))
}

var (
	lobbyTmpl     = lobbyPage()
	lobbyListTmpl = template.Must(template.ParseFS(shellTmplFS,
		"web/templates/lobby.tmpl", "web/templates/board.html"))
)

type lobbySeatView struct {
	Name    string
	Tier    string
	Profile string
}

type lobbyRoomView struct {
	ID         string
	ShortID    string
	Mode       string
	Red        lobbySeatView
	Blue       lobbySeatView
	TC         string
	BO         string
	Spectators int
	Status     string
	MoveCount  int
	CanJoin    bool
	boardData
}

type lobbyListView struct {
	Me    *shellViewer
	Rooms []lobbyRoomView
}

type lobbyView struct {
	Me         *shellViewer
	Rooms      []lobbyRoomView
	TCOptions  []optionView
	BOOptions  []optionView
	BotOptions []botOptionView
	PollMs     int64
	JoinError  string
}

func lobbySeatViewOf(s LobbySeat) lobbySeatView {
	v := lobbySeatView{Name: s.Name, Tier: s.Tier}
	switch {
	case s.Tier != "":
		for i := range config.Tiers {
			if config.Tiers[i].Name == s.Tier {
				v.Profile = "/players/" + url.PathEscape(config.BotAccountName(i))
			}
		}
	case s.Name != openSeatLabel && s.Name != "":
		v.Profile = "/players/" + url.PathEscape(s.Name)
	}
	return v
}

func (p *LobbyPages) lobbyViews(me *shellViewer) []lobbyRoomView {
	snap := p.rooms.LobbySnapshot()
	out := make([]lobbyRoomView, 0, len(snap))
	for _, lr := range snap {
		red, blue := lr.Host, lr.Guest
		if lr.Live && !lr.RedIsHost {
			red, blue = lr.Guest, lr.Host
		}
		out = append(out, lobbyRoomView{
			ID:         lr.ID,
			ShortID:    lr.ID[:8],
			Mode:       lr.Mode,
			Red:        lobbySeatViewOf(red),
			Blue:       lobbySeatViewOf(blue),
			TC:         tcLabel(lr.TCIdx),
			BO:         "bo" + strconv.Itoa(lr.BOLen),
			Spectators: lr.Spectators,
			Status:     lr.Status,
			MoveCount:  lr.MoveCount,
			CanJoin:    lr.Joinable && me != nil && me.ID != lr.HostUserID,
			boardData:  boardData{Cells: boardCells(lr.Moves, nil), BoardSize: config.BoardSize, Mini: true},
		})
	}
	return out
}

func (p *LobbyPages) handleLobby(w http.ResponseWriter, r *http.Request) {
	me, err := resolveViewer(p.store, r)
	if err != nil {
		http.Error(w, "lobby read failed", http.StatusInternalServerError)
		return
	}
	p.renderLobby(w, http.StatusOK, me, "")
}

func (p *LobbyPages) renderLobby(w http.ResponseWriter, status int, me *shellViewer, joinErr string) {
	tc := shellTCOptions()
	tc[0].Selected = true
	bo := shellBOOptions()
	bo[0].Selected = true
	renderShell(w, status, lobbyTmpl, "base", lobbyView{
		Me: me, Rooms: p.lobbyViews(me),
		TCOptions: tc, BOOptions: bo, BotOptions: shellBotOptions(),
		PollMs: int64(config.PagePollMs), JoinError: joinErr,
	})
}

func (p *LobbyPages) handleLobbyPartial(w http.ResponseWriter, r *http.Request) {
	me, err := resolveViewer(p.store, r)
	if err != nil {
		http.Error(w, "lobby read failed", http.StatusInternalServerError)
		return
	}
	renderShell(w, http.StatusOK, lobbyListTmpl, "lobbylist", lobbyListView{Me: me, Rooms: p.lobbyViews(me)})
}

func joinFailMessage(code string) string {
	switch code {
	case codeRoomNotFound:
		return "room not found or already closed"
	case codeRoomFull:
		return "room already has an opponent"
	case codeRoomClosed:
		return "room closed"
	case codeBadRequest:
		return "this room cannot be joined"
	default:
		return "join failed, try again"
	}
}

func (p *LobbyPages) handleJoin(w http.ResponseWriter, r *http.Request) {
	me, err := resolveViewer(p.store, r)
	if err != nil {
		http.Error(w, "join read failed", http.StatusInternalServerError)
		return
	}
	if me == nil {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}
	id := r.PathValue("id")
	if !isRoomID(id) {
		p.renderLobby(w, http.StatusBadRequest, me, joinFailMessage(codeRoomNotFound))
		return
	}
	if err := p.rooms.Join(id, me.ID); err != nil {
		code, status := errorResponse(err)
		if status == http.StatusInternalServerError {
			http.Error(w, "join failed", status)
			return
		}
		p.renderLobby(w, status, me, joinFailMessage(code))
		return
	}
	http.Redirect(w, r, "/rooms/"+id, http.StatusSeeOther)
}
