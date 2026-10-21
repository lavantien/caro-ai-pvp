package main

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

var mLineRe = regexp.MustCompile(
	`^M(\d+), (Red|Blue), ([A-P](?:[1-9]|1[0-6])), d=(\d+), n=([0-9.]+[kmg]?), nps=([0-9.]+[kmg]?), ebf=([0-9.]+), ` +
		`tt=(\d+)%, hf=(\d+)%, fh1=(\d+)%, s=([^,]+), thr=(\d+), t=([0-9.]+), alloc=([0-9.]+)(, (\[[A-Z]+\]))?, pv=(.*)$`)

type mLine struct {
	move        int
	side        string
	cell        string
	depth       int
	nodes       uint64
	nps         uint64
	ebf         float64
	tt, hf, fh1 int
	score       string
	thr         int
	t, alloc    float64
	tag, pv     string
}

func parseMLine(s string) (mLine, error) {
	m := mLineRe.FindStringSubmatch(s)
	if m == nil {
		return mLine{}, fmt.Errorf("logmine: unparsed m-line %q", s)
	}
	var out mLine
	var err error
	if out.move, err = strconv.Atoi(m[1]); err != nil {
		return mLine{}, fmt.Errorf("logmine: move %q: %w", m[1], err)
	}
	out.side, out.cell = m[2], m[3]
	if out.depth, err = strconv.Atoi(m[4]); err != nil {
		return mLine{}, fmt.Errorf("logmine: depth %q: %w", m[4], err)
	}
	if out.nodes, err = parseCompactCount(m[5]); err != nil {
		return mLine{}, err
	}
	if out.nps, err = parseCompactCount(m[6]); err != nil {
		return mLine{}, err
	}
	if out.ebf, err = strconv.ParseFloat(m[7], 64); err != nil {
		return mLine{}, fmt.Errorf("logmine: ebf %q: %w", m[7], err)
	}
	ints := []*int{&out.tt, &out.hf, &out.fh1}
	for i, slot := range ints {
		if *slot, err = strconv.Atoi(m[8+i]); err != nil {
			return mLine{}, fmt.Errorf("logmine: permille %q: %w", m[8+i], err)
		}
	}
	out.score = m[11]
	if out.thr, err = strconv.Atoi(m[12]); err != nil {
		return mLine{}, fmt.Errorf("logmine: thr %q: %w", m[12], err)
	}
	if out.t, err = strconv.ParseFloat(m[13], 64); err != nil {
		return mLine{}, fmt.Errorf("logmine: t %q: %w", m[13], err)
	}
	if out.alloc, err = strconv.ParseFloat(m[14], 64); err != nil {
		return mLine{}, fmt.Errorf("logmine: alloc %q: %w", m[14], err)
	}
	out.tag, out.pv = m[16], m[17]
	return out, nil
}

func parseCompactCount(s string) (uint64, error) {
	mult := uint64(1)
	body := s
	switch {
	case strings.HasSuffix(s, "k"):
		mult, body = 1_000, strings.TrimSuffix(s, "k")
	case strings.HasSuffix(s, "m"):
		mult, body = 1_000_000, strings.TrimSuffix(s, "m")
	case strings.HasSuffix(s, "g"):
		mult, body = 1_000_000_000, strings.TrimSuffix(s, "g")
	}
	v, err := strconv.ParseFloat(body, 64)
	if err != nil {
		return 0, fmt.Errorf("logmine: count %q: %w", s, err)
	}
	return uint64(v * float64(mult)), nil
}
