package main

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"time"

	"github.com/lavantien/caro-ai-pvp/internal/config"
)

type badgeSource struct {
	SchemaVersion int      `json:"schemaVersion"`
	Sha           string   `json:"sha"`
	RunId         string   `json:"runId"`
	Verdict       string   `json:"verdict"`
	Global        *float64 `json:"global,omitempty"`
	Core          *float64 `json:"core,omitempty"`
	Timestamp     string   `json:"timestamp"`
}

func band(p, goodMin, warnMin float64) (fill, color string) {
	switch {
	case p >= goodMin:
		return "#4c1", "brightgreen"
	case p >= warnMin:
		return "#dfb317", "yellow"
	default:
		return "#e05d44", "red"
	}
}

func verdictBand(verdict string) (fill, color string, err error) {
	switch verdict {
	case config.VerdictPassed:
		return "#4c1", "brightgreen", nil
	case config.VerdictFailed:
		return "#e05d44", "red", nil
	default:
		return "", "", fmt.Errorf("unknown verdict %q, want %s or %s", verdict, config.VerdictPassed, config.VerdictFailed)
	}
}

func badgeSVG(label, value, fill string) string {
	lw := len(label)*7 + 12
	vw := len(value)*7 + 12
	return fmt.Sprintf(`<svg xmlns="http://www.w3.org/2000/svg" width="%d" height="20" role="img" aria-label="%s: %s"><linearGradient id="g" x2="0" y2="100%%"><stop offset="0" stop-color="#bbb" stop-opacity=".1"/><stop offset="1" stop-opacity=".1"/></linearGradient><clipPath id="r"><rect width="%d" height="20" rx="3"/></clipPath><g clip-path="url(#r)"><rect width="%d" height="20" fill="#555"/><rect x="%d" width="%d" height="20" fill="%s"/><rect width="%d" height="20" fill="url(#g)"/></g><g fill="#fff" font-family="Verdana,Geneva,DejaVu Sans,sans-serif" font-size="11" text-anchor="middle"><text x="%d" y="14">%s</text><text x="%d" y="14">%s</text></g></svg>`,
		lw+vw, label, value, lw+vw, lw+vw, lw, vw, fill, lw+vw, lw/2, label, lw+vw/2, value)
}

func badgeJSON(label, value, color string) string {
	return fmt.Sprintf(`{"schemaVersion":1,"label":"%s","message":"%s","color":"%s"}`, label, value, color)
}

func ciBadgeSVG(verdict string) (string, error) {
	fill, _, err := verdictBand(verdict)
	if err != nil {
		return "", err
	}
	return badgeSVG("ci", verdict, fill), nil
}

func round1(v float64) float64 {
	return math.Round(v*10) / 10
}

func writeBadges(files map[string]fileCov, dir, sha, runID, verdict string, now time.Time) error {
	if _, _, err := verdictBand(verdict); err != nil {
		return err
	}
	covered, total := overallCov(files)
	if total == 0 {
		return fmt.Errorf("empty profile, no statements counted")
	}
	global := round1(pct(covered, total))
	core := round1(coreMinPct(files))
	src, err := json.Marshal(badgeSource{
		SchemaVersion: 1,
		Sha:           sha,
		RunId:         runID,
		Verdict:       verdict,
		Global:        &global,
		Core:          &core,
		Timestamp:     now.UTC().Format(time.RFC3339),
	})
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	for _, b := range []struct {
		slug  string
		label string
		value float64
		good  float64
		warn  float64
	}{
		{"global", "global coverage", global, config.BadgeCoverageOverallGoodMin, config.BadgeCoverageOverallWarnMin},
		{"core", "core coverage", core, config.BadgeCoverageCoreGoodMin, config.BadgeCoverageCoreWarnMin},
	} {
		fill, color := band(b.value, b.good, b.warn)
		value := fmt.Sprintf("%.1f%%", b.value)
		if err := writeBadgeFile(filepath.Join(dir, "coverage-"+b.slug+".svg"), badgeSVG(b.label, value, fill)); err != nil {
			return err
		}
		if err := writeBadgeFile(filepath.Join(dir, "coverage-"+b.slug+".json"), badgeJSON(b.label, value, color)); err != nil {
			return err
		}
	}
	return writeBadgeFiles(dir, verdict, string(src))
}

func writeVerdict(dir, sha, runID, verdict string, now time.Time) error {
	if _, _, err := verdictBand(verdict); err != nil {
		return err
	}
	src, err := json.Marshal(badgeSource{
		SchemaVersion: 1,
		Sha:           sha,
		RunId:         runID,
		Verdict:       verdict,
		Timestamp:     now.UTC().Format(time.RFC3339),
	})
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	return writeBadgeFiles(dir, verdict, string(src))
}

func writeBadgeFiles(dir, verdict, src string) error {
	svg, err := ciBadgeSVG(verdict)
	if err != nil {
		return err
	}
	if err := writeBadgeFile(filepath.Join(dir, "ci.svg"), svg); err != nil {
		return err
	}
	return writeBadgeFile(filepath.Join(dir, "source.json"), src)
}

func writeBadgeFile(path, content string) error {
	return os.WriteFile(path, []byte(content), 0o644)
}

func badgesCmd(profile, dir, sha, runID, verdict string, now time.Time) error {
	files, err := readProfile(profile)
	if err != nil {
		return err
	}
	return writeBadges(files, dir, sha, runID, verdict, now)
}

func verdictCmd(verdict, dir, sha, runID string, now time.Time) error {
	return writeVerdict(dir, sha, runID, verdict, now)
}

func defaultsCmd(dir string, now time.Time) error {
	return writeDefaults(dir, now)
}

func writeDefaults(dir string, now time.Time) error {
	global, core := 0.0, 0.0
	src, err := json.Marshal(badgeSource{
		SchemaVersion: 1,
		Sha:           "none",
		RunId:         "none",
		Verdict:       config.VerdictFailed,
		Global:        &global,
		Core:          &core,
		Timestamp:     now.UTC().Format(time.RFC3339),
	})
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	for _, b := range []struct {
		slug  string
		label string
		good  float64
		warn  float64
	}{
		{"global", "global coverage", config.BadgeCoverageOverallGoodMin, config.BadgeCoverageOverallWarnMin},
		{"core", "core coverage", config.BadgeCoverageCoreGoodMin, config.BadgeCoverageCoreWarnMin},
	} {
		fill, color := band(0, b.good, b.warn)
		value := "0.0%"
		if err := writeBadgeFile(filepath.Join(dir, "coverage-"+b.slug+".svg"), badgeSVG(b.label, value, fill)); err != nil {
			return err
		}
		if err := writeBadgeFile(filepath.Join(dir, "coverage-"+b.slug+".json"), badgeJSON(b.label, value, color)); err != nil {
			return err
		}
	}
	return writeBadgeFiles(dir, config.VerdictFailed, string(src))
}
