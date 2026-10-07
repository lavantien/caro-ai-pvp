package server

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/lavantien/caro-ai-pvp/internal/clock"
	"github.com/lavantien/caro-ai-pvp/internal/config"
	"github.com/lavantien/caro-ai-pvp/internal/rules"
)

// Room lifecycle of the M6a backend: one RoomManager keyed by opaque
// crypto/rand ids owns every live room, seats opponents, and feeds the rooms
// grid. Rooms are runtime state only per the schema note: the database
// records what happened (series, games, rating events), never what is
// currently happening, so a process restart drops every room cleanly.

var (
	// ErrRoomNotFound maps every manager access of a missing or already
	// retired room id.
	ErrRoomNotFound = errors.New("server: room not found")
	// ErrRoomFull rejects joining a room whose guest seat is taken, the bot
	// seat included.
	ErrRoomFull = errors.New("server: room already has an opponent")
	// ErrRoomClosed rejects every use of a retired room: finished series,
	// forfeit swept, or manager shutdown.
	ErrRoomClosed = errors.New("server: room closed")
	// ErrNotYourTurn rejects a move by a participant who does not hold the
	// side to move, including any human move for a bot seat.
	ErrNotYourTurn = errors.New("server: not this side's turn")
	// ErrIllegalMove rejects a cell the rules package refuses: out of
	// region, occupied, board full, or violating the opening distance.
	ErrIllegalMove = errors.New("server: illegal move")
	// ErrBadOwner rejects room creation for a non-positive user id: SQLite
	// ids start at 1 and 0 doubles as "guest seat still open".
	ErrBadOwner = errors.New("server: room owner must be a positive user id")
	// ErrBadTier rejects a bot-vs-bot create without both tiers seated.
	ErrBadTier = errors.New("server: bot-vs-bot needs both tiers")
	// ErrUnknownTier rejects a tier value outside the config tier table.
	ErrUnknownTier = errors.New("server: unknown bot tier")
	// ErrMachineBusy refuses creating a bot room whose cores do not fit the
	// machine-wide budget the live bot rooms hold (admission.go's ledger).
	ErrMachineBusy = errors.New("server: machine core budget full")
)

// roomIDBytes sizes the crypto/rand room id: 128 bits hex-encoded, long
// enough that collisions between live rooms are ignored rather than
// resolved.
const roomIDBytes = 16

// The synthetic seat ids of Scenario 2's bot-vs-bot tournament matchups,
// where both seats are bots and the room persists nothing (the tournament
// tables and txt logs are the record), so the ids never reach a foreign key.
// Human-vs-bot rooms seat the tier's reserved users row instead, resolved
// through Store.BotAccountID. The Series machine demands two distinct int64
// sides and SQLite user ids are positive, so the negative constants can
// never collide with a real account.
const (
	botGuestUserID int64 = -2
	botHostUserID  int64 = -3
)

// seat is one side of a room: a real user id, or a bot tier when bot is set.
// name carries a bot seat's instance identity (the tournament's roster name:
// two easy instances seat as easy-a and easy-b); empty falls back to the tier
// name, which is unique per seat only while the two tiers differ.
type seat struct {
	userID int64
	bot    *config.Tier
	name   string
}

// RoomManager owns the live rooms and the rooms grid. All manager methods
// take the manager lock before any room lock; room code never nests the
// manager lock, so the order manager then room holds everywhere.
type RoomManager struct {
	mu    sync.Mutex
	rooms map[string]*Room
	hub   *Hub
	store *Store
	wq    *WriteQueue
	// coresUsed is the machine-wide core ledger's live total (admission.go),
	// guarded by mu like the rooms map.
	coresUsed int
	// makeSearcher is the engine factory every new room starts from. A
	// manager-level seam rather than a per-room one because a bot-vs-bot
	// game 1 starts inside CreateBotVsBot, before any per-room override
	// could land; set before the first room exists.
	makeSearcher func(config.Tier) searcher
}

func NewRoomManager(hub *Hub, store *Store, wq *WriteQueue) *RoomManager {
	return &RoomManager{
		rooms: make(map[string]*Room), hub: hub, store: store, wq: wq,
		makeSearcher: newBotSearcher,
	}
}

// Create opens a room owned by ownerUserID under the given settings, with a
// bot guest when vsBot is non-nil and an open guest seat otherwise. The
// settings validate through NewSeries, the same authority that will build
// the series, so the room grid can never advertise settings a series would
// reject: for PvP the guest is still unknown, so a throwaway construction
// checks the time control and length. A bot room persists its pairing row
// before going live, mirroring join, so every match is recorded: games and
// per-move stats land at each completion, ratings never move. A bot room
// also books its tier's cores on the machine ledger (admission.go) before
// that row persists, refusing with ErrMachineBusy when the budget is full.
func (rm *RoomManager) Create(ownerUserID int64, tcIdx, boLen int, vsBot *config.Tier) (*Room, error) {
	if ownerUserID <= 0 {
		return nil, ErrBadOwner
	}
	guest := seat{}
	if vsBot != nil {
		botID, err := rm.store.BotAccountID(*vsBot)
		if err != nil {
			return nil, err
		}
		guest = seat{userID: botID, bot: vsBot}
	}
	series, err := NewSeries(ownerUserID, guest.userID, tcIdx, boLen)
	if err != nil {
		return nil, err
	}
	r := &Room{
		id: newRoomID(), hub: rm.hub, store: rm.store, wq: rm.wq, manager: rm,
		tcIdx: tcIdx, boLen: boLen, createdAt: time.Now(),
		host: seat{userID: ownerUserID}, guest: guest,
		makeSearcher: rm.makeSearcher,
		wake:         make(chan struct{}, 1), quit: make(chan struct{}),
	}
	// The ledger hold spans the booking to the publish: any create step
	// failing in between returns the cores here, and from the publish the
	// room's own retire owns the release.
	published := false
	defer func() {
		if !published {
			rm.releaseRoomCores(r)
		}
	}()
	if vsBot != nil {
		if !rm.bookRoomCores(r, vsBot.Cores) {
			return nil, ErrMachineBusy
		}
		var row SeriesRow
		err = r.wq.Send(func(ctx context.Context) error {
			var perr error
			row, perr = r.store.CreateSeries(ctx, tcIdx, boLen, series.HostUserID(), series.GuestUserID())
			return perr
		})
		if err != nil {
			return nil, err
		}
		r.seriesID = row.ID
		// The bot's handshake is immediate, so only the host ready gates
		// match 1. Ready cannot fail on a fresh series.
		if err := series.Ready(guest.userID); err != nil {
			return nil, err
		}
		r.series = series
		r.startBotWorker()
	}
	rm.mu.Lock()
	rm.rooms[r.id] = r
	published = true
	rm.mu.Unlock()
	return r, nil
}

// CreateBotVsBot opens the Scenario 2 tournament surface: a live room whose
// host seat is a bot too, game 1 running the moment the room publishes. Bot
// handshakes are instant, so both seats ready here and the second one walks
// Room.Ready's own Created-to-Ready transition, the exact path a human room
// drives. The settings validate through NewSeries like Create, the room
// persists nothing (seriesID stays 0), and no player rating is touched.
// hostName and guestName carry the seats' instance identities (the roster's
// easy-a/easy-b); empty names fall back to the tier name. The room books the
// larger tier's cores on the machine ledger (admission.go) before it
// publishes, refusing with ErrMachineBusy when the budget is full.
func (rm *RoomManager) CreateBotVsBot(hostTier *config.Tier, hostName string, guestTier *config.Tier, guestName string, tcIdx, boLen int) (*Room, error) {
	if hostTier == nil || guestTier == nil {
		return nil, ErrBadTier
	}
	series, err := NewBotSeries(botHostUserID, botGuestUserID, tcIdx, boLen)
	if err != nil {
		return nil, err
	}
	r := &Room{
		id: newRoomID(), hub: rm.hub, store: rm.store, wq: rm.wq, manager: rm,
		tcIdx: tcIdx, boLen: boLen, createdAt: time.Now(),
		host:         seat{userID: botHostUserID, bot: hostTier, name: hostName},
		guest:        seat{userID: botGuestUserID, bot: guestTier, name: guestName},
		makeSearcher: rm.makeSearcher,
		wake:         make(chan struct{}, 1), quit: make(chan struct{}),
	}
	// The ledger hold spans the booking to the publish like Create's: turns
	// alternate, so the room's worst case is the max of the two tiers.
	published := false
	defer func() {
		if !published {
			rm.releaseRoomCores(r)
		}
	}()
	if !rm.bookRoomCores(r, max(hostTier.Cores, guestTier.Cores)) {
		return nil, ErrMachineBusy
	}
	r.mu.Lock()
	r.series = series
	r.mu.Unlock()
	// Ready cannot fail on a fresh series: both ids seat here and no
	// terminal state exists yet.
	if err := r.Ready(botHostUserID); err != nil {
		return nil, err
	}
	if err := r.Ready(botGuestUserID); err != nil {
		return nil, err
	}
	// Published before the worker starts, the reverse of Create's order:
	// game 1 is already live, so a scripted sweep could retire within
	// microseconds and a retirement ahead of registration would resurrect
	// a dead room on the grid. Create is safe either way only because its
	// game cannot start until the human host readies.
	rm.mu.Lock()
	rm.rooms[r.id] = r
	published = true
	rm.mu.Unlock()
	r.startBotWorker()
	return r, nil
}

// Join seats userID as the opponent of roomID. The guest seat is the only
// open one: bot rooms and already joined rooms refuse with ErrRoomFull. The
// pairing row persists synchronously through the write queue before the join
// lands, so a finished game can never reference a missing series.
func (rm *RoomManager) Join(roomID string, userID int64) error {
	r, err := rm.Get(roomID)
	if err != nil {
		return err
	}
	return r.join(userID)
}

func (r *Room) join(userID int64) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.over {
		return ErrRoomClosed
	}
	if userID == r.host.userID {
		return ErrSamePlayer
	}
	if r.guest.userID != 0 {
		return ErrRoomFull
	}
	series, err := NewSeries(r.host.userID, userID, r.tcIdx, r.boLen)
	if err != nil {
		return err
	}
	var row SeriesRow
	err = r.wq.Send(func(ctx context.Context) error {
		var perr error
		row, perr = r.store.CreateSeries(ctx, r.tcIdx, r.boLen, series.HostUserID(), series.GuestUserID())
		return perr
	})
	if err != nil {
		return err
	}
	r.series = series
	r.seriesID = row.ID
	r.guest = seat{userID: userID}
	return nil
}

// Get returns the live room for id.
func (rm *RoomManager) Get(roomID string) (*Room, error) {
	rm.mu.Lock()
	defer rm.mu.Unlock()
	r, ok := rm.rooms[roomID]
	if !ok {
		return nil, ErrRoomNotFound
	}
	return r, nil
}

// RoomInfo is one rooms-grid line: settings, seating, series progress.
type RoomInfo struct {
	ID          string
	HostUserID  int64
	GuestUserID int64
	TCIdx       int
	BOLen       int
	State       SeriesState
	HostBotTier string
	VsBotTier   string
	// HostBotName and GuestBotName are the bot seats' final display strings
	// under the room's naming law (instance name or tier, stamped with the
	// room id, collisions suffixed): a bot-vs-bot room never renders one
	// instance on both sides. Empty on a human seat.
	HostBotName  string
	GuestBotName string
	HostWins     int
	GuestWins    int
	CreatedAt    time.Time
}

// List snapshots the open rooms in creation order. Retired rooms (finished
// series, forfeited, closed) left the map at their retirement, so the grid
// only ever shows matches a spectator can still watch.
func (rm *RoomManager) List() []RoomInfo {
	rm.mu.Lock()
	rooms := make([]*Room, 0, len(rm.rooms))
	for _, r := range rm.rooms {
		rooms = append(rooms, r)
	}
	rm.mu.Unlock()
	infos := make([]RoomInfo, 0, len(rooms))
	for _, r := range rooms {
		if info, ok := r.Info(); ok {
			infos = append(infos, info)
		}
	}
	slices.SortFunc(infos, func(a, b RoomInfo) int {
		if c := a.CreatedAt.Compare(b.CreatedAt); c != 0 {
			return c
		}
		return strings.Compare(a.ID, b.ID)
	})
	return infos
}

// LiveBotBoard is one live bot-vs-bot room's spectating read for the
// tournament run page: the room link, both seat names under the room's own
// naming law, the stones on the board in play order, the side to move, and
// the running series score. Human rooms never appear: only the tournament
// surface seats bots on both sides.
type LiveBotBoard struct {
	RoomID    string
	HostName  string
	GuestName string
	CreatedAt time.Time
	Moves     []string
	Turn      string
	RedIsHost bool
	HostWins  int
	GuestWins int
}

// liveBotBoard is one room's spectating read under a single room lock: the
// seating, the series score, and the live game's stones and colors read
// atomically, so a game completing during the read can never pair the new
// board with the old score. False for every room that is not a live
// bot-vs-bot series with a game under way (between games or in the
// retirement race window the room drops off the run page's list).
func (r *Room) liveBotBoard() (LiveBotBoard, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.over || r.host.bot == nil || r.guest.bot == nil || r.series == nil || r.board == nil {
		return LiveBotBoard{}, false
	}
	hostWins, guestWins := r.series.Score()
	hostName, guestName := r.botSeatDisplaysLocked()
	lb := LiveBotBoard{
		RoomID:    r.id,
		HostName:  hostName,
		GuestName: guestName,
		CreatedAt: r.createdAt,
		Turn:      colorName(r.board.Side),
		RedIsHost: r.series.RedUserID() == r.host.userID,
		HostWins:  hostWins, GuestWins: guestWins,
		Moves: make([]string, 0, len(r.moves)),
	}
	for _, m := range r.moves {
		lb.Moves = append(lb.Moves, cellName(rules.Cell(m)))
	}
	return lb, true
}

// BotBoards snapshots every live bot-vs-bot room in creation order, the
// run page's live-board section, sorted by CreatedAt then id like List so
// the polled cards never reshuffle.
func (rm *RoomManager) BotBoards() []LiveBotBoard {
	rm.mu.Lock()
	rooms := make([]*Room, 0, len(rm.rooms))
	for _, r := range rm.rooms {
		rooms = append(rooms, r)
	}
	rm.mu.Unlock()
	out := make([]LiveBotBoard, 0, 2)
	for _, r := range rooms {
		if lb, ok := r.liveBotBoard(); ok {
			out = append(out, lb)
		}
	}
	slices.SortFunc(out, func(a, b LiveBotBoard) int {
		if c := a.CreatedAt.Compare(b.CreatedAt); c != 0 {
			return c
		}
		return strings.Compare(a.RoomID, b.RoomID)
	})
	return out
}

// Shutdown retires and joins every room. Idempotent; the manager keeps no
// rooms afterwards.
func (rm *RoomManager) Shutdown() {
	rm.mu.Lock()
	rooms := make([]*Room, 0, len(rm.rooms))
	for _, r := range rm.rooms {
		rooms = append(rooms, r)
	}
	rm.rooms = make(map[string]*Room)
	rm.mu.Unlock()
	for _, r := range rooms {
		r.Close()
	}
}

// retireRoom drops a finished room from the grid. Called only with no room
// lock held, after retire, so the manager lock and the room lock never
// nest.
func (rm *RoomManager) retireRoom(r *Room) {
	rm.mu.Lock()
	delete(rm.rooms, r.id)
	rm.mu.Unlock()
}

// Room is one live best-of series plus its current game. The mutex guards
// every field below it; the match driver in match.go runs bot searches
// outside the lock on a private board copy and revalidates before applying.
type Room struct {
	id        string
	hub       *Hub
	store     *Store
	wq        *WriteQueue
	manager   *RoomManager
	tcIdx     int
	boLen     int
	createdAt time.Time
	// bookedCores is the admission ledger hold the room carries from its
	// create to its retire: written once at booking under the manager lock,
	// released by the once-guarded retire. Zero on a room with no bot seat.
	bookedCores int

	mu       sync.Mutex
	host     seat
	guest    seat
	series   *Series
	seriesID int64
	over     bool

	// Live game state, all under mu. board is nil until the handshake
	// completes and between-terminal games never happens: the completion
	// path resets in the same critical section. lastMoves is the finished
	// game's authoritative list, captured at each completion and held until
	// the next game completes (LastGameMoves). mlines is the live game's
	// accumulated bot log, one rendered M-line per bot move in publish
	// order, drained into the completion unit at each game end and reset
	// with the moves buffer.
	board     *rules.Board
	moves     []rules.Move
	lastMoves []rules.Move
	mlines    []GameStat
	clock     [2]*clock.GameClock
	turnStart time.Time
	engines   [2]searcher
	legalBuf  [config.BoardCells]rules.Move
	lastM     mLineRecord

	// budgetCap is a test hook capping the granted bot search budget so
	// time-boxed tests run the real engine at a few milliseconds per move;
	// zero means the clock law alone. makeSearcher is the engine factory
	// seam, overridable per room for deterministic bot tests. searchEntry,
	// when set, runs between the worker releasing the room lock and entering
	// Search, the window a concurrent retirement races.
	budgetCap    time.Duration
	makeSearcher func(config.Tier) searcher
	searchEntry  func()

	wake      chan struct{}
	quit      chan struct{}
	closeOnce sync.Once
	wg        sync.WaitGroup
}

// ID is the opaque room key, also the room's hub subscription key.
func (r *Room) ID() string { return r.id }

// SeriesID is the persisted pairing row id, 0 for the tournament bot-vs-bot
// rooms which persist nothing by design.
func (r *Room) SeriesID() int64 {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.seriesID
}

// Subscribe registers an observer's stream on the shared hub under this
// room's key: move events, bot M-lines, game and series ends.
func (r *Room) Subscribe() (*Subscription, error) {
	return r.hub.Subscribe(r.id)
}

// LastGameMoves returns the authoritative move list of the room's most
// recently finished game: the stones the room itself applied, in play
// order, copied under the lock. It stays readable into the next game (the
// tournament conductor reconciles its delivered stream against it at the
// game-end event) and is nil before any completion. A consumer that trails
// a full game behind reads the newer game's list; its reconciliation then
// fails the run loudly, never persists a stale record.
func (r *Room) LastGameMoves() []rules.Move {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]rules.Move(nil), r.lastMoves...)
}

// Info snapshots the grid line. ok is false once the room retired.
func (r *Room) Info() (RoomInfo, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.over {
		return RoomInfo{}, false
	}
	info := RoomInfo{
		ID: r.id, HostUserID: r.host.userID, GuestUserID: r.guest.userID,
		TCIdx: r.tcIdx, BOLen: r.boLen, CreatedAt: r.createdAt,
		HostBotTier: botTierName(r.host.bot), VsBotTier: botTierName(r.guest.bot),
	}
	info.HostBotName, info.GuestBotName = r.botSeatDisplaysLocked()
	if r.series == nil {
		info.State = SeriesCreated
		return info, true
	}
	info.State = r.series.State()
	info.HostWins, info.GuestWins = r.series.Score()
	return info, true
}

func botTierName(t *config.Tier) string {
	if t == nil {
		return ""
	}
	return t.Name
}

// botSeatDisplaysLocked is the one law of a room's bot seat names: each seat
// renders its instance identity (the roster name when the creator gave one,
// else the tier name) stamped with the room id, and when both seats would
// render the same string (an unnamed same-tier pairing) the guest takes a -2
// suffix, so no surface can ever show a bot playing itself. A human seat
// renders empty; callers fall back to the user's own name. Callers hold r.mu.
func (r *Room) botSeatDisplaysLocked() (host, guest string) {
	if r.host.bot != nil {
		host = botDisplayName(r.seatInstanceName(&r.host), r.id)
	}
	if r.guest.bot != nil {
		guest = botDisplayName(r.seatInstanceName(&r.guest), r.id)
	}
	if host != "" && host == guest {
		guest += "-2"
	}
	return host, guest
}

// seatInstanceName is a bot seat's identity: the given instance name, else
// the tier name.
func (r *Room) seatInstanceName(s *seat) string {
	if s.name != "" {
		return s.name
	}
	return s.bot.Name
}

// Close retires the room and joins its bot worker, so no goroutine and no
// engine instance outlives it. Idempotent. Must not be called from the
// room's own bot worker: the worker exits by itself once quit closes.
func (r *Room) Close() {
	r.retire()
	r.wg.Wait()
}

// retire ends the room for good: over flips under the lock so in-flight
// callers fail fast, and quit closes so the worker and any outstanding wake
// tokens drain. Retire never touches the engines: the engine contract
// panics on Search after Close, so an engine may only be closed by the bot
// worker goroutine that calls Search, and the worker releases them on its
// way out once quit closes. Idempotent, safe from any goroutine, and never
// called while holding r.mu.
func (r *Room) retire() {
	r.closeOnce.Do(func() {
		close(r.quit)
		r.mu.Lock()
		r.over = true
		r.mu.Unlock()
		// The ledger release rides the once-guarded retire: every terminal
		// path funnels here, while the manager key survives a plain Close by
		// the pinned mid-retirement law, so releasing at the map delete alone
		// would strand the booking of a closed-but-keyed room.
		r.manager.releaseRoomCores(r)
	})
}

// finishLifecycle runs after a terminal transition (series finished or
// forfeit swept) with no lock held: retire the room, then drop it from the
// grid. The worker exits on quit by itself, so this is safe on the worker
// goroutine too.
func (r *Room) finishLifecycle() {
	r.retire()
	r.manager.retireRoom(r)
}

func newRoomID() string {
	var b [roomIDBytes]byte
	if _, err := rand.Read(b[:]); err != nil {
		// crypto/rand only fails on a broken entropy source; unforgeable
		// room ids are not worth surviving that with a weaker fallback.
		panic("server: crypto/rand room id: " + err.Error())
	}
	return hex.EncodeToString(b[:])
}
