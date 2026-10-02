package server

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"errors"
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
