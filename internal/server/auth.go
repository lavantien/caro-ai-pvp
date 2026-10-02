package server

import (
	"crypto/rand"
	"crypto/subtle"
	"errors"
	"time"
	"unicode"
	"unicode/utf8"

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
// length off, cost parameters outside the accepted envelope) fail closed
// without reaching the KDF and never panic.
func VerifyPassword(stored User, password string) bool {
	return verifyPassword(stored, password, argon2.IDKey)
}

func verifyPassword(stored User, password string, kdf argon2KDF) bool {
	if len(stored.Salt) != config.Argon2SaltBytes || len(stored.Hash) != config.Argon2KeyBytes {
		return false
	}
	// Cost envelope, era rule: rows are stamped at the config parameters of
	// their day, so anything above today's config set is tampered or corrupt
	// and fails closed, while weaker legacy rows (a parameter bump's
	// predecessors) stay verifiable until a rehash migrates them. The bounds
	// also keep the int-to-uint32/uint8 narrowing lossless: below one round
	// or thread argon2.IDKey panics, a parallelism of 256 wraps to uint8
	// zero, and an oversized memory would drive the derivation toward a huge
	// allocation instead of failing.
	if stored.Argon2Time < 1 || stored.Argon2Time > config.Argon2Time ||
		stored.Argon2Parallelism < 1 || stored.Argon2Parallelism > config.Argon2Parallelism ||
		stored.Argon2MemoryKiB < 8*stored.Argon2Parallelism || stored.Argon2MemoryKiB > config.Argon2MemoryKiB {
		return false
	}
	got := argon2Params{
		time:        stored.Argon2Time,
		memoryKiB:   stored.Argon2MemoryKiB,
		parallelism: stored.Argon2Parallelism,
	}.derive(kdf, password, stored.Salt)
	return subtle.ConstantTimeCompare(got, stored.Hash) == 1
}

// ErrInvalidUsername rejects a syntactically unusable username before any
// store access: empty, past config.UsernameMaxBytes, invalid UTF-8, or
// carrying a control or invisible format rune.
var ErrInvalidUsername = errors.New("server: invalid username")

func validateUsername(username string) error {
	if username == "" || len(username) > config.UsernameMaxBytes || !utf8.ValidString(username) {
		return ErrInvalidUsername
	}
	for _, r := range username {
		// Cc covers the C0 and C1 controls; Cf the invisible format runes
		// (bidi overrides, joiners) that spoof rendered names.
		if unicode.IsControl(r) || unicode.Is(unicode.Cf, r) {
			return ErrInvalidUsername
		}
	}
	return nil
}

// sessionTTLSec is the expiry horizon in the store's unix-second domain,
// derived from the config hub's hours so no raw second count lives here.
const sessionTTLSec = int64(config.SessionTTLHours) * int64(time.Hour/time.Second)

// NewSession mints one opaque token of config.SessionTokenBytes random bytes
// for userID, expiring sessionTTLSec past now. The store compares
// expires_at > now exclusively, so the deadline instant is already expired.
func NewSession(userID, now int64) Session {
	token := make([]byte, config.SessionTokenBytes)
	_, _ = rand.Read(token)
	return Session{Token: token, UserID: userID, ExpiresAt: now + sessionTTLSec}
}

// authStore is the store surface the login flow touches. An interface keeps
// the registration-race branch deterministically testable.
type authStore interface {
	UserByUsername(username string) (User, error)
	CreateUserIfAbsent(username string, salt, hash []byte) (User, bool, error)
	InsertSession(sess Session) error
}

// LoginOrCreate implements the one-form auth of first-cause.md Scenario 1:
// an unknown username registers the account and logs in, a known one
// verifies the password and logs in.
//
// Timing shape, the property the KDF-call-count test pins: every
// store-touching path performs exactly one argon2id derivation, with the
// same parameter set on both legs (config params on creation, the row's
// stored params on verification, and every row this server writes carries
// the config params). The derivation dominates wall clock at these costs,
// so wrong-password failure and unknown-username creation are the same
// shape and no dummy burn is needed. Failures all report the one opaque
// ErrBadCredentials. Username rejection is purely syntactic, runs before
// any store access, and burns nothing. The single two-derivation path is
// the registration race below.
//
// The session inserts are plain store calls for now: the integration layer
// routes every write through WriteQueue later.
func LoginOrCreate(store *Store, username, password string, now int64) (User, Session, error) {
	return loginOrCreate(store, username, password, now, argon2.IDKey)
}

func loginOrCreate(store authStore, username, password string, now int64, kdf argon2KDF) (User, Session, error) {
	if err := validateUsername(username); err != nil {
		return User{}, Session{}, err
	}
	u, err := store.UserByUsername(username)
	if errors.Is(err, ErrNotFound) {
		salt := NewSalt()
		hash := currentArgon2Params().derive(kdf, password, salt)
		var created bool
		u, created, err = store.CreateUserIfAbsent(username, salt, hash)
		if err != nil {
			return User{}, Session{}, err
		}
		if created {
			sess, err := startSession(store, u.ID, now)
			if err != nil {
				return User{}, Session{}, err
			}
			return u, sess, nil
		}
		// Registration race lost: the row landed between the lookup and the
		// insert, so the stored row wins and this request verifies against
		// it like any known user. The only path that burns a second
		// derivation, and reaching it requires creating the account first.
	} else if err != nil {
		return User{}, Session{}, err
	}
	if !verifyPassword(u, password, kdf) {
		return User{}, Session{}, ErrBadCredentials
	}
	sess, err := startSession(store, u.ID, now)
	if err != nil {
		return User{}, Session{}, err
	}
	return u, sess, nil
}

func startSession(store authStore, userID, now int64) (Session, error) {
	sess := NewSession(userID, now)
	if err := store.InsertSession(sess); err != nil {
		return Session{}, err
	}
	return sess, nil
}

// Authenticate resolves a live session token to its user. Missing tokens,
// expired tokens, and tokens whose user row vanished all map onto
// ErrBadCredentials, so token probing learns nothing but failure; other
// store failures pass through untouched.
func Authenticate(store *Store, token []byte, now int64) (User, error) {
	sess, err := store.SessionByToken(token, now)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return User{}, ErrBadCredentials
		}
		return User{}, err
	}
	u, err := store.UserByID(sess.UserID)
	if errors.Is(err, ErrNotFound) {
		return User{}, ErrBadCredentials
	}
	return u, err
}

// Logout drops the token. Deleting a missing token is not an error because
// logout races natural expiry. Plain store call for now, WriteQueue-routed
// by the integration layer later.
func Logout(store *Store, token []byte) error {
	return store.DeleteSession(token)
}
