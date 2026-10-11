# server mutation triage, tierrescale2 gate

232 surviving mutants in internal/server, read line by line against the source. Classes: equivalence (EQ, proof cites the readers and the domain), killing-test (KT, test intent names input, call path, observable, and the file it belongs beside), refactor-flag (RF, simplify the code the mutant exposed). Column positions were verified against the working tree with a scripted mapping pass, not assumed from the log.

Abbreviations for the shared harnesses: fake-eng is a searcher stub injected through RoomManager.makeSearcher (or Room.makeSearcher white-box), recording-eng is a fake-eng that records calls, count-kdf is the counting argon2 stand-in in auth_test.go, log-cap is log.SetOutput capture, store-err is the failing-store seam in storefailure_test.go/httpfailure_test.go.

## match.go, 41 KT, 12 EQ (53 total)

- 81:12 || with &&: KT. Create a human room, start the series, Close it (over=true, finished=false), then PlayMove(host, cell). Original returns ErrRoomClosed, mutant falls through to ErrNotReady. Room lifecycle test, beside roomedge_test.go.
- 84:51 0 with 1 and 0 with -1: KT. PlayMove(0, cell) on a guestless room: original ErrNotParticipant, mutant skips the participant gate and returns ErrNotReady. Same test covers both, room_test.go.
- 144:19 0 with -1: KT. Room with seriesID==0 (in-memory test room): complete a game, assert the write queue and store see zero ApplyCompletion calls (counting store). Mutant persists a unit with seriesID 0. matchlifecycle_test.go.
- 147:54 1 with 2: KT. Complete game 1 of a bo3, read the persisted Game: IdxInSeries must be 0, mutant writes -1. matchlifecycle_test.go.
- 177:12 || with &&: KT. Close an unfinished room, Forfeit(host): original ErrRoomClosed, mutant proceeds to the participant or forfeit path. roomedge_test.go.
- 181:51 0 with 1 and 0 with -1: KT. Forfeit(0) on a guestless room: original ErrNotParticipant and the room untouched, mutant passes the gate and retires the room through the series==nil branch. room_test.go.
- 181:53 || with &&: KT. Same input, the gate folds to (guest==0 && userID!=guest) which is false for userID==0. Killed by the same Forfeit(0) test.
- 197:15 true with false: KT. After Forfeit by a participant, assert r.finished==true (white-box) or that a concurrent PlayMove in the finished-before-over window is refused; the deterministic kill is the white-box flag assert. matchlifecycle_test.go.
- 204:19 0 with -1: KT. Forfeit a seriesID==0 room: no store call (counting store), mutant sends ApplyCompletion. matchlifecycle_test.go.
- 214:62 2 with 3: KT. Forfeit mid-game with 5 live moves: persisted FullTurns must be 2, mutant writes 1. matchlifecycle_test.go.
- 218:46 - with + and 218:48 1 with 0: KT. Forfeit with synthetic games: Game.IdxInSeries must equal GameNo-1. matchlifecycle_test.go.
- 285:42 / with *, 285:43 2 with 1 and 2 with 3: EQ. DecodeMoves' make() capacity hint only. The loop appends and the slice is returned by value, the decoded moves and the error surface are byte-identical for every input.
- 287:53 8 with 9: KT. DecodeMoves([]byte{0x05, 0x01}) must fail with "cell 261 out of board", mutant reports 517. The error string is API. match_test.go decode table.
- 322:4 continue with break: KT. Board with an occupied cell at A1 and two separate win-in-1 points for red: WonByTag must return "double 4", the mutant stops the scan at the first occupied cell and returns "4". match_test.go.
- 391:25 0 with 1 and 0 with -1: EQ. grant's initializer: every read (401, 410, 414, 419, 423) is gated by hasGrant, and hasGrant=true is assigned together with grant=bg.Budget() at 394, so the initial value is dead.
- 406:4 continue with break: KT. solverSearcher{inner: fake, vct: vcf.New(KindVCT)} on a VCT-winnable position with a budget above SolverMinGrantMs: Search must return tag BotLogTagVCT, the mutant aborts the pass loop and falls to inner. solversearcher_test.go.
- 410:88 - with +: KT. Budget accounting: fake-eng grant just above SolverMinGrantMs on a position whose VCF pass burns the grant, VCT has a fast proof: original starves the VCT share (grant-elapsed <= 0) and falls to inner, mutant inflates the share and returns the proof. Timing-sensitive, solversearcher_test.go.
- 418:5 negate condition: KT. Search with a non-Budgeter deadline (plain wall deadline) and nil solvers: original passes dl to inner which searches (Nodes in the thousands under a 300ms deadline), mutant wraps NewFixedBudget(-elapsed) and inner returns immediately with ~0 nodes. solversearcher_test.go.
- 419:38 - with +: KT. Same harness with hasGrant: recording-eng asserts the inner budget stays at or below grant after a slow first pass. Timing-sensitive, solversearcher_test.go.
- 442:23 negate condition and 442:25 != with ==: KT. log-cap around solverSearcher.StartPonder plus StopPonder on a normal position: no "ponder solver pass aborted" line may appear. Mutants fire the recover body on normal completion (or suppress it on a real panic from a panicking fake solver). botsearcher_test.go.
- 457:41 0 with 1 and 0 with -1: EQ. proofStats.AllocNs is dead in transit: the only reader of a halted ponder result is collectPonder, which overwrites st.AllocNs=budgetNs at ponder.go:269 before anything renders it, and closeEnginesLocked discards the halt result entirely.
- 505:5 negate condition: KT. NewBotSearcher(hard).Search on any position: st.Threads must equal hard.Cores (SMP), the negate puts a single engine behind it reporting 1. botsearcher_test.go.
- 505:13 > with >= and 505:15 1 with 0: KT, conditional. The Cores==1 boundary (easy tier) swaps engine.NewTiered(tier) for engine.New. If any SearchStats field differs between a 1-worker tiered engine and the single engine, assert it, otherwise these two are implementation-choice equivalence and belong in .mutate-allow with that proof. botsearcher_test.go.
- 505:15 1 with 2: KT. NewBotSearcher(medium).Search: st.Threads must be 2, the mutant builds the single engine. botsearcher_test.go.
- 584:12 and 584:26 || with &&: KT. White-box Room{finished:true, over:false, quit open, board live, guest bot}: botTurnLocked() must return (0,false). The mutants fold finished into an && and let the bot move inside the completeGameLocked window between finished=true and retire's over=true. bot_test.go.
- 585:10 0 with 1 and 589:10 0 with 1: EQ. The color half of botTurnLocked's (0,false) returns: both call sites (wakeBotLocked 575, runBotTurn 595) discard the color when ok is false.
- 601:17 > with >=, 601:19 0 with 1 and 0 with -1, 601:21 && with ||, 601:31 > with >=: KT. budgetCap==0 is the default for every human-created room and must mean uncapped: bot room with no cap set, first bot move must produce st.Nodes > 0 (mutant >=, ||, -1 clamp the budget to 0 or 1ns). The 601:19 1 variant: white-box r.budgetCap=1ns must collapse the search to ~0 nodes. bot_test.go.
- 618:12 and 618:26 || with &&: KT. White-box Room{finished:true, over:false, quit open} with recording-eng: runBotTurn must return false without applying a move, mutants apply a move to a finished room. bot_test.go.
- 618:44 || with &&: KT. White-box retire window (quit closed under lock, over still false): runBotTurn must not apply, mutant applies a post-Close move. bot_test.go.
- 618:62 || with &&: KT. recording-eng whose Search plays a human move on the room under the lock mid-search: runBotTurn must discard the stale move (r.moves unchanged), mutant applies it and corrupts the game. bot_test.go.
- 618:96 || with &&: EQ. The deciding state is r.board.MoveCount==moveCount with r.board.Side flipped. Every legal interleaving changes side only through applyMoveLocked (increments MoveCount first) or startGameLocked (resets both, and equal MoveCount would require the old game at 0 moves while a bot search ran, which the 618 guard's own snapshot excludes). Side cannot differ at equal MoveCount, so the folded condition never fires.
- 620:10 false with true: EQ. The return value only steers botLoop between retry-now and park-on-wake. On every discard path the parked retry is already triggered (wakeBotLocked buffered a wake exactly when the next runBotTurn would act, quit is closed when it would not), and the extra maybeFinish is gated on over||finished and its finishLifecycle is idempotent.
- 642:10 true with false: KT. Bot-vs-bot series whose final game is a draw (board full): the room must retire (rm.Get returns ErrRoomNotFound or over==true). The mutant parks botLoop without running maybeFinish, the room leaks in the manager. botvsbot_test.go with fake-eng.
- 649:9 true with false: EQ. After a normal bot turn, wakeBotLocked at 645 has buffered a wake exactly when the next runBotTurn would act (opponent is a bot), so parking on select and continuing converge on the same next action, and maybeFinish is a no-op mid-game.
- 653:10 0 with 1: KT. Board with exactly one empty cell, recording-eng returns an illegal move: runBotTurn applies the only legal move, the mutant panics on n==1. bot_test.go.
- 653:10 0 with -1: KT. White-box firstLegalLocked on a full board must panic (the invariant guard). bot_test.go.
- 659:9 != with ==: KT. log-cap: logBotCompletion(nil) logs nothing (mutant logs a nil failure line), logBotCompletion(err) logs the line. matchlifecycle_test.go with store-err.

## ui_tourney.go, 40 KT, 9 EQ (49 total)

- 107:45 10 with 9: KT. TourneyBlockedError{RunID: 9}.Error() must read "run 9 is still in progress...". Mutant renders 9 in base 9 as "10". ui_tourney_test.go.
- 190:33 != with ==: KT. POST a form with a valid tier for row 0 and an invalid tc: the re-rendered rows must carry the posted tier selected. Mutant drops every posted tier. ui_tourney_test.go.
- 230:47 0 with 1 and 585:46 0 with 1: EQ. time.Unix(sec, 1) shifts the instant by 1ns, historyTimeFormat ("2006-01-02 15:04", ui_shell.go:49) renders whole minutes, and a 1ns shift cannot cross a minute boundary.
- 244:68 - with +, 244:70 1 with 0 and 1 with 2: KT. renderSetup's parallel options: values and labels must both run 1..TournamentParallelMatches. The mutants corrupt the labels ("-1,0,.." or "0,1,.."). ui_tourney_test.go golden.
- 247:34 0 with 1: KT. make([]optionView, 1, ...) prepends an empty option row, the count select must start at 2 with no phantom row. ui_tourney_test.go.
- 247:69 - with + and 247:70 1 with 2: EQ. Capacity hints on the countOpts make(), the appended set and its rendering are unchanged.
- 248:11 2 with 1 and 2 with 3: KT. The participant count options must be exactly 2..Max. Mutants emit a 1 option or drop the 2. ui_tourney_test.go.
- 294:57 negate condition and 294:61 != with ==: KT. bad() on a non-numeric rating field: the re-rendered form must keep the posted state, mutants clobber state.rating to the Atoi zero. ui_tourney_test.go.
- 300:16, 300:29, 305:16, 310:16, 310:63, 317:16, 322:16, 322:29 || with &&: KT. One boundary table in ui_tourney_test.go: POST tc=abc (expect the time-control error), tc=-1, tc=len(TimeControls), bo=abc, rating=abc, rating=AbsMax+1, parallel=abc, count=abc, count=1. Every mutant lets its field's garbage value through the gate. handleStart validation.
- 300:27 0 with -1: KT. POST tc=-1: original 400 "pick a time control from the list", mutant forwards -1 to StartRun. Same table.
- 300:38 >= with >: KT. POST tc=len(config.TimeControls): must 400, mutant passes an out-of-range index. Same table.
- 317:44 > with >= and 322:38 > with >=: KT. POST parallel=TournamentParallelMatches and count=TournamentMaxParticipants with an otherwise valid form: must not produce the range errors. Same table.
- 322:27 2 with 1: KT. POST count=1: must 400, mutant accepts. Same table.
- 328:15 0 with 1 and 0 with -1: EQ. worstRoom's initializer: count is validated >= 2, so the loop takes at least two max() steps over tier.Cores and RoomCores values that are all >= 1 (config tiers 1..8, RoomCores = max(Cores+PonderThreads,..) >= 1), and max(1 or -1, >=1) lands on the same final value.
- 333:31 - with +, 333:32 1 with 0 and 1 with 2, 347:31 - with +: KT. Empty name or missing tier for seat 1: the inline error must read "bot 1 needs a name" / "bot 1 needs a tier from the list". Mutants renumber the seat. ui_tourney_test.go.
- 336:16 > with >=: KT. A name of exactly TournamentNameMaxBytes bytes must be accepted. Same table.
- 357:40 > with >=: KT. parallel*worstRoom == MachineCores (2 parallel rooms at 8 cores) must be accepted: the machine budget is a ceiling, not an exclusive bound. Same table.
- 388:5 negate condition and 388:9 != with ==: KT. Seed one run, POST an invalid setup: the re-rendered error page must still list the existing run line. Mutants return nil from bestEffortRuns on the success path or the error path. ui_tourney_test.go.
- 429:32 0 with -1: EQ. For a running tourney with zero live boards the mutant returns an empty non-nil slice instead of nil. The only readers are template conditionals and ranges over .Live, both falsy and iteration-free for nil and empty alike.
- 530:63 10 with 9 and 10 with 11: KT. Close a stalled run with runID 9: the Location header must be /tourney/run/9. Mutants render the id in base 9 or 11. ui_tourney_test.go.
- 547:49 10 with 9 and 10 with 11: KT. GET /tourney/run/9 on a run whose id is 9: must resolve (then 404 downstream on a missing run), mutants fail the base-9/11 parse and 404 at tourneyRunID. Direct table on tourneyRunID with "9" suffices. ui_tourney_test.go.
- 547:53 64 with 63: KT. Weak but concrete: id "4611686018427387904" (above 2^62-1, below 2^63) with store-err on the viewer read: original reaches resolveViewer and answers 500 "tournament read failed", mutant 404s at the parse. ui_boundary_test.go.
- 550:10 0 with 1 and 0 with -1: EQ. tourneyRunID's id half on the !ok path: both callers (handleRun 467, handleCloseRun 497) return on !ok before reading the id.
- 550:13 false with true: KT. GET /tourney/run/abc: exactly one 404 page body. The mutant returns ok=true after writing the 404 and handleRun renders the run page on top (superfluous WriteHeader, doubled body on the recorder). ui_tourney_test.go.
- 560:30 false with true and 563:29 false with true: KT. snapshot() on ErrNotFound (and on a generic read failure with a fake failing tourney service): exactly one 404 (or one 500) response, no page rendered after it. ui_tourney_test.go with the TourneyService seam.

## ponder.go, 20 KT, 13 EQ, 1 RF (34 total)

- 30:10 1 with 2: EQ. The ref sentinel in ReferenceDepth: depths enter only via Append(budget, st.Depth) with engine depths >= 0, and every depth compares > both -1 and -2, while the final ref<0 check rejects both sentinels identically.
- 34:22 > with >=: EQ. ref is a running max over matching slots; on a tie the mutant reassigns the same value, the returned max is unchanged for every input.
- 39:9 < with <= and 39:11 0 with 1: KT. ring.Append(g, 0) then ReferenceDepth(g): must return (0, true). Mutants treat a matched depth-0 entry as no reference and push AdoptPonder into the elapsed fallback. ponder_test.go ring table.
- 60:46 1 with 2: EQ. The stability loop ranges i over [n-PonderStableIters, n) with PonderStableIters=3 >= 2, a window containing both n-1 and n-2. "all window entries equal the last" and "all equal the second-to-last" are the same predicate because each forces last==second-to-last.
- 65:10 < with <=, 65:12 0 with 1 and 0 with -1: EQ. drop is clamped for the margin test at line 68. <= and <1 admit drop==0 where -0==0 is identity. <-1 releases only drop==-1, whose mutant value -1 and original value 1 both fail drop > PonderScoreDropMargin=300; every other negative is clamped by both.
- 181:16 != with ==: KT. Fake searcher whose StopPonder panics: ponder.stop(side) must still return (collectPonder's <-done is closed by dispatch's recover). The mutant closes done only for ponderClose commands and the stop hangs forever. ponder_test.go with a timeout guard.
- 217:6 negate condition, 217:15 != with ==, 217:30 && with ||, 217:42 != with ==: KT. abort() protocol: queue a start with a panicking engine, then close(). Original closes the pending non-close cmds' done channels and closes closeDone exactly once. Negate and == starve the stop waiters (hang), || double-closes closeDone through the pending close cmd (panic). ponder_test.go.
- 238:10 0 with 1, 242:10 0 with 1, 259:10 0 with 1: EQ. The move half of collectPonder's (0, stats, "", false) rejections: runBotTouch's caller reads the tuple only when adopted is true (match.go:613).
- 238:39 false with true: EQ. The r.ponder==nil branch: collectPonder's sole caller is runBotTurn, which only runs on the botLoop goroutine, and startBotWorker assigns r.ponder before spawning botLoop. No writer nils it afterwards, so the branch is unreachable through every construction path.
- 245:8 0 with 1 and 0 with -1: EQ. mc's initializer: overwritten whenever r.board != nil, and board==nil requires pres.active==false (ponder arming needs a live board, nothing nils the board after the first game), so the mutated value only flows into the line-255 rejection, which fires for both sentinels anyway (mc==0 is separately rejected by mc != base+1 since base >= 0).
- 250:10 0 with 1: KT. Arm ponder at base 0 with predict != 0 (fake stats passing AdoptPonder), play the predicted move: collectPonder must adopt at mc==1. The mutant skips reading moves[0] and rejects. ponder_protocol_test.go.
- 255:11 0 with 1: KT. Same scenario: adoption at mc==1 must succeed, the mutant's mc==1 disjunct rejects it. ponder_protocol_test.go.
- 255:11 0 with -1: EQ. mc >= 0 always; the only deciding value would be mc==0 accepted by the mutant, which needs base==-1, unreachable since arm.base = board.MoveCount >= 0, and mc==0 is rejected by the next disjunct regardless.
- 255:13 || with &&: KT. Stale ponder: arm at base 5, play two moves (mc==7) with last==predict and adoption-passing stats: collectPonder must reject (mc != base+1), the mutant adopts a stale search. White-box room plus recording-eng. ponder_protocol_test.go.
- 288:17 || with &&: RF. tier==nil is impossible at the only call site (runBotTurn invokes ponderArmLocked for the side whose seat bot botTurnLocked just verified non-nil), so the mutant can only differ by panicking in a state no construction reaches. Simplify: drop the dead tier==nil disjunct (or keep it and allow-list as equivalence with this proof).
- 288:39 < with <=, 288:41 1 with 0 and 1 with 2: KT. Tier matrix: medium (PonderThreads=1) must arm ponder after a bot move (<= and 2 reject it), a PonderThreads=0 tier must not arm (0 admits it). recording-eng StartPonder call count. ponder_test.go.
- 301:13 || with &&: KT. Room with ponderOn=false (bot-vs-bot at floor cores): dispatchPonderArm must never call ponder.start. The mutant skips the !arm.ok guard and starts a zero-value arm (nil engine into the lane). ponder_protocol_test.go with recording-eng.
- 309:21, 309:36, 309:55, 309:73, 309:106, 310:39 && with ||: KT. One white-box dispatchPonderArm state matrix in ponder_protocol_test.go, one mutant per row, each asserting ponder.start is not called: over=true (quit closed), finished=true over=false (completion window), quit closed over=false (retire window), board=nil (the 309:73 row also panics on the nil deref), board moved past arm.base, engines[side] swapped for another instance. Every mutant arms (or panics) in its row's state.

## room.go, 19 KT, 4 EQ, 1 RF (24 total)

- 30:21 16 with 15 and 16 with 17: KT. newRoomID must return 32 hex chars (2*16); isRoomID (web.go:140) pins the length, so 15/17 also breaks every room URL. room_test.go length assert.
- 32:26 2 with 1 and 33:26 3 with 4: KT. CreateBotVsBot room Info(): HostUserID must be -3 and GuestUserID -2, and the synthetic games persist these ids. room_test.go / botvsbot_test.go.
- 58:20 0 with 1: KT. Create(ownerUserID=1, ...) must succeed, mutant answers ErrBadOwner. room_test.go boundary row.
- 78:29 1 with 0 and 1 with 2, 126:29 1 with 2: KT. Wake coalescing: white-box, board live with a bot to move, wakeBotLocked with no receiver parked, then drain r.wake: exactly one token (buffer 1). Buffer 0 loses the token (a wake signaled while the bot is mid-turn must not be lost), buffer 2 stores two. bot_test.go, second row via the bot-vs-bot constructor.
- 80:15 false with true and 128:15 false with true: KT. Create/CreateBotVsBot with store-err on CreateSeries: LiveCoreBookings must return 0 after the failure (the deferred release ran), and a second identical create must reach the store again. Mutants leak the booking and the retry answers ErrMachineBusy. storefailure_test.go / roomedge_test.go.
- 125:44 true with false: RF. CreateBotVsBot's Room literal sets ponderOn=true, then the core-booking ladder (136-148) assigns r.ponderOn on every path before the first reader (startBotWorker at 163). The initializer is dead; drop it from the literal and let the ladder own the field.
- 136:10 > with >= and 145:16 false with true: KT. Tier pair easy+medium: RoomCores = max(1+1, 2+0) = 2 == floor = max(1,2), so the original takes the else branch with ponderOn=false. recording-eng: no StartPonder may occur for the medium seat. Mutants enable pondering without booked cores. botvsbot_test.go.
- 266:12, 266:26, 266:47, 266:69, 266:88 || with &&: KT. liveBotBoard guard matrix, one row per operand with exactly that operand true and the next false: over=true finished=false (room closed via Close, then BotBoards must not list it), finished=true over=false, host bot nil with guest set, guest bot nil, series nil with board set. room_test.go / botstats_test.go.
- 293:33 2 with 1 and 2 with 3: EQ. Capacity hint on BotBoards' out slice, the appended set and its sort are unchanged.
- 300:50 0 with -1: KT. BotBoards ordering: two rooms whose CreatedAt order and ID order disagree (white-box crafted rooms): CreatedAt must win. The mutant falls to ID comparison for the newest-first pair. botstats_test.go.
- 346:16 2 with 3 and 348:16 2 with 3: EQ. clock and engines are indexed only by rules.Red=0 and rules.Blue=1 across the package; the one range reader (closeEnginesLocked 527) nil-guards every slot, so the always-nil third slot is never observed.
- 439:10 true with false: KT. The retire window: close(r.quit) under the lock without setting over, board live with a bot to move: botTurnLocked() must return (0,false). Covers exactly the gap between close(quit) and over=true where quitClosed is the only live guard. bot_test.go white-box.

## ui_room.go, 12 KT, 5 EQ (17 total)

- 92:53 10 with 9: KT. Playback of game id 19 (seed 19 tiny games via ApplyCompletion): GET /rooms/history/19 must render. Mutant's base-9 parse rejects the "9" digit and 404s. ui_room_test.go.
- 92:57 64 with 63: KT. Weak: gameID above 2^62-1 with store-err on the viewer read: original reaches viewerOf and answers 500, mutant 404s at the parse. ui_boundary_test.go.
- 93:16 || with && and 93:22 <= with <: KT. GET /rooms/history/0 with store-err on the viewer read: original 404s before any store touch, mutant runs viewerOf and answers 500. Same boundary file.
- 301:24 and 301:36 && with ||: KT. Guest (no cookie) opens a joined, still-created room: the ready controls must not render (the handshake section is participants-only). Mutants widen the guard to guests. ui_room_test.go golden.
- 314:62 0 with 1: KT. Live game with distinct red and blue banks: the page's red clock line must equal formatClockMs of red's remaining. Mutant renders blue's bank as red's. ui_room_test.go.
- 319:7 negate condition: KT. Red participant views a live game: MyColor must be "red" (board CSS class / data attribute). Mutant inverts the viewer's color. ui_room_test.go.
- 353:19 false with true: KT. Request the room page with a garbage session cookie: dead sessions are guests, no join button and no acting controls may render (CanJoin stays false with ok=false). Mutant treats the zero User as logged in. ui_room_test.go.
- 375:27 0 with -1: KT. Open human room: the guest seat must render "waiting for opponent". Mutant falls to userName(0), misses, and renders "opponent". ui_room_test.go golden.
- 403:17 and 403:24 false with true: EQ. readyFlags' hostReady/guestReady halves on the series==nil path: the only reader (roomViewOf:303) renders them behind the seriesOK gate, which is the exists flag that same tuple sets false.
- 424:18 == with !=: KT. boardCells with three moves: exactly the third cell carries Latest. Mutant flags every move but the last. ui_room_test.go boardCells unit.
- 547:8 < with <= and 547:10 0 with 1 and 0 with -1: EQ. The clamp: <= and <1 admit ms==0 where ms=0 is a no-op; <-1 releases only ms==-1, and formatClockMs(-1) renders "00:00.0" exactly like the clamp (secs=-1/1000=0, (-1%1000)/100=0 under Go truncation), so the output string is identical for every input.
- 548:8 0 with 1: KT. formatClockMs(-1) must be "00:00.0", the mutant's clamp target renders "00:00.1". ui_room_test.go table.

## store.go, 2 KT, 10 EQ (12 total)

- 72:36 != with ==: KT. Open(path in a not-yet-existing directory): original creates the parent, mutant skips MkdirAll and Open fails. store_test.go tempdir row.
- 146:6 negate condition: KT. Hold an open read transaction on a second connection to the same file, then Close(): the WAL checkpoint goes busy and the original surfaces ckErr, the mutant swallows it (Close returns nil with an untruncated WAL). storefailure_test.go.
- 158:24 4 with 3 and 4 with 5: EQ. Capacity hint on the url.Values make, the DSN string is unchanged.
- 231:18 and 235:18 false with true: EQ. CreateUserIfAbsent's created flag on error paths: the sole caller (loginOrCreate:167) returns on err before reading created.
- 319:10 0 with 1 and 0 with -1, 327:10 0 with 1 and 0 with -1: EQ. BotAccountID's id half on error paths: both readers (Create:63, verifyBotSeats:131) return on err before reading the id.
- 564:22 2 with 3: EQ. anyBotSeat's extra probe for id 0: users.id is the INTEGER PRIMARY KEY rowid, always >= 1, so the third iteration always misses with ErrNoRows and the verdict is unchanged.
- 633:10 0 with -1: EQ. currentRatingOn's value on the error path: both callers (applyRatingPair, currentRating's consumers) return on err first.

## httpapi.go, 12 KT (12 total)

- 26:23 64 with 63 and 64 with 65, 26:29 10 with 9 and 10 with 11: KT. One body-size boundary table on POST /api/login: a valid JSON body of exactly 65536 bytes must decode (63<<10 and <<9 truncate it into "malformed login body"), a body of exactly 66560 bytes must be refused (65<<10 and <<11 decode it). httpapi_test.go.
- 120:5 negate condition and 120:12 != with ==: KT. writeDomainError: a 500 (store-err) must answer message "internal error" (mutants leak the raw store error), a domain error like ErrRoomFull must answer the sentinel text (mutants mask it). httpfailure_test.go.
- 195:22 15 with 14 and 15 with 16: KT, slow. SSE stream on a live room through the default NewHTTPAPI handler (no keepalive field override): after the headers, the first keepalive comment must arrive in (14.5s, 15.5s). Kills 14 and 16. The cleaner long-term fix is hoisting the cadence into config beside PagePollMs so tests stop needing wall-clock windows. httpapi_test.go.
- 195:25 * with /: KT. 15/time.Second truncates to 0 and time.NewTicker(0) panics: any request to /api/rooms/{id}/events on the default handler must answer 200 with a stream, not blow up the handler. httpapi_test.go.
- 410:5 continue with break: KT. SSE keepalive tick: after the first keepalive frame on a still-live room the stream must stay open (a second keepalive or a later event arrives). The mutant breaks the loop after one tick and ends the stream with a plain EOF. httpapi_test.go with a tightened keepalive field.
- 567:59 1 with 2: KT. POST /api/logout: the Set-Cookie header must read Max-Age=-1 exactly. Mutant emits -2. httpapi_test.go.
- 567:72 true with false: KT. Same response must carry HttpOnly. Mutant drops the attribute. httpapi_test.go.

## stats.go, 5 KT, 4 EQ (9 total)

- 28:18 160 with 159: KT. MLine with a maximal line (fixed prefix plus 64 PV slots named P16, length exactly mLineCap=416): testing.AllocsPerRun(MLine)==1, beside the existing TestAppendMLineZeroAlloc harness. The mutant's 415 cap forces a second allocation.
- 28:18 160 with 161: EQ. The mutant cap 417 sits above the maximum renderable line (the fixed prefix is bounded by the format's widest fields, ~140 bytes, plus at most SearchMaxPly*4=256 bytes of P16 names = 416), so the wrapper never grows in either version and the output is byte-identical.
- 28:44 4 with 3: KT. Same maximal-line test: cap 352 grows on the long PV, allocs flip 1 to 2.
- 28:44 4 with 5: EQ. Cap 480 exceeds the same 416 maximum, never grows, identical output and allocation count.
- 63:50 10 with 9 and 10 with 11: KT. MLine golden with st.Threads=16: the line must read "thr=16". Base 9 renders "17", base 11 "15". stats_test.go golden table (the zero-alloc test already uses Threads=16 but never checks the text).
- 112:7 >= with >: EQ. At n==compactMega the k-scale path yields whole=1000, frac=0, which triggers the promotion branch (116-119) and recomputes with the mega scale to (1, 0), the exact pair the original computes. All other n choose the same scale under both operators.
- 125:18 < with <= and 125:20 10 with 11: EQ. The else-if is reached only when frac%10 != 0, and the only value the wider guards additionally admit is frac==10, which always has frac%10==0 and is diverted by the first branch. On the reachable domain {1..9, 11..99 sans multiples of 10} both predicates decide identically.

## ui_shell.go, 6 KT, 2 EQ (8 total)

- 254:35 8 with 7 and 8 with 9: KT. Home grid with one live room: the card's short id must be the first 8 hex chars. ui_shell_test.go golden.
- 274:6 negate condition and 276:18 true with false: KT. Grid cards: a SeriesCreated room shows no score line, a room past created shows "0-0". Mutants invert or suppress the score. ui_shell_test.go.
- 443:59 1 with 2: KT. POST /logout (shell): Set-Cookie must read Max-Age=-1 exactly. ui_shell_test.go.
- 443:72 true with false: KT. Same cookie must carry HttpOnly. ui_shell_test.go.
- 546:37 0 with 1 and 0 with -1: EQ. Same 1ns-vs-minute-granularity argument as ui_tourney 230:47: historyTimeFormat renders whole minutes.

## hub.go, 1 KT, 4 EQ (5 total)

- 64:40 1 with 0 and 1 with 2: EQ. Map capacity hint on the per-room subscriber set; no reader observes capacity.
- 78:17 0 with -1: EQ. len() is never negative, so the mutant only removes the empty-set fast path: with set==nil (room absent) the range is empty and the delete of an absent key is a no-op, and h.rooms never holds an empty set anyway (Subscribe adds before returning, Publish and Unsubscribe delete on empty).
- 87:15 true with false: KT. Subscribe, publish HubSubscriberBuffer+1 events to evict, then Unsubscribe: must be a no-op. The mutant leaves closed=false, Unsubscribe proceeds and close()s the already-closed channel: panic. hub_test.go.
- 148:17 0 with -1: EQ. The room-entry delete in Unsubscribe: every h.rooms reader decides identically on empty-set-present versus absent (Subscribe's nil check reuses the empty set to the same effect, Publish's len check early-returns either way, Close iterates nothing). The delete is unobservable cleanup, though dropping it would leak map slots on long-lived hubs.

## auth.go, 5 KT (5 total)

- 84:25 1 with 0: KT. Row with Argon2Time=0 and otherwise in-envelope: verifyPassword must return false with zero kdf calls (count-kdf), the mutant calls the kdf and panics under the real argon2.IDKey (rounds < 1). auth_test.go envelope table.
- 86:26 < with <=: KT. Row with Argon2MemoryKiB == 8*Argon2Parallelism (argon2's legal floor) and a matching hash: VerifyPassword must return true, the mutant fails the row closed. auth_test.go.
- 86:28 8 with 7 and 8 with 9: KT. Boundary rows at parallelism 2: memory 14 KiB must be rejected pre-kdf (count-kdf sees 0 calls, real KDF would panic), memory 16 KiB with a matching hash must verify. auth_test.go.
- 86:29 * with /: KT. Row with parallelism 2, memory 10 KiB: original rejects pre-kdf (10 < 16), mutant's 10 >= 4 admits it and the kdf runs (count-kdf call count flips, real KDF panics). auth_test.go.

## web.go, 2 KT (2 total)

- 66:23 0 with 1: KT. GET /spike: the scratch button must carry data-tc="0" (spikeScratchTCIdx). web_test.go body assert.
- 143:11 0 with 1: KT. GET /spike?room=<uppercase F followed by 31 lowercase hex>: isRoomID must reject it and the page must fall back to the default room id. The mutant skips validating byte 0. web_test.go.

## writeq.go, 1 EQ (1 total)

- 52:27 1 with 0: EQ. Send's done channel loses its buffer: applyOne's send and Send's <-done form a plain rendezvous (applyOne runs on the worker goroutine, Send's receive follows its own enqueue on the caller goroutine in straight-line code), so the delivered error and the ordering are identical, only the worker's hold time changes.

## rating.go, 1 EQ (1 total)

- 62:13 > with >=: EQ. The mutant only decides differently at rWinner==rLoser, where K=10^0=1 and the delta is round(RatingDelta/d): d lies in [RatingDecayMin=500, RatingDivisorFull=3000] under both branches (the band clamps into that range), so 30/d is at most 0.06 and rounds to 0 either way. Both the deltas and the after-ratings are identical for every equal-rating input.

## out of scope

- internal/engine/smp.go:365:23 1 with 2 leaked into this gate's survivor list. Provisional read: res.seq is only ever compared (smp.go:311, strict <), never stored or differenced, and Add(2) preserves strict monotonic order, so it is likely equivalence, but it belongs to the engine package's own accounting and should be re-derived there, not allowed from this triage.

## summary

Totals: 165 killing-test, 65 equivalence, 2 refactor-flag, of 232 server survivors.
Files by survivor count: match.go 53, ui_tourney.go 49, ponder.go 34, room.go 24, ui_room.go 17, store.go 12, httpapi.go 12, stats.go 9, ui_shell.go 8, hub.go 5, auth.go 5, web.go 2, writeq.go 1, rating.go 1.
Kill first: match.go:601:17 (> with >=, the budgetCap gate: budgetCap==0 is every human room's default and the mutant zeroes every bot budget), room.go:80:15 (published false with true: a failed create leaks the core booking and the machine answers ErrMachineBusy forever), match.go:642:10 (true with false: a drawn final bot game never retires the room, botLoop parks and the room leaks in the manager), hub.go:87:15 (true with false: Unsubscribe after eviction double-closes the channel and panics), httpapi.go:120:5 (negate condition: 500 responses leak the raw store error text to the wire).
