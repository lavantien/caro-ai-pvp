package server

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"strings"
	"testing"

	"github.com/lavantien/caro-ai-pvp/internal/config"
)

// fakeKDF is the deterministic argon2 stand-in: it keys the digest on every
// argon2 input (password, salt, and the full parameter set), counts
// derivations, and records the parameters it was handed, so tests assert call
// counts, stored-parameter plumbing, and fail-closed guards without burning
// real argon2.
type fakeKDF struct {
	calls        int
	lastTime     uint32
	lastMemory   uint32
	lastThreads  uint8
	lastKeyLen   uint32
	lastPassword []byte
	lastSalt     []byte
}

func (f *fakeKDF) derive(password, salt []byte, time, memory uint32, threads uint8, keyLen uint32) []byte {
	f.calls++
	f.lastTime, f.lastMemory, f.lastThreads, f.lastKeyLen = time, memory, threads, keyLen
	f.lastPassword, f.lastSalt = password, salt
	in := append(append([]byte{}, password...), salt...)
	in = binary.LittleEndian.AppendUint32(in, time)
	in = binary.LittleEndian.AppendUint32(in, memory)
	in = append(in, threads)
	in = binary.LittleEndian.AppendUint32(in, keyLen)
	sum := sha256.Sum256(in)
	out := make([]byte, keyLen)
	copy(out, sum[:])
	return out
}

func TestNewSaltMatchesConfigAndIsUnique(t *testing.T) {
	a, b := NewSalt(), NewSalt()
	if len(a) != config.Argon2SaltBytes {
		t.Errorf("salt length = %d, want %d", len(a), config.Argon2SaltBytes)
	}
	if bytes.Equal(a, b) {
		t.Error("two salts are identical, crypto/rand is not being used")
	}
}

func TestHashPasswordDeterministicAndSaltSensitive(t *testing.T) {
	salt := NewSalt()
	first := HashPassword("hunter2", salt)
	second := HashPassword("hunter2", salt)
	if len(first) != config.Argon2KeyBytes {
		t.Errorf("key length = %d, want %d", len(first), config.Argon2KeyBytes)
	}
	if !bytes.Equal(first, second) {
		t.Error("same password and salt derived two different keys")
	}
	other := HashPassword("hunter2", NewSalt())
	if bytes.Equal(first, other) {
		t.Error("different salts derived the same key")
	}
}

func TestVerifyPasswordUsesStoredPerUserParams(t *testing.T) {
	f := &fakeKDF{}
	u := User{
		Username:          "alice",
		Argon2Time:        3,
		Argon2MemoryKiB:   2048,
		Argon2Parallelism: 2,
		Salt:              []byte("0123456789abcdef"),
	}
	u.Hash = f.derive([]byte("hunter2"), u.Salt, 3, 2048, 2, config.Argon2KeyBytes)
	f.calls = 0

	if !verifyPassword(u, "hunter2", f.derive) {
		t.Fatal("verify with the matching password = false, want true")
	}
	if f.calls != 1 {
		t.Errorf("verify derivations = %d, want 1", f.calls)
	}
	if f.lastTime != 3 || f.lastMemory != 2048 || f.lastThreads != 2 {
		t.Errorf("derive params = time %d memory %d threads %d, want the user's stored 3/2048/2", f.lastTime, f.lastMemory, f.lastThreads)
	}
	if f.lastKeyLen != uint32(config.Argon2KeyBytes) {
		t.Errorf("derive keyLen = %d, want %d", f.lastKeyLen, config.Argon2KeyBytes)
	}
	if string(f.lastSalt) != string(u.Salt) {
		t.Errorf("derive salt = %q, want the stored salt", f.lastSalt)
	}
}

func TestVerifyPasswordFailsClosed(t *testing.T) {
	f := &fakeKDF{}
	u := User{
		Username:          "alice",
		Argon2Time:        config.Argon2Time,
		Argon2MemoryKiB:   config.Argon2MemoryKiB,
		Argon2Parallelism: config.Argon2Parallelism,
		Salt:              NewSalt(),
	}
	u.Hash = f.derive([]byte("hunter2"), u.Salt, config.Argon2Time, config.Argon2MemoryKiB, config.Argon2Parallelism, config.Argon2KeyBytes)

	tampered := u
	tampered.Hash = bytes.Clone(u.Hash)
	tampered.Hash[0] ^= 0xff

	wrongParams := u
	wrongParams.Argon2Time = u.Argon2Time + 1

	badSaltLen := u
	badSaltLen.Salt = bytes.Clone(u.Salt[:len(u.Salt)-1])

	badHashLen := u
	badHashLen.Hash = bytes.Clone(u.Hash[:len(u.Hash)-1])

	zeroTime := u
	zeroTime.Argon2Time = 0

	zeroThreads := u
	zeroThreads.Argon2Parallelism = 0

	cases := []struct {
		name     string
		user     User
		password string
		want     bool
		// burns says whether a well-formed row reaches the KDF at all.
		burns bool
	}{
		{"wrong password", u, "wrong-password", false, true},
		{"tampered hash byte", tampered, "hunter2", false, true},
		{"params drifted from stored", wrongParams, "hunter2", false, true},
		{"salt one byte short", badSaltLen, "hunter2", false, false},
		{"hash one byte short", badHashLen, "hunter2", false, false},
		{"empty salt", User{Hash: u.Hash, Argon2Time: 1, Argon2Parallelism: 1}, "hunter2", false, false},
		{"empty hash", User{Salt: u.Salt, Argon2Time: 1, Argon2Parallelism: 1}, "hunter2", false, false},
		{"zero time param", zeroTime, "hunter2", false, false},
		{"zero parallelism param", zeroThreads, "hunter2", false, false},
	}
	for _, tc := range cases {
		f.calls = 0
		if got := verifyPassword(tc.user, tc.password, f.derive); got != tc.want {
			t.Errorf("%s: verify = %t, want %t", tc.name, got, tc.want)
		}
		if burns := f.calls == 1; burns != tc.burns {
			t.Errorf("%s: derivations = %d, want %t path", tc.name, f.calls, tc.burns)
		}
	}
}

func TestSamePasswordDifferentSaltsDifferentStoredHashes(t *testing.T) {
	saltA, saltB := NewSalt(), NewSalt()
	hashA, hashB := HashPassword("shared-secret", saltA), HashPassword("shared-secret", saltB)
	if bytes.Equal(saltA, saltB) {
		t.Fatal("two fresh salts collided")
	}
	if bytes.Equal(hashA, hashB) {
		t.Error("same password under different salts stored the same hash")
	}
}

func TestErrBadCredentialsIsTyped(t *testing.T) {
	if ErrBadCredentials == nil || errors.Is(ErrBadCredentials, ErrNotFound) {
		t.Fatal("ErrBadCredentials must be its own typed sentinel, not ErrNotFound")
	}
}

func TestLoginOrCreateRealArgon2RoundTrip(t *testing.T) {
	s := mustOpen(t, dbPath(t))
	defer func() { _ = s.Close() }()
	now := int64(1_700_000_000)

	created, sess1, err := LoginOrCreate(s, "alice", "hunter2", now)
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if created.ID == 0 || created.CreatedAt == 0 || created.Username != "alice" {
		t.Fatalf("created user = %+v, want a stamped alice row", created)
	}
	if created.Argon2Time != config.Argon2Time || created.Argon2MemoryKiB != config.Argon2MemoryKiB || created.Argon2Parallelism != config.Argon2Parallelism {
		t.Errorf("created user params = %+v, want the config params", created)
	}
	if len(created.Salt) != config.Argon2SaltBytes || len(created.Hash) != config.Argon2KeyBytes {
		t.Errorf("created row = salt %d hash %d bytes, want %d and %d", len(created.Salt), len(created.Hash), config.Argon2SaltBytes, config.Argon2KeyBytes)
	}
	if len(sess1.Token) != config.SessionTokenBytes || sess1.UserID != created.ID {
		t.Errorf("created session = %+v, want a config-sized token for the new user", sess1)
	}

	again, sess2, err := LoginOrCreate(s, "alice", "hunter2", now+1)
	if err != nil {
		t.Fatalf("relogin: %v", err)
	}
	if again.ID != created.ID {
		t.Errorf("relogin id = %d, want the same account %d", again.ID, created.ID)
	}
	if bytes.Equal(sess1.Token, sess2.Token) {
		t.Error("relogin reused the previous session token")
	}
	if _, err := Authenticate(s, sess2.Token, now+1); err != nil {
		t.Errorf("authenticate fresh token: %v", err)
	}

	u, sess, err := LoginOrCreate(s, "alice", "wrong-password", now+2)
	if !errors.Is(err, ErrBadCredentials) {
		t.Errorf("wrong password error = %v, want ErrBadCredentials", err)
	}
	if u.ID != 0 || len(sess.Token) != 0 {
		t.Errorf("failed login returned %+v and %+v, want zero values", u, sess)
	}
}

func TestLoginOrCreateBurnsExactlyOneDerivationPerOutcome(t *testing.T) {
	s := mustOpen(t, dbPath(t))
	defer func() { _ = s.Close() }()
	now := int64(1_700_000_000)
	f := &fakeKDF{}

	if _, _, err := loginOrCreate(s, "alice", "hunter2", now, f.derive); err != nil {
		t.Fatalf("create: %v", err)
	}
	if f.calls != 1 {
		t.Fatalf("create derivations = %d, want 1", f.calls)
	}
	if f.lastTime != config.Argon2Time || f.lastMemory != config.Argon2MemoryKiB || f.lastThreads != config.Argon2Parallelism || f.lastKeyLen != config.Argon2KeyBytes {
		t.Errorf("create derive params = %d/%d/%d/%d, want config %d/%d/%d/%d",
			f.lastTime, f.lastMemory, f.lastThreads, f.lastKeyLen,
			config.Argon2Time, config.Argon2MemoryKiB, config.Argon2Parallelism, config.Argon2KeyBytes)
	}

	f.calls = 0
	if _, _, err := loginOrCreate(s, "alice", "hunter2", now+1, f.derive); err != nil {
		t.Fatalf("verify success: %v", err)
	}
	if f.calls != 1 {
		t.Errorf("verify-success derivations = %d, want 1", f.calls)
	}

	f.calls = 0
	if _, _, err := loginOrCreate(s, "alice", "wrong", now+2, f.derive); !errors.Is(err, ErrBadCredentials) {
		t.Fatalf("verify failure error = %v, want ErrBadCredentials", err)
	}
	if f.calls != 1 {
		t.Errorf("verify-failure derivations = %d, want 1: the wrong-password path must not be distinguishable from creation by KDF work", f.calls)
	}
}

func TestLoginOrCreateUsernameValidation(t *testing.T) {
	s := mustOpen(t, dbPath(t))
	defer func() { _ = s.Close() }()
	f := &fakeKDF{}
	now := int64(1_700_000_000)

	cases := []struct {
		name     string
		username string
		wantErr  bool
	}{
		{"empty", "", true},
		{"33 bytes", strings.Repeat("a", 33), true},
		{"embedded newline", "a\nb", true},
		{"nul byte", "a\x00b", true},
		{"del 0x7f", "a\x7fb", true},
		{"32 bytes", strings.Repeat("a", 32), false},
		{"32 bytes of utf-8", strings.Repeat("é", 16), false},
	}
	for _, tc := range cases {
		f.calls = 0
		_, _, err := loginOrCreate(s, tc.username, "hunter2", now, f.derive)
		if tc.wantErr {
			if !errors.Is(err, ErrInvalidUsername) {
				t.Errorf("%s: error = %v, want ErrInvalidUsername", tc.name, err)
			}
			if f.calls != 0 {
				t.Errorf("%s: derivations = %d, want 0 before any store work", tc.name, f.calls)
			}
			if _, err := s.UserByUsername(tc.username); !errors.Is(err, ErrNotFound) {
				t.Errorf("%s: rejected username left a row behind", tc.name)
			}
			continue
		}
		if err != nil {
			t.Errorf("%s: error = %v, want nil", tc.name, err)
		}
	}
}

func TestNewSessionShape(t *testing.T) {
	const now = int64(1_700_000_000)
	a, b := NewSession(7, now), NewSession(7, now)
	if len(a.Token) != config.SessionTokenBytes || len(b.Token) != config.SessionTokenBytes {
		t.Errorf("token lengths = %d and %d, want %d", len(a.Token), len(b.Token), config.SessionTokenBytes)
	}
	if bytes.Equal(a.Token, b.Token) {
		t.Error("two sessions share a token")
	}
	wantExpiry := now + config.SessionTTLHours*3600
	if a.ExpiresAt != wantExpiry || b.ExpiresAt != wantExpiry {
		t.Errorf("expiry = %d and %d, want %d", a.ExpiresAt, b.ExpiresAt, wantExpiry)
	}
	if a.UserID != 7 || b.UserID != 7 {
		t.Errorf("user id = %d and %d, want 7", a.UserID, b.UserID)
	}
}

func TestSessionExpiryBoundaryIsExclusive(t *testing.T) {
	s := mustOpen(t, dbPath(t))
	defer func() { _ = s.Close() }()
	f := &fakeKDF{}
	now := int64(1_700_000_000)

	u, sess, err := loginOrCreate(s, "alice", "hunter2", now, f.derive)
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	got, err := Authenticate(s, sess.Token, sess.ExpiresAt-1)
	if err != nil || got.ID != u.ID {
		t.Errorf("authenticate before expiry = user %+d err %v, want alice and nil", got.ID, err)
	}
	// The store filters expires_at > now, so the deadline instant is gone.
	if _, err := Authenticate(s, sess.Token, sess.ExpiresAt); !errors.Is(err, ErrBadCredentials) {
		t.Errorf("authenticate at deadline = %v, want ErrBadCredentials", err)
	}
	if _, err := Authenticate(s, sess.Token, sess.ExpiresAt+1); !errors.Is(err, ErrBadCredentials) {
		t.Errorf("authenticate past deadline = %v, want ErrBadCredentials", err)
	}
	if _, err := Authenticate(s, []byte("not-a-token"), now); !errors.Is(err, ErrBadCredentials) {
		t.Errorf("authenticate bogus token = %v, want ErrBadCredentials", err)
	}
}

func TestLogoutInvalidates(t *testing.T) {
	s := mustOpen(t, dbPath(t))
	defer func() { _ = s.Close() }()
	f := &fakeKDF{}
	now := int64(1_700_000_000)

	u, sess, err := loginOrCreate(s, "alice", "hunter2", now, f.derive)
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if got, err := Authenticate(s, sess.Token, now); err != nil || got.ID != u.ID {
		t.Fatalf("authenticate before logout = user %d err %v, want alice and nil", got.ID, err)
	}

	if err := Logout(s, sess.Token); err != nil {
		t.Fatalf("logout: %v", err)
	}
	if _, err := Authenticate(s, sess.Token, now); !errors.Is(err, ErrBadCredentials) {
		t.Errorf("authenticate after logout = %v, want ErrBadCredentials", err)
	}
	// Logout races natural expiry, so a second call stays a clean no-op.
	if err := Logout(s, sess.Token); err != nil {
		t.Errorf("repeat logout = %v, want nil", err)
	}
}

func TestLoginOrCreateTamperedRowFailsClosed(t *testing.T) {
	s := mustOpen(t, dbPath(t))
	defer func() { _ = s.Close() }()
	f := &fakeKDF{}
	now := int64(1_700_000_000)

	u, _, err := loginOrCreate(s, "alice", "hunter2", now, f.derive)
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	stored, err := s.UserByUsername("alice")
	if err != nil {
		t.Fatalf("reread row: %v", err)
	}
	flipped := bytes.Clone(stored.Hash)
	flipped[0] ^= 0xff
	if _, err := s.db.Exec("UPDATE users SET hash = ? WHERE id = ?", flipped, u.ID); err != nil {
		t.Fatalf("tamper hash: %v", err)
	}
	if _, _, err := loginOrCreate(s, "alice", "hunter2", now+1, f.derive); !errors.Is(err, ErrBadCredentials) {
		t.Errorf("login against flipped hash = %v, want ErrBadCredentials", err)
	}

	if _, err := s.db.Exec("UPDATE users SET hash = ? WHERE id = ?", []byte("short"), u.ID); err != nil {
		t.Fatalf("tamper hash length: %v", err)
	}
	if _, _, err := loginOrCreate(s, "alice", "hunter2", now+2, f.derive); !errors.Is(err, ErrBadCredentials) {
		t.Errorf("login against truncated hash = %v, want ErrBadCredentials", err)
	}
}

// racingStore simulates a registration race: the username row lands between
// LoginOrCreate's lookup and its insert, so CreateUserIfAbsent reports
// created=false on a name the lookup just missed.
type racingStore struct {
	*Store
	racer    string
	password string
	kdf      argon2KDF
}

func (r *racingStore) UserByUsername(username string) (User, error) {
	if username == r.racer {
		salt := NewSalt()
		hash := currentArgon2Params().derive(r.kdf, r.password, salt)
		if _, _, err := r.CreateUserIfAbsent(username, salt, hash); err != nil {
			return User{}, err
		}
		return User{}, ErrNotFound
	}
	return r.Store.UserByUsername(username)
}

func TestLoginOrCreateRegistrationRaceVerifies(t *testing.T) {
	s := mustOpen(t, dbPath(t))
	defer func() { _ = s.Close() }()
	f := &fakeKDF{}
	now := int64(1_700_000_000)
	r := &racingStore{Store: s, racer: "alice", password: "racing-secret", kdf: f.derive}

	u, sess, err := loginOrCreate(r, "alice", "racing-secret", now, f.derive)
	if err != nil {
		t.Fatalf("raced create with the winning password: %v", err)
	}
	if u.Username != "alice" || u.ID == 0 || sess.UserID != u.ID {
		t.Errorf("raced login = %+v session %+v, want the stored alice row with a fresh session", u, sess)
	}

	r2 := &racingStore{Store: s, racer: "bob", password: "racing-secret", kdf: f.derive}
	if _, _, err := loginOrCreate(r2, "bob", "other-password", now+1, f.derive); !errors.Is(err, ErrBadCredentials) {
		t.Errorf("raced create with a losing password = %v, want ErrBadCredentials, never a session over the winner's account", err)
	}
}
