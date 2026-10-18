# pikafish time management (zread summary)

source: https://zread.ai/official-pikafish/Pikafish/18-time-management (generated from src/timeman.cpp and src/search.cpp)
fetched: 2026-10-18 via WebFetch

## summary as fetched

Two layers: strategic TimeManagement::init() sets per-move budgets from UCI clock data; tactical layer adjusts/stops during iterative deepening. TimeManagement owned by main thread's SearchManager; holds optimumTime (soft), maximumTime (hard), startTime, availableNodes, useNodesTime; elapsed() polymorphic (wall clock or node count).

Allocation (mode 1, basetime + increment, movestogo == 0): mtg capped at 50; under 1 second mtg = scaledTime * 0.05. optScale is a log model of remaining time and ply times originalTimeAdjust computed once per game (0.3356 * log10(timeLeft) - 0.4903). maxConstant = max(3.7803 + 2.8003 * log10(scaledTime/1000), 2.5470); maxScale = min(7.002, maxConstant + ply/13.184). Mode 2 (x moves in y seconds): optScale = min((0.88 + ply/116.4)/mtg, 0.88*time[us]/timeLeft), 0.88 reserves a 12 percent margin; maxScale = 1.3 + 0.11*mtg. Final: optimumTime = max(1, optScale*timeLeft); maximumTime = max(optimumTime, min(0.8237*time[us] - moveOverhead, maxScale*optimumTime)). With default 10 ms Move Overhead no single move consumes more than about 82 percent of remaining time.

Nodes-as-time: availableNodes = npmsec * time[us]; npmsec must be much lower than real throughput or losses are inevitable; advance_nodes_time() subtracts consumed nodes after each search.

Dynamic adjustment after each ID iteration, four multipliers:
- fallingEval = (16.93 + 2.73*(prevAvg - best) + 0.8*(iterVal - best)) / 100, clamped [0.610, 1.860]; dropping score extends time.
- reduction from rootDepth - lastBestMoveDepth, range [0.670, 1.440]; stable best move for 17+ depths cuts budget to about 0.670.
- bestMoveInstability = 0.960 + 1.630 * totBestMoveChanges / threads, floor 0.960, aggregated across threads.
- highBestMoveEffort interpolated from nodesEffort in a 78k-94k permille range, [0.740, 0.960]; over 94 percent effort on best move shortens time.

Hard enforcement check_time(): main thread only, throttled to about every 512 nodes via callsCnt countdown. Three stop conditions: elapsed > tm.maximum(), elapsed >= limits.movetime, nodes_searched >= limits.nodes.

Ponder interaction: pondering grants optimumTime a 25 percent bonus. check_time() returns immediately while pondering, the engine never self-stops; only ponderhit or stop ends it. stopOnPonderhit: when the soft check says stop but ponder is active, sets stopOnPonderhit = true; the next ponderhit triggers the actual stop.

Move Overhead default 10 ms, subtracted as latency buffer; for sub-second games raising it beats tweaking engine parameters. use_time_management() true only if either clock is non-zero; movetime/nodes/infinite override.

Stop decision order by frequency: check_time hard stop (ignored during ponder); soft check after each iteration (stop if elapsed > min(totalTime, maximum), deferred via stopOnPonderhit during ponder); single root move -> immediate stop; mate found -> stop on mate-in-3 or being mated-in-2; go nodes / go depth limits.

System interactions: threads.stop atomic read by all workers; tt.new_search() right after tm.init(); aspiration re-searches can burn time, hence the hard cutoff.

## extraction notes

- The four soft multipliers are the whole "when to stop early" story; our clock PID controller is the analogous layer and these four signals (score falling, best-move age, cross-thread best-move churn, root effort concentration) are measurable in our engine too.
- stopOnPonderhit is the clean protocol answer to "search finished during ponder": keep searching past the soft limit, stop exactly on ponderhit if the soft limit had been crossed. Our ponder lane's adoption gate needs the same distinction (soft-limit-crossed ponder results are lower confidence than time-adequate ones).
