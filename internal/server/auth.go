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

var ErrBadCredentials = errors.New("server: bad credentials")

type argon2KDF func(password, salt []byte, time, memory uint32, threads uint8, keyLen uint32) []byte

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

func NewSalt() []byte {
	salt := make([]byte, config.Argon2SaltBytes)
	_, _ = rand.Read(salt)
	return salt
}

func HashPassword(password string, salt []byte) []byte {
	return currentArgon2Params().derive(argon2.IDKey, password, salt)
}

func VerifyPassword(stored User, password string) bool {
	return verifyPassword(stored, password, argon2.IDKey)
}

func verifyPassword(stored User, password string, kdf argon2KDF) bool {
	if len(stored.Salt) != config.Argon2SaltBytes || len(stored.Hash) != config.Argon2KeyBytes {
		return false
	}
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

var ErrInvalidUsername = errors.New("server: invalid username")

func validateUsername(username string) error {
	if username == "" || username == "." || username == ".." ||
		len(username) > config.UsernameMaxBytes || !utf8.ValidString(username) {
		return ErrInvalidUsername
	}
	for _, r := range username {
		if unicode.IsControl(r) || unicode.Is(unicode.Cf, r) {
			return ErrInvalidUsername
		}
	}
	return nil
}

const sessionTTLSec = int64(config.SessionTTLHours) * int64(time.Hour/time.Second)

func NewSession(userID, now int64) Session {
	token := make([]byte, config.SessionTokenBytes)
	_, _ = rand.Read(token)
	return Session{Token: token, UserID: userID, ExpiresAt: now + sessionTTLSec}
}

type authStore interface {
	UserByUsername(username string) (User, error)
	CreateUserIfAbsent(username string, salt, hash []byte) (User, bool, error)
	InsertSession(sess Session) error
}

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

func Logout(store *Store, token []byte) error {
	return store.DeleteSession(token)
}
