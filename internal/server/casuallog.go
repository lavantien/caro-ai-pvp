package server

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"

	"github.com/lavantien/caro-ai-pvp/internal/config"
)

type casualLogs struct {
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
	Variant     string
	Outcome     string
}

func newCasualLogs(dir, seriesNameFormat string) *casualLogs {
	return &casualLogs{dir: dir, format: seriesNameFormat}
}

func (c *casualLogs) WriteGame(h casualGameHeader, lines []GameStat) error {
	if len(lines) == 0 {
		return nil
	}
	if h.TCIdx < 0 || h.TCIdx >= len(config.TimeControls) {
		return fmt.Errorf("server: casual log tc index %d outside the %d configured controls", h.TCIdx, len(config.TimeControls))
	}
	if err := os.MkdirAll(c.dir, 0o755); err != nil {
		return fmt.Errorf("server: casual log mkdir %s: %w", c.dir, err)
	}
	path := filepath.Join(c.dir, fmt.Sprintf(c.format, h.SeriesID, sanitizeLogName(h.Red), sanitizeLogName(h.Blue)))
	var b []byte
	b = appendCasualHeader(b, h)
	for _, s := range lines {
		b = append(b, s.Line...)
		b = append(b, '\n')
	}
	b = append(b, '\n')
	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o644)
	if err != nil {
		return fmt.Errorf("server: casual log open %s: %w", path, err)
	}
	defer func() {
		if cerr := f.Close(); cerr != nil {
			fmt.Fprintf(os.Stderr, "server: casual log close %s: %v\n", path, cerr)
		}
	}()
	if err := lockExclusive(f, 0); err != nil {
		return fmt.Errorf("server: casual log lock %s: %w", path, err)
	}
	defer func() {
		if uerr := unlockExclusive(f); uerr != nil {
			fmt.Fprintf(os.Stderr, "server: casual log unlock %s: %v\n", path, uerr)
		}
	}()
	if err := trimTornTail(f); err != nil {
		return fmt.Errorf("server: casual log recover %s: %w", path, err)
	}
	end, err := f.Seek(0, io.SeekEnd)
	if err != nil {
		return fmt.Errorf("server: casual log seek %s: %w", path, err)
	}
	if _, err := f.WriteAt(b, end); err != nil {
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
	if h.Variant != "" && h.Variant != config.DefaultVariant {
		dst = append(dst, ' ')
		dst = append(dst, h.Variant...)
	}
	dst = append(dst, " outcome "...)
	dst = append(dst, h.Outcome...)
	dst = append(dst, '\n')
	return dst
}

const casualTailScan = 4096

func trimTornTail(f *os.File) error {
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
	n, err := f.ReadAt(buf, size-scan)
	if err != nil && err != io.EOF {
		return err
	}
	if n > 0 && buf[n-1] == '\n' {
		return nil
	}
	cut := int64(-1)
	for i := n - 2; i >= 0; i-- {
		if buf[i] == '\n' {
			cut = size - scan + int64(i) + 1
			break
		}
	}
	if cut < 0 {
		cut = size - scan
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
