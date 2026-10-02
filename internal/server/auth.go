package server

import (
	"crypto/rand"
	"crypto/subtle"
	"errors"

	"golang.org/x/crypto/argon2"

	"github.com/lavantien/caro-ai-pvp/internal/config"
)

// ErrBadCredentials is the single opaque auth-failure sentinel: a wrong
// password and an unusable session token report the same error, so no caller
// can tell which leg failed or whether the username exists.
var ErrBadCredentials = errors.New("server: bad credentials")

// argon2KDF is argon2.IDKey's signature held as a value: the seam every
// derivation flows through, so tests can count calls and swap in a fast
// stand-in while the exported surface stays bound to the real KDF.
type argon2KDF func(password, salt []byte, time, memory uint32, threads uint8, keyLen uint32) []byte

// argon2Params is one account's argon2id cost parameters. Rows this server
// creates carry the config params; the per-user columns exist so a later
// parameter bump can still verify old rows with the costs they were hashed
// under.
type argon2Params struct {
	time        int
	memoryKiB   int
	parallelism int
}

func (p argon2Params) derive(kdf argon2KDF, password string, salt []byte) []byte {
	return kdf([]byte(password), salt, uint32(p.time), uint32(p.memoryKiB), uint8(p.parallelism), config.Argon2KeyBytes)
}

func currentArgon2Params() argon2Params {
	return argon2Params{
		time:        config.Argon2Time,
		memoryKiB:   config.Argon2MemoryKiB,
		parallelism: config.Argon2Parallelism,
	}
}

// NewSalt returns config.Argon2SaltBytes fresh random bytes. crypto/rand.Read
// is documented to never fail and always fill, so the ignored return is the
// full error surface.
func NewSalt() []byte {
	salt := make([]byte, config.Argon2SaltBytes)
	_, _ = rand.Read(salt)
	return salt
}

// HashPassword derives the argon2id key of password under salt with the
// current config parameters. Callers persist salt, key, and the parameters
// used, so verification recomputes with the row's own costs.
func HashPassword(password string, salt []byte) []byte {
	return currentArgon2Params().derive(argon2.IDKey, password, salt)
}

// VerifyPassword recomputes stored's key from the row's persisted per-user
// parameters and compares in constant time. Degenerate rows (salt or key
// length off, cost parameters argon2 cannot take) fail closed without
// reaching the KDF and never panic.
func VerifyPassword(stored User, password string) bool {
	return verifyPassword(stored, password, argon2.IDKey)
}

func verifyPassword(stored User, password string, kdf argon2KDF) bool {
	if len(stored.Salt) != config.Argon2SaltBytes || len(stored.Hash) != config.Argon2KeyBytes {
		return false
	}
	// argon2.IDKey panics below one round or thread and converts a negative
	// memory into a near-4-TiB allocation; sub-floor memory is degenerate the
	// same way, since the floor is 8*parallelism.
	if stored.Argon2Time < 1 || stored.Argon2Parallelism < 1 || stored.Argon2MemoryKiB < 8*stored.Argon2Parallelism {
		return false
	}
	got := argon2Params{
		time:        stored.Argon2Time,
		memoryKiB:   stored.Argon2MemoryKiB,
		parallelism: stored.Argon2Parallelism,
	}.derive(kdf, password, stored.Salt)
	return subtle.ConstantTimeCompare(got, stored.Hash) == 1
}
