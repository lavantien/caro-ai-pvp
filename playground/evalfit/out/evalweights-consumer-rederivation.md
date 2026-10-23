# fitted eval weight consumer re-derivation

Derives every consumer row of playground/arena/out/evalweights-adoption-preregistration.md on the final fitted integers {0, 97, 122, 512, 513, 2077} for classes None, OpenTwo, BrokenThree, Three, Four, OpenFour, from playground/evalfit/out/evalfit-refit-gm.txt. No code changed; the literal swap at internal/config/config.go happens only inside the single adoption commit. Every constant below was read live at the cited lines. Live scaffold literals: internal/config/config.go:168-172 (100, 500, 1200, 4000, 15000), PatternClassWeights composition at internal/config/config.go:285, EvalTempo 1000 at internal/config/config.go:165.

## strict ascent

0 < 97 < 122 < 512 < 513 < 2077 holds. The Three and Four classes separate by exactly 1 milliunit; every consumer requires only order, never a margin. Pins: internal/config/config_test.go:529-537 asserts w[i] > w[i-1] for i >= 1 and w[0] == 0, internal/pattern/table_test.go:171-178 asserts length 6 and nondecreasing. Both are symbolic over the config symbols and evaluate true on the fitted integers; no rewrite at adoption.

## ordering interlock

orderScore returns static + history (internal/engine/movegen.go:98). static at internal/engine/movegen.go:65 is cell[own][c] + cell[opp][c], and each per-side cell sum accumulates one window per direction (internal/engine/eval.go:48-65), so static <= 2 * PatternDirections * PatternWeightOpenFour = 2 * 4 * 2077 = 16616. History halves at SearchHistoryMax = 1 << 20 (internal/engine/movegen.go:125-131), so the ordering layer bound is 2077 * 2 * 4 + 1048576 = 1065192 against SearchOrderKiller2 = 1 << 29 - 1 = 536870911 (internal/config/config.go:297). Holds with a 504x margin, and is stronger than the scaffold state (15000 * 8 + 1048576 = 1168576) because the fitted top class is smaller. Pin internal/config/config_test.go:558-560 is symbolic over the constants and evaluates true; no rewrite at adoption. SearchOrderTT and SearchOrderKiller1 relations at internal/config/config_test.go:561-563 are weight-independent.

## mate band

eval() = own - opp + tempo (internal/engine/eval.go:68-70). A per-side score sums every window class weight over 4 directions x 256 cells (internal/engine/eval.go:54-55), so each side is at most PatternDirections * BoardCells * PatternWeightOpenFour = 4 * 256 * 2077 = 2126848 and |own - opp| <= 4253696; adding tempo gives 4254696. The mate floor is EvalMateMax - SearchMaxPly * EvalMateScoreStep = 1073741824 - 64 * 16 = 1073740800 (internal/config/config.go:162-163, :175). Holds with a 252x margin, 7 times safer than the scaffold bound of 30720000. Pin internal/config/config_test.go:538-542 checks 2 * maxWindowSum < mateFloor symbolically; it omits the tempo term, and the tempo-inclusive bound above still clears the floor. All mate consumers read EvalMateMax and EvalMateScoreStep only: internal/engine/tt.go:23,37-50, internal/engine/search.go:67,230,269,389, internal/engine/smp.go:393, internal/server/match.go:512, internal/server/stats.go:148-166. Weight-independent.

## movegen forced restriction

Trigger at internal/engine/movegen.go:41-43: when the opponent of the mover holds a live four, minStatic = PatternWeightFour, and fill drops candidates under it (internal/engine/movegen.go:69-70). The fours counter tracks windows with class >= PatternClassFour per color (internal/engine/eval.go:58-63, :84-97). Classifier semantics: a window class >= Four iff the window holds at least one empty cell whose occupation completes an exact five in-window (n1 >= 1; Four is exactly 1 win cell, OpenFour 2 or more), derived in internal/pattern/table.go:73-90 and :120-143.

Superset proof on the fitted vector: an own win-in-1 cell y completes a five along some direction d, so the window centered at y in direction d has n1 >= 1 for the mover, class >= Four, and cell[own][y] >= min(513, 2077) = 513; a forced block x is the opponent's win-in-1 cell, same argument on cell[opp][x]. Every forced cell clears minStatic = 513 exactly through its Four window, so the admitted set stays a superset of the forced set, the node value is preserved, and the fallback stages at internal/engine/movegen.go:45-49 keep the never-empty property untouched.

Quiet crossings: a cell with no window of class >= Four sums at most 8 quiet windows (4 directions x 2 colors, each <= w Three = 512). Crossings of 513 now include Three + OpenTwo = 609, opponent Three + own BrokenThree = 634, two Threes = 1024, and in the extreme 8 x OpenTwo = 776; an isolated Three at 512 misses by 1. This is not a new class of admission: under the scaffold, 3 x Three + BrokenThree = 4100 >= 4000 admits quiet cells too (quiet maximum 8 x 1200 = 9600). The threshold only ever guarantees inclusion of the forced set, it never barred quiet cells under either weight set; the phrase "quiet cells sit strictly under it" at internal/engine/movegen.go:30 is loose under both weight sets and the load-bearing property is only the superset direction above. What shifts at fitted weights: more quiet cells clear the bar at forced nodes, candidate lists and node counts rise, and the relative order inside a list changes (a bare block scores 513 against quiet Three stacks up to 4096 that outrank it); search results stay correct because ordering is heuristic and the forced set is never dropped.

Pinned tests. TestForcedFourRestrictsCandidates (internal/engine/forced_test.go:27-41) holds on the fitted integers: in its board (red H7 and P16, blue H8-H11, internal/engine/forced_test.go:19-25) a column window contains the whole blue four iff its center row lies in [7,12], rows 7 through 11 are stones, so the only empty cell centering a class >= Four window is H12, whose static is exactly 513 at the bar; the strongest quiet static is H13's blue BrokenThree column window at 122 (placing blue H12 there yields a single in-window win cell H13, internal/pattern/table.go:107-112) plus OpenTwo crumbs, at most 413, far under 513. TestForcedFourKeepsEveryForcedCell (internal/engine/forced_test.go:47-77) is the general property and reads config.PatternWeightFour live at :72-73, adapting automatically. No rewrite at adoption.

## tempo cross-check

The recorded decision keeps EvalTempo = 1000 (internal/config/config.go:165, pin internal/config/config_test.go:526-528 against EvalMilliUnit). The ratio moves from 1000/15000 = 6.7 percent to 1000/2077 = 48.1 percent of the top class, and tempo now exceeds Three (512) and Four (513) individually while under the scaffold it sat below Three (1200). The consumer sweep finds no ratio or ordering encoding anywhere: internal/engine/eval.go:69 uses tempo additively; internal/engine/eval_test.go:78-79, :164-168, internal/engine/evalweights_test.go:75-84, :137-142, and internal/server/evalweights_test.go:37-60 read config.EvalTempo live and assert structural identities (empty board equals tempo, color swap negates around tempo, zero weights score equals tempo, default depth-1 score differs from tempo); playground/evalfit/fit.go:106 anchors the fit intercept at tempo, so the fitted vector is expressed around 1000 and the arena pair isolates the swap with tempo fixed. Behavior note only: the side-to-move term now outweighs any single sub-Four window, which the arena pair measures directly; no pinned invariant constrains tempo below any class.

## literal sweep

No test or consumer hardcodes the scaffold literals as expectations. The only 1200, 4000, and 15000 hits outside internal/config/config.go are synthetic fixtures deliberately independent of the shipped weights: playground/evalfit/fit_test.go:84,95 truth vectors, parse fixtures in playground/arena and playground/logstats, rank scores in playground/bookgen/tree_test.go:405,668. Every consumer pin is symbolic over the config symbols, so the adoption commit swaps the 6 literals at internal/config/config.go:168-172 and nothing else in tests, and every pin above re-evaluates true on {0, 97, 122, 512, 513, 2077}.

## adoption checklist

Swap the literals at internal/config/config.go:168-172 to 0, 97, 122, 512, 513, 2077 in one commit; PatternClassWeights at :285 composes automatically. No pinned inequality rewrite is required: config_test.go:526-528, :529-537, :538-542, :558-560, table_test.go:171-178, and the forced tests are all symbolic and hold on the fitted integers. Optional hardening in the adoption commit, mirroring the TestBookTreeConstantsMatchPreregistration style at internal/config/config_test.go:491-524:

```go
func TestPatternWeightsMatchAdoption(t *testing.T) {
	want := [PatternClassCount]int{0, 97, 122, 512, 513, 2077}
	if PatternClassWeights != want {
		t.Fatalf("PatternClassWeights = %v, want the adopted fitted integers %v", PatternClassWeights, want)
	}
}
```

Re-run at adoption per the prereg: eval, table, and forced tests, then the full 4-tier 4-TC re-gate ladder before any tag.
