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
)

// roomIDBytes sizes the crypto/rand room id: 128 bits hex-encoded, long
// enough that collisions between live rooms are ignored rather than
// resolved.
const roomIDBytes = 16

// The synthetic seat ids of bots: the guest and, for Scenario 2's
// bot-vs-bot matchups, the host. The Series machine demands two distinct
// int64 sides and SQLite user ids are positive, so two distinct negative
// constants can never collide with a real account. Bot rooms persist nothing
// (Scenario 2 keeps a separate tournament rating space and record
// ownership), so the ids never reach a foreign key.
const (
	botGuestUserID int64 = -2
	botHostUserID  int64 = -3
)

// seat is one side of a room: a real user id, or a bot tier when bot is set.
type seat struct {
	userID int64
	bot    *config.Tier
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
// checks the time control and length.
func (rm *RoomManager) Create(ownerUserID int64, tcIdx, boLen int, vsBot *config.Tier) (*Room, error) {
	if ownerUserID <= 0 {
		return nil, ErrBadOwner
	}
	guest := seat{}
	if vsBot != nil {
		guest = seat{userID: botGuestUserID, bot: vsBot}
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
	if vsBot != nil {
		// The bot's handshake is immediate, so only the host ready gates
		// match 1. Ready cannot fail on a fresh series.
		if err := series.Ready(botGuestUserID); err != nil {
			return nil, err
		}
		r.series = series
		r.startBotWorker()
	}
	rm.mu.Lock()
	rm.rooms[r.id] = r
	rm.mu.Unlock()
	return r, nil
}

// CreateBotVsBot opens the Scenario 2 tournament surface: a live room whose
// host seat is a bot too, game 1 running the moment the room publishes. Bot
// handshakes are instant, so both seats ready here and the second one walks
// Room.Ready's own Created-to-Ready transition, the exact path a human room
// drives. The settings validate through NewSeries like Create, the room
// persists nothing (seriesID stays 0), and no player rating is touched.
func (rm *RoomManager) CreateBotVsBot(hostTier, guestTier *config.Tier, tcIdx, boLen int) (*Room, error) {
	if hostTier == nil || guestTier == nil {
		return nil, ErrBadTier
	}
	series, err := NewSeries(botHostUserID, botGuestUserID, tcIdx, boLen)
	if err != nil {
		return nil, err
	}
	r := &Room{
		id: newRoomID(), hub: rm.hub, store: rm.store, wq: rm.wq, manager: rm,
		tcIdx: tcIdx, boLen: boLen, createdAt: time.Now(),
		host:         seat{userID: botHostUserID, bot: hostTier},
		guest:        seat{userID: botGuestUserID, bot: guestTier},
		makeSearcher: rm.makeSearcher,
		wake:         make(chan struct{}, 1), quit: make(chan struct{}),
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
	HostWins    int
	GuestWins   int
	CreatedAt   time.Time
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

	mu       sync.Mutex
	host     seat
	guest    seat
	series   *Series
	seriesID int64
	over     bool

	// Live game state, all under mu. board is nil until the handshake
	// completes and between-terminal games never happens: the completion
	// path resets in the same critical section.
	board     *rules.Board
	moves     []rules.Move
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

// SeriesID is the persisted pairing row id, 0 for bot rooms which persist
// nothing by design.
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
