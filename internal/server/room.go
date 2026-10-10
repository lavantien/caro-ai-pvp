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

var (
	ErrRoomNotFound = errors.New("server: room not found")
	ErrRoomFull     = errors.New("server: room already has an opponent")
	ErrRoomClosed   = errors.New("server: room closed")
	ErrNotYourTurn  = errors.New("server: not this side's turn")
	ErrIllegalMove  = errors.New("server: illegal move")
	ErrBadOwner     = errors.New("server: room owner must be a positive user id")
	ErrBadTier      = errors.New("server: bot-vs-bot needs both tiers")
	ErrUnknownTier  = errors.New("server: unknown bot tier")
	ErrMachineBusy  = errors.New("server: machine core budget full")
)

const roomIDBytes = 16
const (
	botGuestUserID int64 = -2
	botHostUserID  int64 = -3
)

type seat struct {
	userID int64
	bot    *config.Tier
	name   string
}
type RoomManager struct {
	mu           sync.Mutex
	rooms        map[string]*Room
	hub          *Hub
	store        *Store
	wq           *WriteQueue
	coresUsed    int
	makeSearcher func(config.Tier) searcher
}

func NewRoomManager(hub *Hub, store *Store, wq *WriteQueue) *RoomManager {
	return &RoomManager{
		rooms: make(map[string]*Room), hub: hub, store: store, wq: wq,
		makeSearcher: newBotSearcher,
	}
}
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
		makeSearcher: rm.makeSearcher, ponderOn: true,
		wake: make(chan struct{}, 1), quit: make(chan struct{}),
	}
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
		makeSearcher: rm.makeSearcher, ponderOn: true,
		wake: make(chan struct{}, 1), quit: make(chan struct{}),
	}
	published := false
	defer func() {
		if !published {
			rm.releaseRoomCores(r)
		}
	}()
	peak := config.RoomCores(*hostTier, *guestTier)
	floor := max(hostTier.Cores, guestTier.Cores)
	if peak > floor {
		if rm.bookRoomCores(r, peak) {
			r.ponderOn = true
		} else if rm.bookRoomCores(r, floor) {
			r.ponderOn = false
		} else {
			return nil, ErrMachineBusy
		}
	} else {
		r.ponderOn = false
		if !rm.bookRoomCores(r, floor) {
			return nil, ErrMachineBusy
		}
	}
	r.mu.Lock()
	r.series = series
	r.mu.Unlock()
	if err := r.Ready(botHostUserID); err != nil {
		return nil, err
	}
	if err := r.Ready(botGuestUserID); err != nil {
		return nil, err
	}
	rm.mu.Lock()
	rm.rooms[r.id] = r
	published = true
	rm.mu.Unlock()
	r.startBotWorker()
	return r, nil
}
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
	if r.over || r.finished {
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
func (rm *RoomManager) Get(roomID string) (*Room, error) {
	rm.mu.Lock()
	defer rm.mu.Unlock()
	r, ok := rm.rooms[roomID]
	if !ok {
		return nil, ErrRoomNotFound
	}
	return r, nil
}

type RoomInfo struct {
	ID           string
	HostUserID   int64
	GuestUserID  int64
	TCIdx        int
	BOLen        int
	State        SeriesState
	HostBotTier  string
	VsBotTier    string
	HostBotName  string
	GuestBotName string
	HostWins     int
	GuestWins    int
	CreatedAt    time.Time
}

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

func (r *Room) liveBotBoard() (LiveBotBoard, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.over || r.finished || r.host.bot == nil || r.guest.bot == nil || r.series == nil || r.board == nil {
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
func (rm *RoomManager) retireRoom(r *Room) {
	rm.mu.Lock()
	delete(rm.rooms, r.id)
	rm.mu.Unlock()
}

type Room struct {
	id           string
	hub          *Hub
	store        *Store
	wq           *WriteQueue
	manager      *RoomManager
	tcIdx        int
	boLen        int
	createdAt    time.Time
	bookedCores  int
	mu           sync.Mutex
	host         seat
	guest        seat
	series       *Series
	seriesID     int64
	finished     bool
	over         bool
	board        *rules.Board
	moves        []rules.Move
	lastMoves    []rules.Move
	mlines       []GameStat
	clock        [2]*clock.GameClock
	turnStart    time.Time
	engines      [2]searcher
	legalBuf     [config.BoardCells]rules.Move
	lastM        mLineRecord
	ponder       *ponderLane
	ponderOn     bool
	ponderRing   [2]GrantDepthRing
	budgetCap    time.Duration
	makeSearcher func(config.Tier) searcher
	searchEntry  func()
	wake         chan struct{}
	quit         chan struct{}
	closeOnce    sync.Once
	wg           sync.WaitGroup
}

func (r *Room) ID() string { return r.id }
func (r *Room) SeriesID() int64 {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.seriesID
}
func (r *Room) Subscribe() (*Subscription, error) {
	return r.hub.Subscribe(r.id)
}
func (r *Room) LastGameMoves() []rules.Move {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]rules.Move(nil), r.lastMoves...)
}
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
func (r *Room) seatInstanceName(s *seat) string {
	if s.name != "" {
		return s.name
	}
	return s.bot.Name
}
func (r *Room) Close() {
	r.retire()
	r.wg.Wait()
}
func (r *Room) retire() {
	r.closeOnce.Do(func() {
		r.mu.Lock()
		close(r.quit)
		r.mu.Unlock()
		r.manager.releaseRoomCores(r)
		r.mu.Lock()
		r.over = true
		r.mu.Unlock()
	})
}
func (r *Room) quitClosed() bool {
	select {
	case <-r.quit:
		return true
	default:
		return false
	}
}
func (r *Room) finishLifecycle() {
	r.retire()
	r.manager.retireRoom(r)
}
func newRoomID() string {
	var b [roomIDBytes]byte
	if _, err := rand.Read(b[:]); err != nil {
		panic("server: crypto/rand room id: " + err.Error())
	}
	return hex.EncodeToString(b[:])
}
