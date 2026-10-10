package server

import "github.com/lavantien/caro-ai-pvp/internal/config"

func (rm *RoomManager) bookCoresLocked(n int) bool {
	if rm.coresUsed+n > config.MachineCores {
		return false
	}
	rm.coresUsed += n
	return true
}

func (rm *RoomManager) releaseCoresLocked(n int) {
	rm.coresUsed -= n
}

func (rm *RoomManager) bookRoomCores(r *Room, n int) bool {
	rm.mu.Lock()
	defer rm.mu.Unlock()
	if !rm.bookCoresLocked(n) {
		return false
	}
	r.bookedCores = n
	return true
}

func (rm *RoomManager) releaseRoomCores(r *Room) {
	rm.mu.Lock()
	defer rm.mu.Unlock()
	rm.releaseCoresLocked(r.bookedCores)
}

func (rm *RoomManager) LiveCoreBookings() int {
	rm.mu.Lock()
	defer rm.mu.Unlock()
	return rm.coresUsed
}
