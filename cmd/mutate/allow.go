package main

import (
	"fmt"
	"os"
	"sort"
	"strings"
)

// allowlist maps a mutant identity, formatted exactly like a report line up
// to but excluding the verdict ("file:line:col descriptor"), to the
// behavioral-equivalence proof that justifies suppressing its gate failure.
type allowlist map[string]string

// loadAllowlist parses path. An empty path or a missing file means no
// allowances. Every entry must be "<mutant key> # <reason>" with a non-empty
// reason; blank lines and lines starting with '#' are ignored.
func loadAllowlist(path string) (allowlist, error) {
	if path == "" {
		return nil, nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	out := allowlist{}
	for n, raw := range strings.Split(string(data), "\n") {
		line := strings.TrimSpace(raw)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, reason, ok := strings.Cut(line, " # ")
		key, reason = strings.TrimSpace(key), strings.TrimSpace(reason)
		if !ok || key == "" || reason == "" {
			return nil, fmt.Errorf("%s:%d: want entry 'file:line:col descriptor # proof'", path, n+1)
		}
		if _, dup := out[key]; dup {
			return nil, fmt.Errorf("%s:%d: duplicate allow entry %q", path, n+1, key)
		}
		out[key] = reason
	}
	return out, nil
}

// scopeAllows keeps only entries whose mutant file path starts with one of
// the prefixes, so a scoped gate (one package in CI) does not fail its
// unused-allow check over the packages it deliberately skipped. Empty
// prefixes keeps everything.
func scopeAllows(allows allowlist, prefixes []string) allowlist {
	if len(prefixes) == 0 {
		return allows
	}
	out := allowlist{}
	for k, v := range allows {
		for _, p := range prefixes {
			if strings.HasPrefix(k, p) {
				out[k] = v
				break
			}
		}
	}
	return out
}

// unconsumed returns the allowlist entries no executed mutant matched.
func unconsumed(allows, consumed allowlist) allowlist {
	pending := allowlist{}
	for k, v := range allows {
		if _, ok := consumed[k]; !ok {
			pending[k] = v
		}
	}
	return pending
}

// unusedAllowError reports entries that matched no executed mutant, so a
// stale or moved allowance fails the gate instead of rotting silently. In
// challenge mode a killed allowance is demoted explicitly before this
// check; without challenge, an entry is consumed on match and only moved
// or vanished mutants leave it unconsumed.
func unusedAllowError(pending allowlist) error {
	if len(pending) == 0 {
		return nil
	}
	keys := make([]string, 0, len(pending))
	for k := range pending {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return fmt.Errorf("%d unused allow entries (mutant killed, moved, or stale; prune the allowlist): %s", len(keys), strings.Join(keys, "; "))
}
