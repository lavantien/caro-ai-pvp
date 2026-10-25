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
	"io/fs"
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

const shortIDEdge = 4

func shortID(id string) string {
	if len(id) <= 2*shortIDEdge {
		return id
	}
	return id[:shortIDEdge] + id[len(id)-shortIDEdge:]
}

func botNameDisplay(name string) string {
	i := strings.LastIndexByte(name, '-')
	if i < 0 || !isRoomID(name[i+1:]) {
		return name
	}
	return name[:i+1] + shortID(name[i+1:])
}

var uiFuncs = template.FuncMap{"disp": botNameDisplay}

func parseUI(files ...string) *template.Template {
	return parseUIFS(shellTmplFS, files...)
}

func parseUIFS(fsys fs.FS, files ...string) *template.Template {
	return template.Must(template.New("ui").Funcs(uiFuncs).ParseFS(fsys, files...))
}

func shellPage(page string) *template.Template {
	return parseUI(
		"web/templates/base.tmpl", "web/templates/rooms.tmpl", "web/templates/seg.tmpl",
		"web/templates/brand.html", "web/templates/"+page)
}

var (
	homeTmpl    = shellPage("home.tmpl")
	loginTmpl   = shellPage("login.tmpl")
	historyTmpl = shellPage("history.tmpl")
	roomsTmpl   = parseUI("web/templates/rooms.tmpl", "web/templates/brand.html")
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
	Level    int
}

type viewerVariantStats struct {
	Variant string
	WLD     string
	Rating  int
	Level   int
}

func wldString(w, l, d int) string {
	return strconv.Itoa(w) + "W-" + strconv.Itoa(l) + "L-" + strconv.Itoa(d) + "D"
}

func variantStatsOf(byVariant map[string]wldRecord, ratings, levels map[string]int) []viewerVariantStats {
	out := make([]viewerVariantStats, 0, len(config.Variants))
	for i := range config.Variants {
		name := config.Variants[i].Name
		rec := byVariant[name]
		rating := config.RatingStart
		if r, ok := ratings[name]; ok {
			rating = r
		}
		out = append(out, viewerVariantStats{
			Variant: name, WLD: wldString(rec.Wins, rec.Losses, rec.Draws),
			Rating: rating, Level: levels[name],
		})
	}
	return out
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
		ID: u.ID, Username: u.Username, Rating: rating, Level: stats.SeriesWon,
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

type segOption struct {
	Value    string
	Label    string
	Selected bool
}

type segView struct {
	Name    string
	Tight   bool
	Options []segOption
}

func (v segView) defaultFirst() segView {
	for _, o := range v.Options {
		if o.Selected {
			return v
		}
	}
	if len(v.Options) > 0 {
		v.Options[0].Selected = true
	}
	return v
}

func intSeg(name string, opts []optionView) segView {
	seg := segView{Name: name, Options: make([]segOption, len(opts))}
	for i, o := range opts {
		seg.Options[i] = segOption{
			Value: strconv.Itoa(o.Value), Label: o.Label, Selected: o.Selected,
		}
	}
	return seg.defaultFirst()
}

func stringSeg(name string, opts []botOptionView) segView {
	seg := segView{Name: name, Options: make([]segOption, len(opts))}
	for i, o := range opts {
		seg.Options[i] = segOption{Value: o.Value, Label: o.Label, Selected: o.Selected}
	}
	return seg.defaultFirst()
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

func shellVariantOptions() []botOptionView {
	opts := make([]botOptionView, len(config.Variants))
	for i := range config.Variants {
		opts[i] = botOptionView{
			Value: config.Variants[i].Name, Label: variantLabel(config.Variants[i]),
			Selected: config.Variants[i].Name == config.DefaultVariant,
		}
	}
	return opts
}

func variantLabel(spec config.VariantSpec) string {
	if spec.Kind == "hex" {
		return "hexagon " + strconv.Itoa(spec.Size)
	}
	return "square " + strconv.Itoa(spec.Size) + "x" + strconv.Itoa(spec.Size)
}

type roomCardView struct {
	ID        string
	ShortID   string
	TC        string
	BO        string
	Variant   string
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
			ID: info.ID, ShortID: shortID(info.ID),
			TC: tcLabel(info.TCIdx), BO: "bo" + strconv.Itoa(info.BOLen),
			Variant: info.Variant, State: info.State.String(),
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
	Variants    []viewerVariantStats
	RoomsList   roomsListView
	SegTC       segView
	SegBO       segView
	SegVariant  segView
	BotOptions  []botOptionView
	CreateError string
	PollMs      int64
}

func (p *shellPages) variantRows(userID int64) ([]viewerVariantStats, error) {
	byVariant, err := p.store.UserWLDByVariant(userID)
	if err != nil {
		return nil, err
	}
	ratings, err := p.store.UserRatingByVariant(userID)
	if err != nil {
		return nil, err
	}
	levels, err := p.store.UserSeriesWonByVariant(userID)
	if err != nil {
		return nil, err
	}
	return variantStatsOf(byVariant, ratings, levels), nil
}

func (p *shellPages) handleHome(w http.ResponseWriter, r *http.Request) {
	me, err := p.me(r)
	if err != nil {
		http.Error(w, "home read failed", http.StatusInternalServerError)
		return
	}
	var variants []viewerVariantStats
	if me != nil {
		if variants, err = p.variantRows(me.ID); err != nil {
			http.Error(w, "home read failed", http.StatusInternalServerError)
			return
		}
	}
	p.renderHome(r, w, http.StatusOK, me, variants, "")
}

func (p *shellPages) renderHome(r *http.Request, w http.ResponseWriter, status int,
	me *shellViewer, variants []viewerVariantStats, createErr string) {
	renderShell(w, status, homeTmpl, "base", homeView{
		Me: me, Variants: variants, RoomsList: p.roomsList(r.Context()),
		SegTC:       intSeg("tc", shellTCOptions()),
		SegBO:       intSeg("bo", shellBOOptions()),
		SegVariant:  stringSeg("variant", shellVariantOptions()),
		BotOptions:  shellBotOptions(),
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
	GameID   int64
	RedUser  int64
	BlueUser int64
	RedBot   bool
	BlueBot  bool
}

func (s *Store) matchHistoryWithIDs(userID int64, variant string) ([]historyGameRow, error) {
	variantFilter := ""
	args := []any{OutcomeRed, OutcomeBlue, OutcomeRed, OutcomeBlue, userID, userID}
	if variant != "" {
		variantFilter = " AND COALESCE(se.variant, 'square') = ?"
		args = append(args, variant)
	}
	rows, err := s.db.Query(`
SELECT g.id, g.played_at,
	CASE WHEN length(ru.salt) = 0 AND length(ru.hash) = 0
	     THEN COALESCE(g.bot_name, ru.username) ELSE ru.username END,
	CASE WHEN length(bu.salt) = 0 AND length(bu.hash) = 0
	     THEN COALESCE(g.bot_name, bu.username) ELSE bu.username END,
	(SELECT COUNT(*) FROM games w
	 WHERE w.series_id = g.series_id AND w.idx_in_series <= g.idx_in_series
	   AND ((w.outcome = ? AND w.red_user = g.red_user) OR (w.outcome = ? AND w.blue_user = g.red_user))),
	(SELECT COUNT(*) FROM games w
	 WHERE w.series_id = g.series_id AND w.idx_in_series <= g.idx_in_series
	   AND ((w.outcome = ? AND w.red_user = g.blue_user) OR (w.outcome = ? AND w.blue_user = g.blue_user))),
	g.full_turns, COALESCE(se.variant, 'square'), g.moves, g.won_by,
	g.red_user, g.blue_user,
	CASE WHEN length(ru.salt) = 0 AND length(ru.hash) = 0 THEN 1 ELSE 0 END,
	CASE WHEN length(bu.salt) = 0 AND length(bu.hash) = 0 THEN 1 ELSE 0 END
FROM games g
JOIN users ru ON ru.id = g.red_user
JOIN users bu ON bu.id = g.blue_user
JOIN series se ON se.id = g.series_id
WHERE (g.red_user = ? OR g.blue_user = ?)`+variantFilter+`
ORDER BY g.played_at DESC, g.id DESC`,
		args...,
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
			&r.RedWins, &r.BlueWins, &r.FullTurns, &r.Variant, &r.Moves, &wonBy,
			&r.RedUser, &r.BlueUser, &r.RedBot, &r.BlueBot); err != nil {
			return nil, fmt.Errorf("server: match history with ids %d: %w", userID, err)
		}
		r.WonBy = nullString(wonBy)
		out = append(out, r)
	}
	return out, rows.Err()
}

type playerStatView struct {
	SeatClass string
	Seat      string
	Name      string
	WLD       string
}

type historyRowView struct {
	GameID        int64
	When          string
	Red           string
	Blue          string
	Variant       string
	Score         string
	Turns         int
	WonBy         string
	MoveItems     []moveItemView
	Truncated     bool
	IsBot         bool
	StatLines     []string
	StatTruncated bool
	Players       []playerStatView
}

func playerStatsOf(seat, name string, rec wldRecord) playerStatView {
	class := "s-blue"
	if seat == "red" {
		class = "s-red"
	}
	return playerStatView{SeatClass: class, Seat: seat, Name: name,
		WLD: wldString(rec.Wins, rec.Losses, rec.Draws)}
}

type historyView struct {
	Me      *shellViewer
	Variant string
	Rows    []historyRowView
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
	variant := r.URL.Query().Get("variant")
	if variant != "" {
		if _, _, err := resolveVariant(variant); err != nil {
			http.Error(w, "unknown variant", http.StatusBadRequest)
			return
		}
	}
	gameRows, err := p.store.matchHistoryWithIDs(me.ID, variant)
	if err != nil {
		http.Error(w, "history read failed", http.StatusInternalServerError)
		return
	}
	wldIDs := make([]int64, 0, 2*len(gameRows))
	for _, row := range gameRows {
		if !row.RedBot && !row.BlueBot {
			wldIDs = append(wldIDs, row.RedUser, row.BlueUser)
		}
	}
	wlds, err := p.store.userWLDs(wldIDs)
	if err != nil {
		http.Error(w, "history read failed", http.StatusInternalServerError)
		return
	}
	view := historyView{Me: me, Variant: variant, Rows: make([]historyRowView, 0, len(gameRows))}
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
			Variant:   row.Variant,
			Score:     strconv.Itoa(e.RedWins) + "-" + strconv.Itoa(e.BlueWins),
			Turns:     e.FullTurns,
			MoveItems: newestMoveItems(e.Preview),
			Truncated: e.Truncated,
			IsBot:     row.RedBot || row.BlueBot,
		}
		if e.WonBy != nil {
			hv.WonBy = *e.WonBy
		}
		if hv.IsBot {
			lines, more, err := p.store.gameStatTail(row.GameID, config.HistoryStatTail)
			if err != nil {
				http.Error(w, "history read failed", http.StatusInternalServerError)
				return
			}
			hv.StatLines, hv.StatTruncated = lines, more
		} else {
			hv.Players = []playerStatView{
				playerStatsOf("red", e.Red, wlds[row.RedUser]),
				playerStatsOf("blue", e.Blue, wlds[row.BlueUser]),
			}
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
	variants, err := p.variantRows(me.ID)
	if err != nil {
		http.Error(w, "create read failed", http.StatusInternalServerError)
		return
	}
	if r.PostFormValue("origin") == "lobby" {
		renderLobbyPage(w, status, p.rooms, me, msg)
		return
	}
	p.renderHome(r, w, status, me, variants, msg)
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
	if _, _, err := resolveVariant(r.PostFormValue("variant")); err != nil {
		bad()
		return
	}
	room, err := p.rooms.Create(me.ID, tcIdx, boLen, tier, r.PostFormValue("variant"))
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
		if errors.Is(err, ErrServerResourcesFull) {
			p.renderCreateError(r, w, me, http.StatusConflict,
				"server resources are currently full, try again later")
			return
		}
		bad()
		return
	}
	http.Redirect(w, r, "/rooms/"+room.ID(), http.StatusSeeOther)
}
