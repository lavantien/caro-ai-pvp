package main

import (
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/lavantien/caro-ai-pvp/internal/config"
)

type sidecarLock struct {
	path string
	f    *os.File
	stop chan struct{}
	done chan struct{}
}

func lockPath(corpusPath, phase string) string {
	return corpusPath + "." + phase + ".lock"
}

func acquireSidecarLock(corpusPath, phase string) (*sidecarLock, error) {
	path := lockPath(corpusPath, phase)
	f, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_RDWR, 0o644)
	if err != nil {
		if !os.IsExist(err) {
			return nil, fmt.Errorf("bookgen: lock %s: %w", path, err)
		}
		fi, serr := os.Stat(path)
		if serr != nil {
			return nil, fmt.Errorf("bookgen: lock %s: %w", path, serr)
		}
		if time.Since(fi.ModTime()) < time.Duration(config.BookLockStaleMs)*time.Millisecond {
			holder, _ := os.ReadFile(path)
			return nil, fmt.Errorf("bookgen: lock %s held by pid %s, concurrent %s run refused", path, string(holder), phase)
		}
		if rerr := os.Remove(path); rerr != nil {
			return nil, fmt.Errorf("bookgen: stale lock %s: %w", path, rerr)
		}
		f, err = os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_RDWR, 0o644)
		if err != nil {
			return nil, fmt.Errorf("bookgen: lock %s after stale takeover: %w", path, err)
		}
	}
	if _, err := f.WriteString(strconv.Itoa(os.Getpid())); err != nil {
		_ = f.Close()
		_ = os.Remove(path)
		return nil, fmt.Errorf("bookgen: lock %s pid write: %w", path, err)
	}
	if err := f.Sync(); err != nil {
		_ = f.Close()
		_ = os.Remove(path)
		return nil, fmt.Errorf("bookgen: lock %s sync: %w", path, err)
	}
	l := &sidecarLock{path: path, f: f, stop: make(chan struct{}), done: make(chan struct{})}
	go l.beat()
	return l, nil
}

func (l *sidecarLock) beat() {
	defer close(l.done)
	tick := time.NewTicker(time.Duration(config.BookLockHeartbeatMs) * time.Millisecond)
	defer tick.Stop()
	for {
		select {
		case <-l.stop:
			return
		case <-tick.C:
			now := time.Now()
			_ = os.Chtimes(l.path, now, now)
		}
	}
}

func (l *sidecarLock) Release() {
	close(l.stop)
	<-l.done
	_ = l.f.Close()
	_ = os.Remove(l.path)
}
