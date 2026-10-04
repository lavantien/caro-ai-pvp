package server

// The M7 tournament UI of first-cause.md Scenario 2 over the tourney
// manager's service seam: the setup form (roster builder, time control,
// best-of, start rating, parallel matches), the live run page with the
// htmx-polled leaderboard, and the past-runs section. The pages depend on
// the TourneyService interface only, so page tests script the service while
// the serve composition wires the tourney manager over the room surface.
// The bot-vs-bot rooms of a live run stay on the home grid beside these
// pages; nothing here hides or duplicates them.

import (
	"context"
	"errors"
	"html/template"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/lavantien/caro-ai-pvp/internal/config"
)

// tourneyPage parses the base layout, the shared leaderboard fragment, and
// one tournament page template.
func tourneyPage(page string) *template.Template {
	return template.Must(template.ParseFS(shellTmplFS,
		"web/templates/base.tmpl", "web/templates/rooms.tmpl",
		"web/templates/tourney_board.tmpl", "web/templates/"+page))
}

var (
	tourneyTmpl      = tourneyPage("tourney.tmpl")
	tourneyRunTmpl   = tourneyPage("tourney_run.tmpl")
	tourneyBoardTmpl = template.Must(template.ParseFS(shellTmplFS, "web/templates/tourney_board.tmpl"))
)

// TourneySeat is one roster seat of a run.
type TourneySeat struct {
	Slot int
	Name string
	Tier string
}

// TourneySetup is one validated tournament start: the roster in slot order,
// the time control index, the series length, the start rating, and the
// parallel-room count.
type TourneySetup struct {
	Seats       []TourneySeat
	TCIdx       int
	BOLen       int
	StartRating int
	Parallel    int
}

// TourneySeriesLine is one pairing of a run: the seats as wins of the
// red-first participant, the settled winner, and the finish.
type TourneySeriesLine struct {
	PairingSlot   int
	RedFirstSlot  int
	BlueFirstSlot int
	RedFirstWins  int
	BlueFirstWins int
	WinnerSlot    *int
	Finished      bool
}

// TourneyStanding is one leaderboard row, ordered rating desc, wins desc,
// slot asc by the service that derives it.
type TourneyStanding struct {
	Slot        int
	Rating      int
	Wins        int
	Losses      int
	Draws       int
	SeriesWon   int
	GamesPlayed int
}

// TourneyRunInfo is one run header plus the drive state the pages render:
// Finished from the run row, Running while the manager still drives it,
// Failure the terminal drive error.
type TourneyRunInfo struct {
	ID          int64
	CreatedAt   int64
	TCIdx       int
	BOLen       int
	StartRating int
	Finished    bool
	Running     bool
	Failure     string
}

// TourneySnapshot is the run page's whole read.
type TourneySnapshot struct {
	Run    TourneyRunInfo
	Seats  []TourneySeat
	Series []TourneySeriesLine
	Board  []TourneyStanding
}

// TourneyRunSummary is one line of the setup page's runs section: the header
// plus the roster and the standings leader.
type TourneyRunSummary struct {
	TourneyRunInfo
	Seats  []TourneySeat
	Leader string
}

// TourneyLiveBoard is one ongoing bot series' spectating read for the run
// page: the room link, both seats under the room's naming law with the
// running series score told from red's side, the stones in play order, and
// the side to move.
type TourneyLiveBoard struct {
	RoomID   string
	RedName  string
	BlueName string
	RedWins  int
	BlueWins int
	Turn     string
	Moves    []string
}

// TourneyBanner is the home page's live-run read: the ongoing run's id and
// its settled-series progress over all pairings.
type TourneyBanner struct {
	RunID int64
	Done  int
	Total int
}

// TourneyBlockedError names the ongoing run holding the machine-wide run
// gate; the setup form renders the blocking id inline instead of a bare
// outage.
type TourneyBlockedError struct{ RunID int64 }

func (e *TourneyBlockedError) Error() string {
	return "run " + strconv.FormatInt(e.RunID, 10) +
		" is still in progress, close it before starting another"
}

// TourneyService is the tournament UI's one seam onto the M7 conductor. The
// serve composition wires the tourney manager over it; page tests wire a
// script.
type TourneyService interface {
	// StartRun validates, persists, and begins driving one tournament. It
	// returns once the run row exists, so the caller can redirect onto the
	// run's live page, and refuses with TourneyBlockedError while another
	// run holds the machine-wide gate.
	StartRun(ctx context.Context, setup TourneySetup) (int64, error)
	// RunSnapshot reads one run's whole render state; an unknown run maps
	// onto ErrNotFound.
	RunSnapshot(ctx context.Context, runID int64) (TourneySnapshot, error)
	// Runs lists run headers newest first with their rosters and leaders.
	Runs(ctx context.Context) ([]TourneyRunSummary, error)
	// OngoingRun reads the run this process currently drives, the home
	// page's live-tournament banner; ok is false when no drive is live.
	OngoingRun(ctx context.Context) (TourneyBanner, bool, error)
	// LiveBoards reads the ongoing run's live bot series for spectating.
	// One run holds the machine at a time, so every live bot-vs-bot room
	// belongs to the ongoing run; a driven run's page renders them, every
	// other page ignores them.
	LiveBoards() []TourneyLiveBoard
	// CloseStalledRun closes a stalled run's row, an ongoing run no live
	// drive owns. It refuses an unknown run with ErrNotFound and a live
	// drive or an already-closed run with an error the page renders inline.
	CloseStalledRun(ctx context.Context, runID int64) error
}

// TournamentPages serves the Scenario 2 setup, run, and leaderboard pages
// over a TourneyService and the same store the shell reads sessions
// through.
type TournamentPages struct {
	store   *Store
	tourney TourneyService
}

// NewTournamentPages builds the page handlers for the composition to mount.
func NewTournamentPages(store *Store, tourney TourneyService) *TournamentPages {
	return &TournamentPages{store: store, tourney: tourney}
}

// Mount registers the page routes on mux, the room pages' pattern:
//
//	GET  /tourney                 the runs list for everyone, the setup form for the admin
//	POST /tourney                 validate and start, redirect to the run page (admin only)
//	GET  /tourney/run/{id}        the run page (public, like the rooms grid)
//	GET  /tourney/run/{id}/board  the polled leaderboard fragment
//	POST /tourney/run/{id}/close  close a stalled run's row (admin only)
func (p *TournamentPages) Mount(mux *http.ServeMux) {
	mux.HandleFunc("GET /tourney", p.handleSetup)
	mux.HandleFunc("POST /tourney", p.handleStart)
	mux.HandleFunc("GET /tourney/run/{id}", p.handleRun)
	mux.HandleFunc("GET /tourney/run/{id}/board", p.handleBoard)
	mux.HandleFunc("POST /tourney/run/{id}/close", p.handleCloseRun)
}

// isAdmin names the one account the tournament controls answer to: the
// seeded admin row. Every viewer can read the pages; only this session
// starts runs and closes them.
func isAdmin(me *shellViewer) bool {
	return me != nil && me.Username == config.AdminName
}

// tourneyRowView is one roster-builder row: the index behind the form field
// names, the name input's value, and the tier select with its preselection.
type tourneyRowView struct {
	N     int
	Name  string
	Tiers []botOptionView
}

// tourneyRunLineView is one line of the setup page's runs section.
type tourneyRunLineView struct {
	ID     int64
	When   string
	TC     string
	BO     string
	State  string
	Roster string
	Leader string
}

// tourneySetupView is the setup page: the form options and prefill from the
// config hub (rendered for the admin only), an inline error, and the runs
// list everyone reads.
type tourneySetupView struct {
	Me              *shellViewer
	Admin           bool
	TCOptions       []optionView
	BOOptions       []optionView
	ParallelOptions []optionView
	CountOptions    []optionView
	Rows            []tourneyRowView
	StartRating     int
	MaxNameLen      int
	Error           string
	Runs            []tourneyRunLineView
}

// tourneyTierOptions lists every config tier as a select option, marking the
// wanted one selected.
func tourneyTierOptions(selected string) []botOptionView {
	opts := make([]botOptionView, len(config.Tiers))
	for i := range config.Tiers {
		opts[i] = botOptionView{
			Value: config.Tiers[i].Name, Label: "AI " + config.Tiers[i].Name,
			Selected: config.Tiers[i].Name == selected,
		}
	}
	return opts
}

// tourneyRows builds the roster builder: the config default roster by name
// and tier, or the submitted values echoing back through a failed post.
func tourneyRows(names, tiers []string) []tourneyRowView {
	def := config.DefaultRoster()
	rows := make([]tourneyRowView, len(def))
	for i, seat := range def {
		name, tier := seat.Name, seat.Tier
		if i < len(names) && names[i] != "" {
			name = names[i]
		}
		if i < len(tiers) && tiers[i] != "" {
			tier = tiers[i]
		}
		rows[i] = tourneyRowView{N: i, Name: name, Tiers: tourneyTierOptions(tier)}
	}
	return rows
}

// tourneySetupState holds the parsed form values across the validate-fail
// re-render, so a rejected post echoes what the operator typed.
type tourneySetupState struct {
	names  []string
	tiers  []string
	rating int
}

// handleSetup renders the Scenario 2 page: the runs section for every
// viewer, guest included, and the setup form for the admin session only.
func (p *TournamentPages) handleSetup(w http.ResponseWriter, r *http.Request) {
	me, err := resolveViewer(p.store, r)
	if err != nil {
		http.Error(w, "tournament read failed", http.StatusInternalServerError)
		return
	}
	lines, err := p.runLines(r)
	if err != nil {
		http.Error(w, "tournament list failed", http.StatusInternalServerError)
		return
	}
	p.renderSetup(w, http.StatusOK, me, &tourneySetupState{rating: config.TournamentStartRating}, "", lines)
}

// runLines shapes the runs section of the setup page.
func (p *TournamentPages) runLines(r *http.Request) ([]tourneyRunLineView, error) {
	runs, err := p.tourney.Runs(r.Context())
	if err != nil {
		return nil, err
	}
	lines := make([]tourneyRunLineView, len(runs))
	for i, run := range runs {
		names := make([]string, len(run.Seats))
		for j, seat := range run.Seats {
			names[j] = seat.Name
		}
		lines[i] = tourneyRunLineView{
			ID: run.ID, When: time.Unix(run.CreatedAt, 0).UTC().Format(historyTimeFormat),
			TC: tcLabel(run.TCIdx), BO: "bo" + strconv.Itoa(run.BOLen),
			State: runStateOf(run.TourneyRunInfo), Roster: strings.Join(names, ", "),
			Leader: run.Leader,
		}
	}
	return lines, nil
}

// renderSetup paints the setup page around the form state, the runs section,
// and an inline error.
func (p *TournamentPages) renderSetup(w http.ResponseWriter, status int, me *shellViewer,
	state *tourneySetupState, errMsg string, runs []tourneyRunLineView) {

	parallelOpts := make([]optionView, config.TournamentParallelMatches)
	for n := range parallelOpts {
		parallelOpts[n] = optionView{Value: n + 1, Label: strconv.Itoa(n + 1),
			Selected: n+1 == config.TournamentParallelMatches}
	}
	countOpts := make([]optionView, 0, config.TournamentMaxParticipants-1)
	for n := 2; n <= config.TournamentMaxParticipants; n++ {
		countOpts = append(countOpts, optionView{Value: n, Label: strconv.Itoa(n),
			Selected: n == config.TournamentMaxParticipants})
	}
	var names, tiers []string
	rating := config.TournamentStartRating
	if state != nil {
		names, tiers, rating = state.names, state.tiers, state.rating
	}
	renderShell(w, status, tourneyTmpl, "base", tourneySetupView{
		Me:        me,
		Admin:     isAdmin(me),
		TCOptions: shellTCOptions(), BOOptions: shellBOOptions(),
		ParallelOptions: parallelOpts, CountOptions: countOpts,
		Rows:        tourneyRows(names, tiers),
		StartRating: rating, MaxNameLen: config.TournamentNameMaxBytes,
		Error: errMsg, Runs: runs,
	})
}

// handleStart is the setup form POST: guests bounce like every acting route,
// a non-admin session gets the setup page back with the refusal inline, then
// every config law validated inline, the roster rows the count select
// governs, and the service start with the redirect onto the run's live page.
func (p *TournamentPages) handleStart(w http.ResponseWriter, r *http.Request) {
	me, err := resolveViewer(p.store, r)
	if err != nil {
		http.Error(w, "tournament read failed", http.StatusInternalServerError)
		return
	}
	if me == nil {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}
	if !isAdmin(me) {
		p.renderSetup(w, http.StatusForbidden, me, &tourneySetupState{rating: config.TournamentStartRating},
			"only the admin account starts tournaments", p.bestEffortRuns(r))
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, httpBodyLimitBytes)
	if err := r.ParseForm(); err != nil {
		p.renderSetup(w, http.StatusBadRequest, me, nil, "malformed form body", p.bestEffortRuns(r))
		return
	}
	state := &tourneySetupState{
		names: formStrings(r, "name", config.TournamentMaxParticipants),
		tiers: formStrings(r, "tier", config.TournamentMaxParticipants),
	}
	// A rejected post re-renders with the runs section best-effort: the
	// inline error is the answer, a listing hiccup must not mask it.
	bad := func(msg string) {
		state.rating = config.TournamentStartRating
		if v, err := strconv.Atoi(r.PostFormValue("rating")); err == nil {
			state.rating = v
		}
		p.renderSetup(w, http.StatusBadRequest, me, state, "invalid tournament setup: "+msg, p.bestEffortRuns(r))
	}
	tcIdx, err := strconv.Atoi(r.PostFormValue("tc"))
	if err != nil || tcIdx < 0 || tcIdx >= len(config.TimeControls) {
		bad("pick a time control from the list")
		return
	}
	boLen, err := strconv.Atoi(r.PostFormValue("bo"))
	if err != nil || !slices.Contains(config.SeriesLengths[:], boLen) {
		bad("pick a best of from the list")
		return
	}
	rating, err := strconv.Atoi(r.PostFormValue("rating"))
	if err != nil || rating > config.TournamentStartRatingAbsMax || rating < -config.TournamentStartRatingAbsMax {
		bad("start rating must sit within " + strconv.Itoa(config.TournamentStartRatingAbsMax) +
			" of zero")
		return
	}
	state.rating = rating
	parallel, err := strconv.Atoi(r.PostFormValue("parallel"))
	if err != nil || parallel < 1 || parallel > config.TournamentParallelMatches {
		bad("parallel must sit between 1 and " + strconv.Itoa(config.TournamentParallelMatches))
		return
	}
	count, err := strconv.Atoi(r.PostFormValue("count"))
	if err != nil || count < 2 || count > config.TournamentMaxParticipants {
		bad("participants must sit between 2 and " + strconv.Itoa(config.TournamentMaxParticipants))
		return
	}
	seats := make([]TourneySeat, count)
	seen := make(map[string]bool, count)
	maxCores := 0
	for i := range seats {
		name := state.names[i]
		if name == "" {
			bad("bot " + strconv.Itoa(i+1) + " needs a name")
			return
		}
		if len(name) > config.TournamentNameMaxBytes {
			bad("bot names stay within " + strconv.Itoa(config.TournamentNameMaxBytes) + " bytes")
			return
		}
		if seen[name] {
			bad("bot name " + name + " is used twice")
			return
		}
		seen[name] = true
		tier := tierByName(state.tiers[i])
		if tier == nil {
			bad("bot " + strconv.Itoa(i+1) + " needs a tier from the list")
			return
		}
		if tier.Cores > maxCores {
			maxCores = tier.Cores
		}
		seats[i] = TourneySeat{Slot: i, Name: name, Tier: tier.Name}
	}
	// The conductor's core budget law, mirrored client of the same config:
	// parallel rooms each book the roster's largest tier.
	if need := parallel * maxCores; need > config.MachineCores {
		bad(strconv.Itoa(parallel) + " parallel rooms at " + strconv.Itoa(maxCores) +
			" live-search cores each need " + strconv.Itoa(need) +
			" cores, the machine budget is " + strconv.Itoa(config.MachineCores))
		return
	}
	runID, err := p.tourney.StartRun(r.Context(), TourneySetup{
		Seats: seats, TCIdx: tcIdx, BOLen: boLen, StartRating: rating, Parallel: parallel,
	})
	if err != nil {
		// The machine-wide run gate is an answer, not an outage: the form
		// re-renders with the blocking run's id inline.
		var blocked *TourneyBlockedError
		if errors.As(err, &blocked) {
			p.renderSetup(w, http.StatusConflict, me, state, blocked.Error(), p.bestEffortRuns(r))
			return
		}
		http.Error(w, "tournament start failed", http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/tourney/run/"+strconv.FormatInt(runID, 10), http.StatusSeeOther)
}

// formStrings reads the numbered form fields name0..name{n-1} and
// tier0..tier{n-1} into index-aligned slices.
func formStrings(r *http.Request, prefix string, n int) []string {
	out := make([]string, n)
	for i := range out {
		out[i] = r.PostFormValue(prefix + strconv.Itoa(i))
	}
	return out
}

// bestEffortRuns shapes the runs section for a re-render, empty on a listing
// failure.
func (p *TournamentPages) bestEffortRuns(r *http.Request) []tourneyRunLineView {
	lines, err := p.runLines(r)
	if err != nil {
		return nil
	}
	return lines
}

// seriesLineView is one rendered pairing line, red-first seat first.
type seriesLineView struct {
	Red      string
	Blue     string
	Score    string
	Winner   string
	Finished bool
}

// standingRowView is one rendered leaderboard row.
type standingRowView struct {
	Rank                          int
	Name                          string
	Tier                          string
	Rating, Wins, Losses          int
	Draws, SeriesWon, GamesPlayed int
}

// tourneyBoardView is the polled fragment: the settled count, the pairing
// lines, and the leaderboard.
type tourneyBoardView struct {
	Done   int
	Total  int
	Series []seriesLineView
	Board  []standingRowView
	Live   []liveBoardView
}

// liveBoardView is one ongoing bot series' section: the linked room, the
// seats under the room naming law, the running score with the side to move,
// and the mini board of stones in play order.
type liveBoardView struct {
	RoomID    string
	Red       string
	Blue      string
	Score     string
	Turn      string
	MoveCount int
	Cells     []cellView
	BoardSize int
}

// liveBoardsOf shapes the service's live reads; empty while the run's drive
// is not live, so only the ongoing run's page shows boards.
func liveBoardsOf(running bool, boards []TourneyLiveBoard) []liveBoardView {
	if !running || len(boards) == 0 {
		return nil
	}
	out := make([]liveBoardView, len(boards))
	for i, b := range boards {
		out[i] = liveBoardView{
			RoomID: b.RoomID, Red: b.RedName, Blue: b.BlueName,
			Turn: b.Turn, MoveCount: len(b.Moves),
			Score: strconv.Itoa(b.RedWins) + "-" + strconv.Itoa(b.BlueWins),
			Cells: boardCells(b.Moves, nil), BoardSize: config.BoardSize,
		}
	}
	return out
}

// tourneyRunHeaderView is the run page's summary line.
type tourneyRunHeaderView struct {
	ID          int64
	When        string
	TC          string
	BO          string
	StartRating int
	State       string
	Failure     string
}

// tourneyRunView is the run page's whole render state: the board plus, on
// a stalled run, the close form and its inline refusal.
type tourneyRunView struct {
	Me         *shellViewer
	Header     tourneyRunHeaderView
	RunID      int64
	PollMs     int64
	Board      tourneyBoardView
	CanClose   bool
	CloseError string
}

// handleRun renders one run's live page: public like the rooms grid, the
// poll wiring around the board fragment the run refreshes in place.
func (p *TournamentPages) handleRun(w http.ResponseWriter, r *http.Request) {
	runID, ok := tourneyRunID(w, r)
	if !ok {
		return
	}
	snap, ok := p.snapshot(w, r, runID)
	if !ok {
		return
	}
	me, err := resolveViewer(p.store, r)
	if err != nil {
		http.Error(w, "tournament read failed", http.StatusInternalServerError)
		return
	}
	p.renderRun(w, http.StatusOK, me, runID, snap, "")
}

// renderRun paints the run page around the board fragment, optionally with
// the close form's inline refusal.
func (p *TournamentPages) renderRun(w http.ResponseWriter, status int, me *shellViewer,
	runID int64, snap TourneySnapshot, closeErr string) {

	header := runHeaderOf(snap)
	renderShell(w, status, tourneyRunTmpl, "base", tourneyRunView{
		Me: me, Header: header, RunID: runID,
		PollMs: int64(config.PagePollMs),
		Board:  boardViewOf(snap, liveBoardsOf(snap.Run.Running, p.tourney.LiveBoards())),
		// A stalled run (no live drive) and a failed drive both leave an
		// ongoing row holding the machine-wide run gate: the close form is
		// the admin's only UI escape from either.
		CanClose:   (header.State == runStateStalled || header.State == "failed") && isAdmin(me),
		CloseError: closeErr,
	})
}

// handleCloseRun is the stalled-run close form POST: guests bounce like
// every acting route and only the admin session closes. The close resolves
// a run row no live drive owns; a refusal re-renders the run page with the
// reason inline, an unknown run stays the 404 page.
func (p *TournamentPages) handleCloseRun(w http.ResponseWriter, r *http.Request) {
	runID, ok := tourneyRunID(w, r)
	if !ok {
		return
	}
	me, err := resolveViewer(p.store, r)
	if err != nil {
		http.Error(w, "tournament read failed", http.StatusInternalServerError)
		return
	}
	if me == nil {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}
	if !isAdmin(me) {
		snap, ok := p.snapshot(w, r, runID)
		if !ok {
			return
		}
		p.renderRun(w, http.StatusForbidden, me, runID, snap, "only the admin account closes runs")
		return
	}
	if err := p.tourney.CloseStalledRun(r.Context(), runID); err != nil {
		if errors.Is(err, ErrNotFound) {
			writePageNotFound(w)
			return
		}
		snap, ok := p.snapshot(w, r, runID)
		if !ok {
			return
		}
		p.renderRun(w, http.StatusConflict, me, runID, snap, err.Error())
		return
	}
	http.Redirect(w, r, "/tourney/run/"+strconv.FormatInt(runID, 10), http.StatusSeeOther)
}

// handleBoard serves the polled fragment of one run's page.
func (p *TournamentPages) handleBoard(w http.ResponseWriter, r *http.Request) {
	runID, ok := tourneyRunID(w, r)
	if !ok {
		return
	}
	snap, ok := p.snapshot(w, r, runID)
	if !ok {
		return
	}
	renderShell(w, http.StatusOK, tourneyBoardTmpl, "tourboard",
		boardViewOf(snap, liveBoardsOf(snap.Run.Running, p.tourney.LiveBoards())))
}

// tourneyRunID parses the path id; a garbage or non-positive id is the 404
// page.
func tourneyRunID(w http.ResponseWriter, r *http.Request) (int64, bool) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || id <= 0 {
		writePageNotFound(w)
		return 0, false
	}
	return id, true
}

// snapshot reads one run's render state, mapping the not-found sentinel onto
// the 404 page and everything else onto the 500 line.
func (p *TournamentPages) snapshot(w http.ResponseWriter, r *http.Request, runID int64) (TourneySnapshot, bool) {
	snap, err := p.tourney.RunSnapshot(r.Context(), runID)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			writePageNotFound(w)
			return TourneySnapshot{}, false
		}
		http.Error(w, "tournament read failed", http.StatusInternalServerError)
		return TourneySnapshot{}, false
	}
	return snap, true
}

// runStateStalled names the display state an ongoing row no process drives
// takes; the run page hands that state its close form.
const runStateStalled = "stalled"

// runStateOf names the run's display state: a terminal drive failure wins,
// then the run row's finish, then the manager's liveness; an ongoing row no
// process drives is stalled (a previous lifetime's abort left it open).
func runStateOf(run TourneyRunInfo) string {
	switch {
	case run.Failure != "":
		return "failed"
	case run.Finished:
		return "finished"
	case run.Running:
		return "running"
	}
	return runStateStalled
}

// runHeaderOf shapes one run's summary line.
func runHeaderOf(snap TourneySnapshot) tourneyRunHeaderView {
	run := snap.Run
	return tourneyRunHeaderView{
		ID: run.ID, When: time.Unix(run.CreatedAt, 0).UTC().Format(historyTimeFormat),
		TC: tcLabel(run.TCIdx), BO: "bo" + strconv.Itoa(run.BOLen), StartRating: run.StartRating,
		State: runStateOf(run), Failure: run.Failure,
	}
}

// boardViewOf shapes the polled fragment: names by slot with the raw slot as
// the unresolvable fallback, red-first lines in pairing order, ranks over
// the service's leaderboard order, and the live spectating section.
func boardViewOf(snap TourneySnapshot, live []liveBoardView) tourneyBoardView {
	name := func(slot int) string {
		for _, seat := range snap.Seats {
			if seat.Slot == slot {
				return seat.Name
			}
		}
		return "slot " + strconv.Itoa(slot)
	}
	series := make([]seriesLineView, len(snap.Series))
	done := 0
	for i, line := range snap.Series {
		v := seriesLineView{
			Red: name(line.RedFirstSlot), Blue: name(line.BlueFirstSlot),
			Score:    strconv.Itoa(line.RedFirstWins) + "-" + strconv.Itoa(line.BlueFirstWins),
			Finished: line.Finished,
		}
		if line.Finished {
			done++
			switch line.WinnerSlot {
			case nil:
				v.Winner = "drawn"
			default:
				v.Winner = name(*line.WinnerSlot) + " wins"
			}
		}
		series[i] = v
	}
	board := make([]standingRowView, len(snap.Board))
	for i, st := range snap.Board {
		board[i] = standingRowView{
			Rank: i + 1, Name: name(st.Slot), Tier: tierOfSeat(snap.Seats, st.Slot),
			Rating: st.Rating, Wins: st.Wins, Losses: st.Losses, Draws: st.Draws,
			SeriesWon: st.SeriesWon, GamesPlayed: st.GamesPlayed,
		}
	}
	return tourneyBoardView{Done: done, Total: len(snap.Series), Series: series, Board: board, Live: live}
}

// tierOfSeat resolves one slot's tier for the leaderboard rows.
func tierOfSeat(seats []TourneySeat, slot int) string {
	for _, seat := range seats {
		if seat.Slot == slot {
			return seat.Tier
		}
	}
	return ""
}
