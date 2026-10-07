package server

import "github.com/lavantien/caro-ai-pvp/internal/config"

// The v0.23 machine-wide core ledger: the room manager books every
// bot-holding room against config.MachineCores for the room's whole life.
// Turns alternate inside a room, so only one seat searches at any instant
// and a room books the max of its tiers' cores while a human seat books
// nothing. The whole-room hold is conservative and deliberate; v0.26
// replaces it with search-level admission that reserves cores only while a
// search actually runs.

// bookCoresLocked reserves n cores against the machine budget: true when the
// booking lands, false when used+n would exceed config.MachineCores, in
// which case nothing changes. Callers hold rm.mu.
func (rm *RoomManager) bookCoresLocked(n int) bool {
	if rm.coresUsed+n > config.MachineCores {
		return false
	}
	rm.coresUsed += n
	return true
}

// releaseCoresLocked returns a booking to the budget. Every release pairs
// with exactly one landed booking, so the total never goes negative.
// Callers hold rm.mu.
func (rm *RoomManager) releaseCoresLocked(n int) {
	rm.coresUsed -= n
}

// bookRoomCores is the create surfaces' booking step: reserve the room's
// cores and stamp them on the room, so the terminal retire returns exactly
// what landed here. False is the ErrMachineBusy refusal.
func (rm *RoomManager) bookRoomCores(r *Room, n int) bool {
	rm.mu.Lock()
	defer rm.mu.Unlock()
	if !rm.bookCoresLocked(n) {
		return false
	}
	r.bookedCores = n
	return true
}

// releaseRoomCores returns one room's booking to the budget. Retire's
// closeOnce is the single funnel every terminal path runs (series complete,
// forfeit, room close, the shutdown sweep), so the release rides it exactly
// once per room; the manager key survives a plain Room.Close by the pinned
// mid-retirement law, so the map delete alone cannot own the release.
func (rm *RoomManager) releaseRoomCores(r *Room) {
	rm.mu.Lock()
	defer rm.mu.Unlock()
	rm.releaseCoresLocked(r.bookedCores)
}

// LiveCoreBookings reads the ledger's live total, the tournament start
// gate's casual-room check: at startRun time every booking belongs to an
// open room outside the run, and MachineCores budgets the machine as a
// whole.
func (rm *RoomManager) LiveCoreBookings() int {
	rm.mu.Lock()
	defer rm.mu.Unlock()
	return rm.coresUsed
}
