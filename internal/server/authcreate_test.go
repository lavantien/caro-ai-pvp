package server

import (
	"errors"
	"strings"
	"testing"
)

// failingCreates wraps the store so the registration insert always fails,
// the branch where the account write itself is the broken leg.
type failingCreates struct {
	*Store
}

func (failingCreates) CreateUserIfAbsent(string, []byte, []byte) (User, bool, error) {
	return User{}, false, errors.New("server: simulated create failure")
}

func TestLoginOrCreateCreateFailureSurfaces(t *testing.T) {
	s := mustOpen(t, dbPath(t))
	defer func() { _ = s.Close() }()
	f := &fakeKDF{}
	broken := failingCreates{Store: s}

	_, _, err := loginOrCreate(broken, "alice", "hunter2", 1_700_000_000, f.derive)
	if err == nil || !strings.Contains(err.Error(), "simulated create failure") {
		t.Fatalf("login with a failing create = %v, want the create failure surfaced", err)
	}
	if f.calls != 1 {
		t.Errorf("derivations on the failed create = %d, want exactly the one", f.calls)
	}
	// The account row never landed, so the store still reports the name free.
	if _, err := s.UserByUsername("alice"); !errors.Is(err, ErrNotFound) {
		t.Errorf("user after the failed create = %v, want ErrNotFound", err)
	}
}
