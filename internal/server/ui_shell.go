package server

import (
	"bytes"
	"context"
	"database/sql"
	"embed"
	"encoding/hex"
	"errors"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/lavantien/caro-ai-pvp/internal/config"
)

//go:embed web/templates
var shellTmplFS embed.FS

//go:embed web/shell.css
var shellCSS []byte

const historyTimeFormat = "2006-01-02 15:04"

func shellPage(page string) *template.Template {
	return template.Must(template.ParseFS(shellTmplFS,
		"web/templates/base.tmpl", "web/templates/rooms.tmpl", "web/templates/"+page))
}

var (
	homeTmpl    = shellPage("home.tmpl")
	loginTmpl   = shellPage("login.tmpl")
	historyTmpl = shellPage("history.tmpl")
	roomsTmpl   = template.Must(template.ParseFS(shellTmplFS, "web/templates/rooms.tmpl"))
)

type shellPages struct {
	store   *Store
	rooms   *RoomManager
	tourney TourneyService
}

func NewShellPages(store *Store, rooms *RoomManager, tour TourneyService) http.Handler {
	p := &shellPages{store: store, rooms: rooms, tourney: tour}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /{$}", p.handleHome)
	mux.HandleFunc("GET /login", p.handleLoginForm)
	mux.HandleFunc("POST /login", p.handleLogin)
	mux.HandleFunc("POST /logout", p.handleLogout)
	mux.HandleFunc("GET /history", p.handleHistory)
	mux.HandleFunc("GET /partials/rooms", p.handleRoomsPartial)
	mux.HandleFunc("GET /rooms", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/", http.StatusSeeOther)
	})
	mux.HandleFunc("POST /rooms", p.handleCreateRoom)
	mux.HandleFunc("GET /shell.css", p.handleShellCSS)
	mountPWA(mux)
	return mux
}

type shellViewer struct {
	ID       int64
	Username string
	Rating   int
	Wins     int
	Losses   int
	Draws    int
	Level    int
}

func (v shellViewer) WLD() string {
	return fmt.Sprintf("%dW-%dL-%dD", v.Wins, v.Losses, v.Draws)
}

func (p *shellPages) me(r *http.Request) (*shellViewer, error) {
	return resolveViewer(p.store, r)
}

func resolveViewer(store *Store, r *http.Request) (*shellViewer, error) {
	token, err := sessionToken(r)
	if err != nil {
		return nil, nil
	}
	u, err := Authenticate(store, token, time.Now().Unix())
	if err != nil {
		if errors.Is(err, ErrBadCredentials) {
			return nil, nil
		}
		return nil, err
	}
	rating, err := currentRating(store, u.ID)
	if err != nil {
		return nil, err
	}
	stats, err := store.UserStats(u.ID)
	if err != nil {
		return nil, err
	}
	return &shellViewer{
		ID: u.ID, Username: u.Username, Rating: rating,
		Wins: stats.Wins, Losses: stats.Losses, Draws: stats.Draws, Level: stats.SeriesWon,
	}, nil
}

func renderShell(w http.ResponseWriter, status int, tmpl *template.Template, name string, data any) {
	var buf bytes.Buffer
	if err := tmpl.ExecuteTemplate(&buf, name, data); err != nil {
		http.Error(w, "shell render failed", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_, _ = w.Write(buf.Bytes())
}

func (p *shellPages) handleShellCSS(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/css; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache")
	_, _ = w.Write(shellCSS)
}

type optionView struct {
	Value    int
	Label    string
	Selected bool
}

type botOptionView struct {
	Value    string
	Label    string
	Selected bool
}

func shellTCOptions() []optionView {
	opts := make([]optionView, len(config.TimeControls))
	for i := range config.TimeControls {
		opts[i] = optionView{Value: i, Label: tcLabel(i)}
	}
	return opts
}

func shellBOOptions() []optionView {
	opts := make([]optionView, len(config.SeriesLengths))
	for i, bo := range config.SeriesLengths {
		opts[i] = optionView{Value: bo, Label: "bo" + strconv.Itoa(bo)}
	}
	return opts
}

func shellBotOptions() []botOptionView {
	opts := []botOptionView{{Label: "human, open seat"}}
	for i := range config.Tiers {
		opts = append(opts, botOptionView{
			Value: config.Tiers[i].Name, Label: "AI " + config.Tiers[i].Name,
		})
	}
	return opts
}

type roomCardView struct {
	ID        string
	ShortID   string
	TC        string
	BO        string
	State     string
	Host      string
	Guest     string
	Score     string
	ShowScore bool
}

func (p *shellPages) roomViews() []roomCardView {
	infos := p.rooms.List()
	out := make([]roomCardView, 0, len(infos))
	for _, info := range infos {
		v := roomCardView{
			ID: info.ID, ShortID: info.ID[:8],
			TC: tcLabel(info.TCIdx), BO: "bo" + strconv.Itoa(info.BOLen),
			State: info.State.String(),
		}
		if info.HostBotName != "" {
			v.Host = info.HostBotName
		} else {
			v.Host = p.seatName(info.HostUserID)
		}
		switch {
		case info.GuestBotName != "":
			v.Guest = info.GuestBotName
		case info.GuestUserID != 0:
			v.Guest = p.seatName(info.GuestUserID)
		default:
			v.Guest = "open seat"
		}
		if info.State != SeriesCreated {
			v.Score = strconv.Itoa(info.HostWins) + "-" + strconv.Itoa(info.GuestWins)
			v.ShowScore = true
		}
		out = append(out, v)
	}
	return out
}

func (p *shellPages) seatName(id int64) string {
	u, err := p.store.UserByID(id)
	if err != nil {
		return "#" + strconv.FormatInt(id, 10)
	}
	return u.Username
}

type tourneyBannerView struct {
	RunID int64
	Done  int
	Total int
}

type roomsListView struct {
	Rooms []roomCardView
	Live  *tourneyBannerView
}

func (p *shellPages) banner(ctx context.Context) *tourneyBannerView {
	if p.tourney == nil {
		return nil
	}
	b, ok, err := p.tourney.OngoingRun(ctx)
	if err != nil {
		log.Printf("server: home tournament banner read: %v", err)
		return nil
	}
	if !ok {
		return nil
	}
	return &tourneyBannerView{RunID: b.RunID, Done: b.Done, Total: b.Total}
}

func (p *shellPages) roomsList(ctx context.Context) roomsListView {
	return roomsListView{Rooms: p.roomViews(), Live: p.banner(ctx)}
}

type homeView struct {
	Me          *shellViewer
	RoomsList   roomsListView
	TCOptions   []optionView
	BOOptions   []optionView
	BotOptions  []botOptionView
	CreateError string
	PollMs      int64
}

func (p *shellPages) handleHome(w http.ResponseWriter, r *http.Request) {
	me, err := p.me(r)
	if err != nil {
		http.Error(w, "home read failed", http.StatusInternalServerError)
		return
	}
	p.renderHome(r, w, http.StatusOK, me, "")
}

func (p *shellPages) renderHome(r *http.Request, w http.ResponseWriter, status int, me *shellViewer, createErr string) {
	renderShell(w, status, homeTmpl, "base", homeView{
		Me: me, RoomsList: p.roomsList(r.Context()),
		TCOptions: shellTCOptions(), BOOptions: shellBOOptions(), BotOptions: shellBotOptions(),
		CreateError: createErr, PollMs: int64(config.PagePollMs),
	})
}

type loginView struct {
	Me         *shellViewer
	Error      string
	MaxNameLen int
}

func (p *shellPages) handleLoginForm(w http.ResponseWriter, r *http.Request) {
	me, err := p.me(r)
	if err != nil {
		http.Error(w, "login read failed", http.StatusInternalServerError)
		return
	}
	if me != nil {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}
	renderShell(w, http.StatusOK, loginTmpl, "base",
		loginView{MaxNameLen: config.UsernameMaxBytes})
}

func loginMessage(code string) string {
	switch code {
	case codeBadCredentials:
		return "wrong password"
	case codeInvalidUsername:
		return "invalid username: 1 to " + strconv.Itoa(config.UsernameMaxBytes) +
			" bytes, no control characters"
	default:
		return "login failed, try again"
	}
}

func (p *shellPages) handleLogin(w http.ResponseWriter, r *http.Request) {
	fail := func(status int, msg string) {
		renderShell(w, status, loginTmpl, "base",
			loginView{Error: msg, MaxNameLen: config.UsernameMaxBytes})
	}
	r.Body = http.MaxBytesReader(w, r.Body, httpBodyLimitBytes)
	if err := r.ParseForm(); err != nil {
		fail(http.StatusBadRequest, "malformed form body")
		return
	}
	_, sess, err := LoginOrCreate(p.store, r.PostFormValue("username"),
		r.PostFormValue("password"), time.Now().Unix())
	if err != nil {
		code, status := errorResponse(err)
		fail(status, loginMessage(code))
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
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func (p *shellPages) handleLogout(w http.ResponseWriter, r *http.Request) {
	if token, err := sessionToken(r); err == nil {
		if err := Logout(p.store, token); err != nil {
			http.Error(w, "logout failed", http.StatusInternalServerError)
			return
		}
	}
	http.SetCookie(w, &http.Cookie{
		Name: sessionCookieName, Value: "", Path: "/", MaxAge: -1, HttpOnly: true,
	})
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

type historyGameRow struct {
	MatchHistoryRow
	GameID int64
}

func (s *Store) matchHistoryWithIDs(userID int64) ([]historyGameRow, error) {
	rows, err := s.db.Query(`
SELECT g.id, g.played_at, ru.username, bu.username,
	(SELECT COUNT(*) FROM games w
	 WHERE w.series_id = g.series_id AND w.idx_in_series <= g.idx_in_series
	   AND ((w.outcome = ? AND w.red_user = g.red_user) OR (w.outcome = ? AND w.blue_user = g.red_user))),
	(SELECT COUNT(*) FROM games w
	 WHERE w.series_id = g.series_id AND w.idx_in_series <= g.idx_in_series
	   AND ((w.outcome = ? AND w.red_user = g.blue_user) OR (w.outcome = ? AND w.blue_user = g.blue_user))),
	g.full_turns, g.moves, g.won_by
FROM games g
JOIN users ru ON ru.id = g.red_user
JOIN users bu ON bu.id = g.blue_user
WHERE g.red_user = ? OR g.blue_user = ?
ORDER BY g.played_at DESC, g.id DESC`,
		OutcomeRed, OutcomeBlue, OutcomeRed, OutcomeBlue, userID, userID,
	)
	if err != nil {
		return nil, fmt.Errorf("server: match history with ids %d: %w", userID, err)
	}
	defer func() { _ = rows.Close() }()
	var out []historyGameRow
	for rows.Next() {
		var r historyGameRow
		var wonBy sql.NullString
		if err := rows.Scan(&r.GameID, &r.PlayedAt, &r.Red, &r.Blue,
			&r.RedWins, &r.BlueWins, &r.FullTurns, &r.Moves, &wonBy); err != nil {
			return nil, fmt.Errorf("server: match history with ids %d: %w", userID, err)
		}
		r.WonBy = nullString(wonBy)
		out = append(out, r)
	}
	return out, rows.Err()
}

type historyRowView struct {
	GameID    int64
	When      string
	Red       string
	Blue      string
	Score     string
	Turns     int
	WonBy     string
	Preview   string
	Truncated bool
}

type historyView struct {
	Me   *shellViewer
	Rows []historyRowView
}

func (p *shellPages) handleHistory(w http.ResponseWriter, r *http.Request) {
	me, err := p.me(r)
	if err != nil {
		http.Error(w, "history read failed", http.StatusInternalServerError)
		return
	}
	if me == nil {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}
	gameRows, err := p.store.matchHistoryWithIDs(me.ID)
	if err != nil {
		http.Error(w, "history read failed", http.StatusInternalServerError)
		return
	}
	view := historyView{Me: me, Rows: make([]historyRowView, 0, len(gameRows))}
	for _, row := range gameRows {
		e, err := historyEntryOf(row.MatchHistoryRow)
		if err != nil {
			continue
		}
		hv := historyRowView{
			GameID:    row.GameID,
			When:      time.Unix(e.PlayedAt, 0).UTC().Format(historyTimeFormat),
			Red:       e.Red,
			Blue:      e.Blue,
			Score:     strconv.Itoa(e.RedWins) + "-" + strconv.Itoa(e.BlueWins),
			Turns:     e.FullTurns,
			Preview:   strings.Join(e.Preview, " "),
			Truncated: e.Truncated,
		}
		if e.WonBy != nil {
			hv.WonBy = *e.WonBy
		}
		view.Rows = append(view.Rows, hv)
	}
	renderShell(w, http.StatusOK, historyTmpl, "base", view)
}

func (p *shellPages) handleRoomsPartial(w http.ResponseWriter, r *http.Request) {
	renderShell(w, http.StatusOK, roomsTmpl, "roomslist", p.roomsList(r.Context()))
}

func (p *shellPages) renderCreateError(r *http.Request, w http.ResponseWriter, me *shellViewer,
	status int, msg string) {
	if r.PostFormValue("origin") == "lobby" {
		renderLobbyPage(w, status, p.rooms, me, msg)
		return
	}
	p.renderHome(r, w, status, me, msg)
}

func (p *shellPages) handleCreateRoom(w http.ResponseWriter, r *http.Request) {
	me, err := p.me(r)
	if err != nil {
		http.Error(w, "create read failed", http.StatusInternalServerError)
		return
	}
	if me == nil {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, httpBodyLimitBytes)
	if err := r.ParseForm(); err != nil {
		p.renderCreateError(r, w, me, http.StatusBadRequest, "invalid room settings, pick from the lists")
		return
	}
	bad := func() {
		p.renderCreateError(r, w, me, http.StatusBadRequest, "invalid room settings, pick from the lists")
	}
	tcIdx, errTC := strconv.Atoi(r.PostFormValue("tc"))
	boLen, errBO := strconv.Atoi(r.PostFormValue("bo"))
	if errTC != nil || errBO != nil {
		bad()
		return
	}
	var tier *config.Tier
	if bot := r.PostFormValue("bot"); bot != "" {
		if tier = tierByName(bot); tier == nil {
			bad()
			return
		}
	}
	room, err := p.rooms.Create(me.ID, tcIdx, boLen, tier)
	if err != nil {
		if _, status := errorResponse(err); status == http.StatusInternalServerError {
			http.Error(w, "room create failed", status)
			return
		}
		if errors.Is(err, ErrMachineBusy) {
			p.renderCreateError(r, w, me, http.StatusConflict,
				"the machine is fully booked by bot rooms, wait for one to close and try again")
			return
		}
		bad()
		return
	}
	http.Redirect(w, r, "/rooms/"+room.ID(), http.StatusSeeOther)
}
