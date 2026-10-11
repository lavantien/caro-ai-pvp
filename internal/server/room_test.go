package server

import (
	"errors"
	"testing"
	"time"

	"github.com/lavantien/caro-ai-pvp/internal/config"
	"github.com/lavantien/caro-ai-pvp/internal/rules"
)

type stack struct {
	store *Store
	wq    *WriteQueue
	hub   *Hub
	rm    *RoomManager
}

func newStack(t *testing.T) *stack {
	t.Helper()
	st := mustOpen(t, dbPath(t))
	wq := NewWriteQueue(nil)
	hub := NewHub()
	rm := NewRoomManager(hub, st, wq)
	t.Cleanup(func() {
		rm.Shutdown()
		wq.Close()
		_ = st.Close()
		hub.Close()
	})
	return &stack{store: st, wq: wq, hub: hub, rm: rm}
}

func TestRoomCreateValidatesSettings(t *testing.T) {
	s := newStack(t)
	alice := seedUser(t, s.store, "alice")

	if _, err := s.rm.Create(alice.ID, -1, config.SeriesBO3, nil); !errors.Is(err, ErrBadTimeControl) {
		t.Errorf("create tc -1 = %v, want ErrBadTimeControl", err)
	}
	if _, err := s.rm.Create(alice.ID, len(config.TimeControls), config.SeriesBO3, nil); !errors.Is(err, ErrBadTimeControl) {
		t.Errorf("create tc overflow = %v, want ErrBadTimeControl", err)
	}
	for _, bo := range []int{0, 2, 4} {
		if _, err := s.rm.Create(alice.ID, 0, bo, nil); !errors.Is(err, ErrBadSeriesLength) {
			t.Errorf("create bo %d = %v, want ErrBadSeriesLength", bo, err)
		}
	}
	if _, err := s.rm.Create(0, 0, config.SeriesBO3, nil); !errors.Is(err, ErrBadOwner) {
		t.Errorf("create owner 0 = %v, want ErrBadOwner", err)
	}

	r, err := s.rm.Create(alice.ID, 1, config.SeriesBO5, nil)
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if len(r.ID()) != 2*roomIDBytes {
		t.Errorf("id %q, want %d hex chars", r.ID(), 2*roomIDBytes)
	}
	rooms := s.rm.List()
	if len(rooms) != 1 {
		t.Fatalf("list = %d rooms, want 1", len(rooms))
	}
	want := RoomInfo{
		ID: r.ID(), HostUserID: alice.ID, GuestUserID: 0,
		TCIdx: 1, BOLen: config.SeriesBO5, State: SeriesCreated,
		VsBotTier: "", CreatedAt: rooms[0].CreatedAt,
	}
	if rooms[0] != want {
		t.Errorf("info = %+v, want %+v", rooms[0], want)
	}
}

func TestRoomIDsAreUniqueOpaqueStrings(t *testing.T) {
	s := newStack(t)
	alice := seedUser(t, s.store, "alice")
	seen := make(map[string]bool, 32)
	for range 32 {
		r, err := s.rm.Create(alice.ID, 0, config.SeriesBO3, nil)
		if err != nil {
			t.Fatalf("create: %v", err)
		}
		if seen[r.ID()] {
			t.Fatalf("duplicate room id %q", r.ID())
		}
		seen[r.ID()] = true
	}
}

func TestRoomJoinSeatsOpponentAndPersistsPairing(t *testing.T) {
	s := newStack(t)
	alice := seedUser(t, s.store, "alice")
	bob := seedUser(t, s.store, "bob")
	carol := seedUser(t, s.store, "carol")
	r, err := s.rm.Create(alice.ID, 0, config.SeriesBO3, nil)
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	if _, err := s.rm.Get("no-such-room"); !errors.Is(err, ErrRoomNotFound) {
		t.Errorf("get missing = %v, want ErrRoomNotFound", err)
	}
	if err := s.rm.Join("no-such-room", bob.ID); !errors.Is(err, ErrRoomNotFound) {
		t.Errorf("join missing = %v, want ErrRoomNotFound", err)
	}
	if err := s.rm.Join(r.ID(), alice.ID); !errors.Is(err, ErrSamePlayer) {
		t.Errorf("host joins own room = %v, want ErrSamePlayer", err)
	}
	if err := s.rm.Join(r.ID(), bob.ID); err != nil {
		t.Fatalf("join: %v", err)
	}
	if err := s.rm.Join(r.ID(), carol.ID); !errors.Is(err, ErrRoomFull) {
		t.Errorf("third join = %v, want ErrRoomFull", err)
	}

	info, ok := r.Info()
	if !ok {
		t.Fatal("info after join: want ok")
	}
	if info.GuestUserID != bob.ID || info.State != SeriesCreated || info.HostUserID != alice.ID {
		t.Errorf("info after join = %+v, want bob seated and created", info)
	}

	sr, err := s.store.SeriesByID(r.SeriesID())
	if err != nil {
		t.Fatalf("series row: %v", err)
	}
	if sr.TCIdx != 0 || sr.BOLen != config.SeriesBO3 || sr.RedUser != alice.ID || sr.BlueUser != bob.ID {
		t.Errorf("series row = %+v, want host on red against bob", sr)
	}
	if sr.State != SeriesStateOngoing || sr.Winner != nil || sr.FinishedAt != nil {
		t.Errorf("series row = %+v, want fresh ongoing", sr)
	}
}

func TestRoomCreateBotSeating(t *testing.T) {
	s := newStack(t)
	alice := seedUser(t, s.store, "alice")
	bob := seedUser(t, s.store, "bob")
	r, err := s.rm.Create(alice.ID, 0, config.SeriesBO3, &config.Tiers[1])
	if err != nil {
		t.Fatalf("create vs bot: %v", err)
	}
	botID, err := s.store.BotAccountID(config.Tiers[1])
	if err != nil {
		t.Fatalf("resolve medium seat: %v", err)
	}

	info, ok := r.Info()
	if !ok {
		t.Fatal("info: want ok")
	}
	if info.GuestUserID != botID || info.VsBotTier != config.TierMedium.Name {
		t.Errorf("info = %+v, want the medium bot's reserved account seated as guest", info)
	}
	if err := s.rm.Join(r.ID(), bob.ID); !errors.Is(err, ErrRoomFull) {
		t.Errorf("join bot room = %v, want ErrRoomFull", err)
	}

	r.mu.Lock()
	series, guestReady, hostReady := r.series, r.series.GuestReady(), r.series.HostReady()
	r.mu.Unlock()
	if series == nil {
		t.Fatal("bot room: want the series formed at create")
	}
	if !guestReady || hostReady {
		t.Errorf("bot readiness = guest %t host %t, want guest ready and host pending", guestReady, hostReady)
	}

	sr, err := s.store.SeriesByID(r.SeriesID())
	if err != nil {
		t.Fatalf("series row: %v", err)
	}
	if sr.RedUser != alice.ID || sr.BlueUser != botID || sr.State != SeriesStateOngoing {
		t.Errorf("bot pairing row = %+v, want ongoing host over the reserved seat %d", sr, botID)
	}
}

func TestRoomListOrderedByCreation(t *testing.T) {
	s := newStack(t)
	hosts := []string{"alice", "bob", "carol"}
	ids := []int64{}
	for _, name := range hosts {
		u := seedUser(t, s.store, name)
		ids = append(ids, u.ID)
		if _, err := s.rm.Create(u.ID, 0, config.SeriesBO3, nil); err != nil {
			t.Fatalf("create %s: %v", name, err)
		}
		time.Sleep(2 * time.Millisecond)
	}
	rooms := s.rm.List()
	if len(rooms) != len(hosts) {
		t.Fatalf("list = %d rooms, want %d", len(rooms), len(hosts))
	}
	for i, info := range rooms {
		if info.HostUserID != ids[i] {
			t.Errorf("rooms[%d] host = %d, want %d in creation order", i, info.HostUserID, ids[i])
		}
	}
}

func TestRoomShutdownClosesAndClears(t *testing.T) {
	s := newStack(t)
	alice := seedUser(t, s.store, "alice")
	first, err := s.rm.Create(alice.ID, 0, config.SeriesBO3, nil)
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if _, err := s.rm.Create(alice.ID, 1, config.SeriesBO5, nil); err != nil {
		t.Fatalf("create: %v", err)
	}

	s.rm.Shutdown()
	if rooms := s.rm.List(); len(rooms) != 0 {
		t.Errorf("list after shutdown = %d rooms, want 0", len(rooms))
	}
	if _, err := s.rm.Get(first.ID()); !errors.Is(err, ErrRoomNotFound) {
		t.Errorf("get after shutdown = %v, want ErrRoomNotFound", err)
	}
	s.rm.Shutdown()
}

func TestRoomCreateAcceptsOwnerIDOne(t *testing.T) {
	s := newStack(t)
	r, err := s.rm.Create(1, 0, config.SeriesBO3, nil)
	if err != nil {
		t.Fatalf("create owner 1 = %v, want the positive-id boundary accepted", err)
	}
	if _, err := s.rm.Get(r.ID()); err != nil {
		t.Errorf("get after boundary create = %v, want the room live", err)
	}
}

func TestNewRoomIDEncodes32HexChars(t *testing.T) {
	for range 8 {
		id := newRoomID()
		if len(id) != 32 {
			t.Fatalf("room id %q len = %d, want 32 hex chars", id, len(id))
		}
		for i := range id {
			c := id[i]
			if (c < '0' || c > '9') && (c < 'a' || c > 'f') {
				t.Fatalf("room id %q byte %d = %q, want lowercase hex", id, i, c)
			}
		}
	}
}

func TestCreateBotVsBotSeatsSyntheticAccounts(t *testing.T) {
	s := newStack(t)
	release := scriptBotVsBotTiers(t, s, map[string][]string{
		"easy":   {"D4", "H8", "D5", "D6", "D7", "D3"},
		"medium": {"P16", "P12", "P8", "N16", "M4"},
	})
	defer release()
	r, err := s.rm.CreateBotVsBot(&config.TierEasy, "", &config.TierMedium, "", 0, config.SeriesBO3)
	if err != nil {
		t.Fatalf("create bot vs bot: %v", err)
	}
	info, ok := r.Info()
	if !ok {
		t.Fatal("info: want ok while the gated bots hold the game open")
	}
	if info.HostUserID != -3 || info.GuestUserID != -2 {
		t.Errorf("synthetic seats = host %d guest %d, want the reserved -3 and -2", info.HostUserID, info.GuestUserID)
	}
}

func TestLiveBotBoardGuardMatrix(t *testing.T) {
	rows := []struct {
		name string
		mut  func(*Room)
	}{
		{"closed room", func(r *Room) { r.over = true }},
		{"completed series", func(r *Room) { r.finished = true }},
		{"host seat human", func(r *Room) { r.host = seat{userID: 1} }},
		{"guest seat human", func(r *Room) { r.guest = seat{userID: 2} }},
		{"series not formed", func(r *Room) { r.series = nil }},
	}
	for _, row := range rows {
		r := roomcoreLiveRoom(t, nil)
		row.mut(r)
		if _, ok := r.liveBotBoard(); ok {
			t.Errorf("%s: liveBotBoard = ok, want the guard to refuse the room", row.name)
		}
	}
}

func roomcoreLiveRoom(t *testing.T, rm *RoomManager) *Room {
	t.Helper()
	series, err := NewBotSeries(1, 2, 0, config.SeriesBO3)
	if err != nil {
		t.Fatalf("craft bot series: %v", err)
	}
	return &Room{
		id: newRoomID(), manager: rm, createdAt: time.Now(),
		host:   seat{userID: 1, bot: &config.Tiers[0]},
		guest:  seat{userID: 2, bot: &config.Tiers[0]},
		series: series, board: rules.NewBoard(),
		wake: make(chan struct{}, 1), quit: make(chan struct{}),
	}
}
