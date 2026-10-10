package server

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

func tourneyPage(page string) *template.Template {
	return template.Must(template.ParseFS(shellTmplFS,
		"web/templates/base.tmpl", "web/templates/rooms.tmpl",
		"web/templates/tourney_board.tmpl", "web/templates/board.html",
		"web/templates/"+page))
}

var (
	tourneyTmpl      = tourneyPage("tourney.tmpl")
	tourneyRunTmpl   = tourneyPage("tourney_run.tmpl")
	tourneyBoardTmpl = template.Must(template.ParseFS(shellTmplFS,
		"web/templates/tourney_board.tmpl", "web/templates/board.html"))
)

type TourneySeat struct {
	Slot int
	Name string
	Tier string
}

type TourneySetup struct {
	Seats       []TourneySeat
	TCIdx       int
	BOLen       int
	StartRating int
	Parallel    int
}

type TourneySeriesLine struct {
	PairingSlot   int
	RedFirstSlot  int
	BlueFirstSlot int
	RedFirstWins  int
	BlueFirstWins int
	WinnerSlot    *int
	Finished      bool
}

type TourneyStanding struct {
	Slot        int
	Rating      int
	Wins        int
	Losses      int
	Draws       int
	SeriesWon   int
	GamesPlayed int
}

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

type TourneySnapshot struct {
	Run    TourneyRunInfo
	Seats  []TourneySeat
	Series []TourneySeriesLine
	Board  []TourneyStanding
}

type TourneyRunSummary struct {
	TourneyRunInfo
	Seats  []TourneySeat
	Leader string
}

type TourneyLiveBoard struct {
	RoomID   string
	RedName  string
	BlueName string
	RedWins  int
	BlueWins int
	Turn     string
	Moves    []string
}

type TourneyBanner struct {
	RunID int64
	Done  int
	Total int
}

type TourneyBlockedError struct{ RunID int64 }

func (e *TourneyBlockedError) Error() string {
	return "run " + strconv.FormatInt(e.RunID, 10) +
		" is still in progress, close it before starting another"
}

type TourneyService interface {
	StartRun(ctx context.Context, setup TourneySetup) (int64, error)
	RunSnapshot(ctx context.Context, runID int64) (TourneySnapshot, error)
	Runs(ctx context.Context) ([]TourneyRunSummary, error)
	OngoingRun(ctx context.Context) (TourneyBanner, bool, error)
	LiveBoards() []TourneyLiveBoard
	CloseStalledRun(ctx context.Context, runID int64) error
}

type TournamentPages struct {
	store   *Store
	tourney TourneyService
}

func NewTournamentPages(store *Store, tourney TourneyService) *TournamentPages {
	return &TournamentPages{store: store, tourney: tourney}
}

func (p *TournamentPages) Mount(mux *http.ServeMux) {
	mux.HandleFunc("GET /tourney", p.handleSetup)
	mux.HandleFunc("POST /tourney", p.handleStart)
	mux.HandleFunc("GET /tourney/run/{id}", p.handleRun)
	mux.HandleFunc("GET /tourney/run/{id}/board", p.handleBoard)
	mux.HandleFunc("POST /tourney/run/{id}/close", p.handleCloseRun)
}

func isAdmin(me *shellViewer) bool {
	return me != nil && me.Username == config.AdminName
}

type tourneyRowView struct {
	N     int
	Name  string
	Tiers []botOptionView
}

type tourneyRunLineView struct {
	ID     int64
	When   string
	TC     string
	BO     string
	State  string
	Roster string
	Leader string
}

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

type tourneySetupState struct {
	names  []string
	tiers  []string
	rating int
}

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
	worstRoom := 0
	var picked []*config.Tier
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
		worstRoom = max(worstRoom, tier.Cores)
		for _, other := range picked {
			worstRoom = max(worstRoom, config.RoomCores(*tier, *other))
		}
		picked = append(picked, tier)
		seats[i] = TourneySeat{Slot: i, Name: name, Tier: tier.Name}
	}
	if need := parallel * worstRoom; need > config.MachineCores {
		bad(strconv.Itoa(parallel) + " parallel rooms at " + strconv.Itoa(worstRoom) +
			" live-search cores each need " + strconv.Itoa(need) +
			" cores, the machine budget is " + strconv.Itoa(config.MachineCores))
		return
	}
	runID, err := p.tourney.StartRun(r.Context(), TourneySetup{
		Seats: seats, TCIdx: tcIdx, BOLen: boLen, StartRating: rating, Parallel: parallel,
	})
	if err != nil {
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

func formStrings(r *http.Request, prefix string, n int) []string {
	out := make([]string, n)
	for i := range out {
		out[i] = r.PostFormValue(prefix + strconv.Itoa(i))
	}
	return out
}

func (p *TournamentPages) bestEffortRuns(r *http.Request) []tourneyRunLineView {
	lines, err := p.runLines(r)
	if err != nil {
		return nil
	}
	return lines
}

type seriesLineView struct {
	Red      string
	Blue     string
	Score    string
	Winner   string
	Finished bool
}

type standingRowView struct {
	Rank                          int
	Name                          string
	Tier                          string
	Rating, Wins, Losses          int
	Draws, SeriesWon, GamesPlayed int
}

type tourneyBoardView struct {
	Done   int
	Total  int
	Series []seriesLineView
	Board  []standingRowView
	Live   []liveBoardView
}

type liveBoardView struct {
	RoomID    string
	Red       string
	Blue      string
	Score     string
	Turn      string
	MoveCount int
	boardData
}

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
			boardData: boardData{
				Cells: boardCells(b.Moves, nil), BoardSize: config.BoardSize, Mini: true,
			},
		}
	}
	return out
}

type tourneyRunHeaderView struct {
	ID          int64
	When        string
	TC          string
	BO          string
	StartRating int
	State       string
	Failure     string
}

type tourneyRunView struct {
	Me         *shellViewer
	Header     tourneyRunHeaderView
	RunID      int64
	PollMs     int64
	Board      tourneyBoardView
	CanClose   bool
	CloseError string
}

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

func (p *TournamentPages) renderRun(w http.ResponseWriter, status int, me *shellViewer,
	runID int64, snap TourneySnapshot, closeErr string) {

	header := runHeaderOf(snap)
	renderShell(w, status, tourneyRunTmpl, "base", tourneyRunView{
		Me: me, Header: header, RunID: runID,
		PollMs:     int64(config.PagePollMs),
		Board:      boardViewOf(snap, liveBoardsOf(snap.Run.Running, p.tourney.LiveBoards())),
		CanClose:   (header.State == runStateStalled || header.State == "failed") && isAdmin(me),
		CloseError: closeErr,
	})
}

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

func tourneyRunID(w http.ResponseWriter, r *http.Request) (int64, bool) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || id <= 0 {
		writePageNotFound(w)
		return 0, false
	}
	return id, true
}

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

const runStateStalled = "stalled"

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

func runHeaderOf(snap TourneySnapshot) tourneyRunHeaderView {
	run := snap.Run
	return tourneyRunHeaderView{
		ID: run.ID, When: time.Unix(run.CreatedAt, 0).UTC().Format(historyTimeFormat),
		TC: tcLabel(run.TCIdx), BO: "bo" + strconv.Itoa(run.BOLen), StartRating: run.StartRating,
		State: runStateOf(run), Failure: run.Failure,
	}
}

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

func tierOfSeat(seats []TourneySeat, slot int) string {
	for _, seat := range seats {
		if seat.Slot == slot {
			return seat.Tier
		}
	}
	return ""
}
