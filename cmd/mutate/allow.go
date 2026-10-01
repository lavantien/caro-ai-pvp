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

func (a allowlist) clone() allowlist {
	out := make(allowlist, len(a))
	for k, v := range a {
		out[k] = v
	}
	return out
}

// unusedAllowError reports entries that matched no surviving mutant, so a
// killed or stale allowance fails the gate instead of rotting silently.
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
