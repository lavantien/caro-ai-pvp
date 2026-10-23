package server

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"sync"

	"github.com/lavantien/caro-ai-pvp/internal/config"
)

type casualLogs struct {
	mu     sync.Mutex
	dir    string
	format string
}

type casualGameHeader struct {
	SeriesID    int64
	GameNo      int
	RoomID      string
	Mode        string
	Red         string
	Blue        string
	PairingRed  string
	PairingBlue string
	TCIdx       int
	BOLen       int
	Outcome     string
}

func newCasualLogs(dir, seriesNameFormat string) *casualLogs {
	return &casualLogs{dir: dir, format: seriesNameFormat}
}

func (c *casualLogs) WriteGame(h casualGameHeader, lines []GameStat) error {
	if len(lines) == 0 {
		return nil
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if err := os.MkdirAll(c.dir, 0o755); err != nil {
		return fmt.Errorf("server: casual log mkdir %s: %w", c.dir, err)
	}
	path := filepath.Join(c.dir, fmt.Sprintf(c.format, h.SeriesID, sanitizeLogName(h.Red), sanitizeLogName(h.Blue)))
	if err := trimTornTail(path); err != nil {
		return fmt.Errorf("server: casual log recover %s: %w", path, err)
	}
	var b []byte
	b = appendCasualHeader(b, h)
	for _, s := range lines {
		b = append(b, s.Line...)
		b = append(b, '\n')
	}
	b = append(b, '\n')
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return fmt.Errorf("server: casual log open %s: %w", path, err)
	}
	defer func() {
		if cerr := f.Close(); cerr != nil {
			fmt.Fprintf(os.Stderr, "server: casual log close %s: %v\n", path, cerr)
		}
	}()
	if _, err := f.Write(b); err != nil {
		return fmt.Errorf("server: casual log write %s: %w", path, err)
	}
	return nil
}

func appendCasualHeader(dst []byte, h casualGameHeader) []byte {
	dst = append(dst, "game "...)
	dst = strconv.AppendInt(dst, int64(h.GameNo), 10)
	dst = append(dst, " room "...)

	dst = append(dst, h.RoomID...)
	dst = append(dst, " mode "...)
	dst = append(dst, h.Mode...)
	dst = append(dst, '\n')

	dst = append(dst, "pairing "...)
	dst = append(dst, h.PairingRed...)
	dst = append(dst, " vs "...)
	dst = append(dst, h.PairingBlue...)
	dst = append(dst, '\n')

	tc := config.TimeControls[h.TCIdx]
	dst = append(dst, "tc "...)
	dst = strconv.AppendInt(dst, int64(tc.InitialMin), 10)
	dst = append(dst, '+')
	dst = strconv.AppendInt(dst, int64(tc.IncrementSec), 10)
	dst = append(dst, " bo"...)
	dst = strconv.AppendInt(dst, int64(h.BOLen), 10)
	dst = append(dst, " outcome "...)
	dst = append(dst, h.Outcome...)
	dst = append(dst, '\n')
	return dst
}

const casualTailScan = 4096

func trimTornTail(path string) error {
	f, err := os.OpenFile(path, os.O_RDWR, 0o644)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	defer func() { _ = f.Close() }()
	info, err := f.Stat()
	if err != nil {
		return err
	}
	size := info.Size()
	if size == 0 {
		return nil
	}
	scan := int64(casualTailScan)
	if scan > size {
		scan = size
	}
	buf := make([]byte, scan)
	if _, err := f.ReadAt(buf, size-scan); err != nil && err != io.EOF {
		return err
	}
	if buf[len(buf)-1] == '\n' {
		return nil
	}
	cut := int64(-1)
	for i := len(buf) - 2; i >= 0; i-- {
		if buf[i] == '\n' {
			cut = size - scan + int64(i) + 1
			break
		}
	}
	if cut < 0 {
		cut = 0
	}
	return f.Truncate(cut)
}

const casualNameMaxRunes = 64

func sanitizeLogName(name string) string {
	out := make([]rune, 0, min(len(name), casualNameMaxRunes))
	for _, r := range name {
		if len(out) == casualNameMaxRunes {
			break
		}
		if r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' ||
			r == '.' || r == '-' || r == '_' {
			out = append(out, r)
			continue
		}
		out = append(out, '_')
	}
	return string(out)
}
