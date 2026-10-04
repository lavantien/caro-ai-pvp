package server

// The M6b shell UI over the landed M6a backend: the login, home, and
// match-history pages of first-cause.md Scenario 1, server-rendered from
// the same store and room manager the JSON API reads. html/template with
// embedded templates, htmx only where a partial swap earns its keep (the
// cross-room grid poll), plain form POSTs everywhere else so the shell
// works without any JavaScript. The room view and the playback board are
// the M6b room-page agent's pages: this shell only links into them by
// path contract (/rooms/{id} and /rooms/history/{gameID}), 404 or not.

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

// shellCSS is the one stylesheet the shell pages own, served at
// /shell.css. It lives beside the vendored htmx tree but outside it (the
// vendor dir stays the exact pinned distribution), and it ships in the
// binary, so no-cache revalidation keeps a redeploy from serving stale
// style under the same URL.
//
//go:embed web/shell.css
var shellCSS []byte

// The rooms grid polls its partial at config.PagePollMs. The grid
// aggregates every live room while the SSE hub streams per-room events, so
// one cheap manager snapshot per viewer beats wiring a cross-room fan-out
// topic or one stream per room; polling is the simple correct tool here.

// historyTimeFormat renders the play stamp dense and sortable, UTC so the
// line never depends on the server's locale.
const historyTimeFormat = "2006-01-02 15:04"

// shellPage parses the base layout, the shared rooms fragment, and one
// page template; every page defines "content" on top of "base".
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

// shellPages carries the backend dependencies the pages read: the
// auth/rating/history store, the live-room manager (the same pair the JSON
// API rides), and the tourney service behind the home banner.
type shellPages struct {
	store   *Store
	rooms   *RoomManager
	tourney TourneyService
}

// NewShellPages builds the M6b shell UI over store, rooms, and the tourney
// service whose ongoing run the home banner reads (nil: no banner):
//
//	GET  /{$}            home: stats line, rooms grid, create room, M7 stub
//	GET  /login          the login/create form
//	POST /login          form login; the same session cookie as /api/login
//	POST /logout         drops the session and expires the cookie
//	GET  /history        the match history tab (guests bounce to /login)
//	GET  /partials/rooms the rooms grid fragment the home page polls
//	POST /rooms          the create-room form submit (guests bounce)
//	GET  /shell.css      the one stylesheet the shell pages own
//
// The returned handler is a ServeMux of exact patterns, so it mounts whole
// or pattern by pattern under the API mux.
func NewShellPages(store *Store, rooms *RoomManager, tour TourneyService) http.Handler {
	p := &shellPages{store: store, rooms: rooms, tourney: tour}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /{$}", p.handleHome)
	mux.HandleFunc("GET /login", p.handleLoginForm)
	mux.HandleFunc("POST /login", p.handleLogin)
	mux.HandleFunc("POST /logout", p.handleLogout)
	mux.HandleFunc("GET /history", p.handleHistory)
	mux.HandleFunc("GET /partials/rooms", p.handleRoomsPartial)
	// The collection URL is the home grid: only the form POST lives here,
	// so a typed GET lands on home instead of a bare 405.
	mux.HandleFunc("GET /rooms", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/", http.StatusSeeOther)
	})
	mux.HandleFunc("POST /rooms", p.handleCreateRoom)
	mux.HandleFunc("GET /shell.css", p.handleShellCSS)
	return mux
}

// shellViewer is the resolved session for the shell pages: the user id the
// acting handlers need plus the summary line the header and home stats
// render, from the same reads as /api/me.
type shellViewer struct {
	ID       int64
	Username string
	Rating   int
	Wins     int
	Losses   int
	Draws    int
	Level    int
}

// WLD renders the clickable W-L-D field into the history tab.
func (v shellViewer) WLD() string {
	return fmt.Sprintf("%dW-%dL-%dD", v.Wins, v.Losses, v.Draws)
}

// me resolves the request's session into the viewer line, nil for a guest.
// An unusable token is just a guest; a store failure surfaces as an error.
func (p *shellPages) me(r *http.Request) (*shellViewer, error) {
	return resolveViewer(p.store, r)
}

// resolveViewer reads the request's session into the shell viewer line, the
// shared resolver of the shell and tournament pages: nil for a guest (a
// missing cookie, an unusable token, a dead session), a store failure as an
// error.
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

// renderShell writes one page (or fragment): session-dependent HTML, so
// every response is no-store.
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

// handleShellCSS serves the shell stylesheet; session-independent bytes,
// revalidated every fetch so a redeploy never serves stale style.
func (p *shellPages) handleShellCSS(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/css; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache")
	_, _ = w.Write(shellCSS)
}

// optionView is one create-room select option rendered from the config
// hub: the form value plus the human label, and whether the setup screen
// preselects it.
type optionView struct {
	Value    int
	Label    string
	Selected bool
}

// botOptionView is the opponent option: an empty value keeps the guest
// seat open for a human, a tier name seats that bot.
type botOptionView struct {
	Value    string
	Label    string
	Selected bool
}

// shellTCOptions mirrors config.TimeControls into the time-control select.
func shellTCOptions() []optionView {
	opts := make([]optionView, len(config.TimeControls))
	for i := range config.TimeControls {
		opts[i] = optionView{Value: i, Label: tcLabel(i)}
	}
	return opts
}

// shellBOOptions mirrors config.SeriesLengths into the best-of select.
func shellBOOptions() []optionView {
	opts := make([]optionView, len(config.SeriesLengths))
	for i, bo := range config.SeriesLengths {
		opts[i] = optionView{Value: bo, Label: "bo" + strconv.Itoa(bo)}
	}
	return opts
}

// shellBotOptions lists the open human seat first, then every config tier.
func shellBotOptions() []botOptionView {
	opts := []botOptionView{{Label: "human, open seat"}}
	for i := range config.Tiers {
		opts = append(opts, botOptionView{
			Value: config.Tiers[i].Name, Label: "AI " + config.Tiers[i].Name,
		})
	}
	return opts
}

// roomCardView is one rooms-grid card: the room page link, the settings,
// the state, both seats, and the series score once a series is under way.
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

// roomViews snapshots the manager grid, resolving both seats through the
// store; an unresolvable name falls back to the raw id so one broken
// lookup cannot break the listing.
func (p *shellPages) roomViews() []roomCardView {
	infos := p.rooms.List()
	out := make([]roomCardView, 0, len(infos))
	for _, info := range infos {
		v := roomCardView{
			ID: info.ID, ShortID: info.ID[:8],
			TC: tcLabel(info.TCIdx), BO: "bo" + strconv.Itoa(info.BOLen),
			State: info.State.String(),
		}
		// A bot host renders like a bot guest, named per room; seatName's
		// numeric fallback must never leak a synthetic seat id.
		if info.HostBotTier != "" {
			v.Host = botDisplayName(info.HostBotTier, info.ID)
		} else {
			v.Host = p.seatName(info.HostUserID)
		}
		switch {
		case info.VsBotTier != "":
			v.Guest = botDisplayName(info.VsBotTier, info.ID)
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

// seatName resolves one seat's user id, falling back to the numeric id.
func (p *shellPages) seatName(id int64) string {
	u, err := p.store.UserByID(id)
	if err != nil {
		return "#" + strconv.FormatInt(id, 10)
	}
	return u.Username
}

// tourneyBannerView is the home page's live-tournament line: the run page
// link with the settled-series progress.
type tourneyBannerView struct {
	RunID int64
	Done  int
	Total int
}

// roomsListView is the rooms grid fragment: the cards plus the live
// tournament banner, so both ride the one poll.
type roomsListView struct {
	Rooms []roomCardView
	Live  *tourneyBannerView
}

// banner reads the ongoing run behind the tourney seam. A missing seam or
// no live drive renders nothing; a read failure logs and renders nothing,
// the run page stays the source of truth for the run itself.
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

// roomsList shapes the polled fragment.
func (p *shellPages) roomsList(ctx context.Context) roomsListView {
	return roomsListView{Rooms: p.roomViews(), Live: p.banner(ctx)}
}

// homeView is the home page: the viewer line (nil for a guest), the rooms
// grid with the live-tournament banner, the create-room options rendered
// from config, an inline create error, and the poll cadence.
type homeView struct {
	Me          *shellViewer
	RoomsList   roomsListView
	TCOptions   []optionView
	BOOptions   []optionView
	BotOptions  []botOptionView
	CreateError string
	PollMs      int64
}

// handleHome renders the shell's front page for guests and members alike.
func (p *shellPages) handleHome(w http.ResponseWriter, r *http.Request) {
	me, err := p.me(r)
	if err != nil {
		http.Error(w, "home read failed", http.StatusInternalServerError)
		return
	}
	p.renderHome(r, w, http.StatusOK, me, "")
}

// renderHome paints the home page, optionally with an inline create-room
// error under the form. The banner read rides the request's context, so a
// client gone mid-render cancels the store reads behind it.
func (p *shellPages) renderHome(r *http.Request, w http.ResponseWriter, status int, me *shellViewer, createErr string) {
	renderShell(w, status, homeTmpl, "base", homeView{
		Me: me, RoomsList: p.roomsList(r.Context()),
		TCOptions: shellTCOptions(), BOOptions: shellBOOptions(), BotOptions: shellBotOptions(),
		CreateError: createErr, PollMs: int64(config.PagePollMs),
	})
}

// loginView is the login/create form page.
type loginView struct {
	Me         *shellViewer
	Error      string
	MaxNameLen int
}

// handleLoginForm paints the one-form login; a live session bounces home.
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

// loginMessage maps the wire error codes of the API envelope onto the
// inline form message; anything else is a generic retry line.
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

// handleLogin is the one-form auth of the spec as a server-side form POST:
// the same LoginOrCreate and the same session cookie attributes as
// /api/login, then onto the home page. A failure re-renders the form with
// the message inline under the button, at the envelope's status.
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

// handleLogout mirrors /api/logout: drop the token behind the cookie,
// expire the cookie, back to home. Without a usable cookie it still
// succeeds, natural expiry raced.
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

// historyGameRow couples one store MatchHistoryRow with its games.id, the
// key of the playback link contract.
type historyGameRow struct {
	MatchHistoryRow
	GameID int64
}

// matchHistoryWithIDs is the shell's history read: store.MatchHistory's
// query plus g.id, the key of the /rooms/history/{gameID} playback link
// contract. The shared MatchHistoryRow lacks the id (API gap reported
// with the milestone); widening the shared row would touch every consumer,
// so the shell owns this mirror of the query until the store lifts the id
// itself.
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

// historyRowView is one rendered history line: when, who vs who, the
// series score as of that game, the full-turn count, how the game was won,
// and the trimmed move preview linking into the playback board.
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

// historyView is the history tab page.
type historyView struct {
	Me   *shellViewer
	Rows []historyRowView
}

// handleHistory renders the match history tab: one line per game with the
// first config.HistoryPreviewTurns full turns of stones, the ellipsis
// past that, and the /rooms/history/{gameID} playback link (the room-page
// agent owns that board; it may 404 until M6b lands it). A row whose
// moves blob does not decode is skipped, not fatal, the same policy as
// /api/history.
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

// handleRoomsPartial serves the grid fragment the home page polls: the
// same card list and live-tournament banner, public like GET /api/rooms.
func (p *shellPages) handleRoomsPartial(w http.ResponseWriter, r *http.Request) {
	renderShell(w, http.StatusOK, roomsTmpl, "roomslist", p.roomsList(r.Context()))
}

// handleCreateRoom is the create-room form POST: settings validated by the
// same RoomManager.Create the JSON API rides, then straight into the room
// page (/rooms/{id}, the room-view agent's page). The form's options are
// rendered from the config hub, so the selects constrain every value
// client-side; the manager re-validates server-side.
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
	bad := func() {
		p.renderHome(r, w, http.StatusBadRequest, me, "invalid room settings, pick from the lists")
	}
	r.Body = http.MaxBytesReader(w, r.Body, httpBodyLimitBytes)
	if err := r.ParseForm(); err != nil {
		bad()
		return
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
		bad()
		return
	}
	http.Redirect(w, r, "/rooms/"+room.ID(), http.StatusSeeOther)
}
