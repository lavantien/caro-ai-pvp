package tourney

// The production RoomSource adapter's own arms over a real room manager:
// the create refusal for a broken tier pair, the subscribe refusal over a
// closed hub (which must retire the room it opened), the subscription
// wrapper surfacing the hub-close explainer, and the run-gate error's
// render.

import (
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lavantien/caro-ai-pvp/internal/config"
	"github.com/lavantien/caro-ai-pvp/internal/server"
)

// newRoomStack boots the room surface the adapter rides: store, write
// queue, hub, and manager, all torn down by the test.
func newRoomStack(t *testing.T) (*server.RoomManager, *server.Hub) {
	t.Helper()
	srv, err := server.Open(filepath.Join(t.TempDir(), "caro.sqlite"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	wq := server.NewWriteQueue(nil)
	hub := server.NewHub()
	rm := server.NewRoomManager(hub, srv, wq)
	t.Cleanup(func() {
		rm.Shutdown()
		wq.Close()
		_ = srv.Close()
		hub.Close()
	})
	return rm, hub
}

func TestRoomSourceRefusesBrokenTierPair(t *testing.T) {
	rm, _ := newRoomStack(t)
	src := RoomSource{RM: rm}

	if _, err := src.StartSeries(nil, &config.TierEasy, 0, config.SeriesBO3); !errors.Is(err, server.ErrBadTier) {
		t.Errorf("nil host tier = %v, want ErrBadTier", err)
	}
	if _, err := src.StartSeries(&config.TierEasy, nil, 0, config.SeriesBO3); !errors.Is(err, server.ErrBadTier) {
		t.Errorf("nil guest tier = %v, want ErrBadTier", err)
	}
	if n := len(rm.List()); n != 0 {
		t.Errorf("rooms after the refusals = %d, want 0", n)
	}
}

// TestRoomSourceRetiresRoomWhenSubscribeFails pins the failed-subscribe
// cleanup: over a closed hub the room itself comes up (Publish is
// fire-and-forget) but the subscription refuses, and the stream it cannot
// hand back must retire the room instead of leaking a live bot series.
func TestRoomSourceRetiresRoomWhenSubscribeFails(t *testing.T) {
	rm, hub := newRoomStack(t)
	hub.Close()

	_, err := RoomSource{RM: rm}.StartSeries(&config.TierEasy, &config.TierEasy, mustTC(1, 0), config.SeriesBO3)
	if !errors.Is(err, server.ErrHubClosed) {
		t.Fatalf("start over a closed hub = %v, want ErrHubClosed", err)
	}
	if n := len(rm.List()); n != 0 {
		t.Errorf("rooms after the refused subscribe = %d, want the opened room retired", n)
	}
}

// TestRoomStreamSurfacesHubClose pins the subscription wrapper over a live
// room: the hub's close ends the event channel and the wrapper's Err
// carries the hub-close explainer the conductor reports, and Close retires
// the room behind it.
func TestRoomStreamSurfacesHubClose(t *testing.T) {
	rm, hub := newRoomStack(t)
	room, err := rm.CreateBotVsBot(&config.TierEasy, &config.TierEasy, mustTC(1, 0), config.SeriesBO3)
	if err != nil {
		t.Fatalf("create bot room: %v", err)
	}
	sub, err := room.Subscribe()
	if err != nil {
		t.Fatalf("subscribe: %v", err)
	}
	stream := roomStream{room: room, sub: sub}

	hub.Close()
	if _, open := <-stream.Events(); open {
		t.Error("events channel still open after the hub close")
	}
	if err := stream.Err(); !errors.Is(err, server.ErrHubClosed) {
		t.Errorf("stream err = %v, want ErrHubClosed", err)
	}
	if got := stream.TruthMoves(); got != nil {
		t.Errorf("truth moves before any game end = %v, want nil", got)
	}
	stream.Close()
	if n := len(rm.List()); n != 0 {
		t.Errorf("rooms after the stream close = %d, want the room retired", n)
	}
}

func TestRunInProgressErrorRendersItsRun(t *testing.T) {
	e := &RunInProgressError{RunID: 42}
	if msg := e.Error(); !strings.Contains(msg, "42") || !strings.Contains(msg, "one run holds the machine") {
		t.Errorf("render = %q, want it naming run 42 and the machine-wide law", msg)
	}
	if !errors.Is(e, ErrRunInProgress) {
		t.Errorf("render does not match ErrRunInProgress under errors.Is")
	}
}
