Our current engine, benchmark pipeline, PvP infrastructure, and UI/UX are a mess with scope creeps and full of bugs. We need to nuke the whole repo and rebuild all from scratch, from the ground up, from first principles, and bottom up. There is no need to archive or backup anything.

### First Principles Grounding
Without source attribution, save related reference topics offline to `./ref/` for repeated referencing during development.

### Engine Grounding

Use browser to save pages or intercept HTTP payloads, curl might get blocked by Cloudflare and captchas.

Reference `https://chessprogramming.org/` and source it offline to `./ref/` for repeated referencing. Start with basics & principlal topics, then get the full `https://chessprogramming.org/Stockfish` page and all its selected features, excluding NNUE.

Reference `https://gomocup.org/download-for-developers/` and source the docs and recommended source codes offline to `./ref/` for repeated referencing. Note that we won't follow their template or APIs, it's purely for educational and reference.

#### Things to focus
- Minimax Alpha-Beta / Principal Variation Search
- Transposition Tables with Zobrist Hashing with optimizations for power-of-two 16x16 board (four uint64 integers per player, or one __m256i AVX2 register per color / math/bits in go maybe?)
- Iterative Deepening
- Move Ordering (History Heuristic, Killer Moves)
- Lockless Hash Tables (SMP implemented and shared TT between threads for parallel search)
- Selective Threat Extension (only expanding forced 4-blocks, open 3s, ...)
- Pattern-Based Evaluation (open 4, closed 4, open 3, broken 3, open 2, ...)
- Bitboard convolution masks, vertical and diagonal lines can be checked using shearing/rotations
- SIMD bitwise shifts and masks across all 16 rows simultaneously, to compute pattern matching lines
- The search core must be 100% zero-alloc on the heap during search:
    - Pre-allocate move lists per ply e.g. `type MoveStack [MAX_PLY][MAX_MOVES]Move`
    - Pre-allocate TT buffers once at startup
    - Never slice dynamically inside the tree; use index pointer offsets
    - You must verify zero-allocation e.g. by running `go build -gcflags="-m"` and `go test -bench . -benchmem`. The benchmark must show `0 B/op` and `0 allocs/op` for the `Search()` function.

#### Things to avoid
- Quiescence Search
- Piece-Square Evaluation
- LMR / Null-Move Pruning
- float numbers for minimax evaluation inside hot search loop

#### Things to R&D with playground
- A-Star and Priority Queue
- Ant Colony Optimization
- Genetic Algorithm

## Game Rules

### Board and Variant
It is neither Gomoku nor Renju nor the usual Caro. This is a custom variant with a non-standard board size. The board size is 16x16 cells (A1 to P16) to allow for easy binary and algorithmic tricks.

### Turn Order and Balancing
Red O goes first, blue X goes second. In PvP best of series, the loser will take red and go first, but in benchmarks or bot tournaments, both players will alternately take red the same number of times.

### Opening Rule Restrictions
Red's second move must be physically greater than 2 cells distance in between from their first move, even if diagonal (Chebyshev distance >= 3, `max(|x2 - x1|, |y2 - y1|) >= 3`).

### Win Conditions
The condition is first to exactly continuous five in any straight line, even diagonal, without being close blocked by stones (wall boundary is not counted as blocked) at both ends. Close blocked means that in the same line, both blocking stones must be immediately next to both ends. Greater than five overlines don't count as a win, already being close-blocked at both ends doesn't count.

### Time Controls
1+0, 2+1, and 3+2. Best of series include bo3, bo5, bo7, and bo11.

### Bot Difficulties
The bot difficulty needs to relatively scale independently of the hardware so the relative difficulty between configurations will not be different regardless of environment. Never hardcode depth or nodes or NPS or anything to do with depth-based logic, we want to relationally maximize the usage of allocated resources. It needs to scale based on time remaining, so that when in increment it would never time out. The only acceptable result is a win, a loss, or a draw with a full board; never a timeout. Beside standard searches, we need dedicated VCF and VCT solvers that are exhaustively constructed via exhaustive combinatorial proofs (might be even use Lean programming language) on our exact specific ruleset, alongside with Proof-Number Search to strictly generate only threat moves (fours for VCF; threes and fours for VCT), and exhaustive line-pattern tables (enumerate every window of cells including edge-distance cases and classify its threats under our exact rule, this is a k^n state space so should be fully computable, the VCF/VCT searches use the tables, never hand-write patterns), cross-check solver results against brute-force alpha-beta on random positions and on a reduced board (say 8x8) with the same rules, soundness (a claimed win is never refuted) is provable by testing, completeness isn't. This is an absolute must and we need to find the optimal way to do it. It cannot be an arbitrary set or hard coded because you might be wrong or miss the nuanced overall picture, especially since our variant is non-standard and non-existent anywhere else, meaning we cannot rely on stale data or instructions online.

## Infrastructure and Guidelines

### Tech Stack (All Latest Only)
- Go 1.27.1+, take advantage of all latest features
- SQLite exclusively for v0.20, high TPS in WAL mode, later DuckDB queried directly against SQLite files
- HTMX 4+
- Web-Sockets/Sever-Sent Events
- Custom simple message/worker queue for WAL writes

### Hardware Budget
To fully utilize the hardware, it is safe to assume that the running machine will have at least 8 CPU cores, 8GB of RAM, and 8GB of disc space. The initial machine is 32GB RAM, 16 cores 20 threads CPU, 10GB VRAM, 1TB SSD. Because we will mostly test and benchmark with bot vs bot matches and leagues, each instance should maximumly allocate half of this resource, except disc space as it should grow naturally. Each instance of the engine (the bot) is idempotent and independent of each other. Nothing is shared at runtime between instances, not even instances of hash tables, to avoid anything leaking, contaminating, or influencing results. The engine has a different set of configurations for different difficulties.

### Ports Protocol
Avoid serving on common ports to prevent any possible conflict. Keep a script for the Windows firewall to easily allow these specific ports first.

### Dev Principles
Always rely exclusively on standard libraries and then google.org/x packages first as things that are composable with stdlib must be built with stdlib, never make up custom commands on the spot as anything that need to run need to be in the makefile, TDD, zero hard code, centralized constants and configs hub, KISS, first principles, and bottom up. Avoid abstractionization or complex patterns unless absolutely necessary. Be concurrence and parallel-native. What, why, how, where, and when must always be precisely explainable in this exact order in any decision. Any text or prose written must follow the writing guidelines and rules. Never assume or guess anything or rely on memory. Always base implementations on the latest verified data and double check everything using edge cases attack, e2e, screenshots, profiling, benchmarking, and adversarial blind attacks. Always write with the highest information density and simplicity while staying low verbosity and keeping the lowest noise possible. Use zero bluff or unnecessary comments, as the prose and code should speak for itself.

### Readme Requirements
The Readme will be the centralized place for all documentation work and graphs or diagrams. It should have a table of content and a coverage badge at the top. Enforce 95%+ overall and aim for 100% test coverage if possible, 100% on rules and engine core is non-negotiable, add mutation testing on the core. Include CI pass or fail badges for run tests on pushes. Additionally, provide 3 screenshots: first a room while a bot vs bot match is happening, second the home screen where you are logged in and can see the room grid and their info, and third the match history screen with an embedded playback board to completion taking visual priority.

## For v0.20 MVP

20 minor versions headroom

### Scenario 1

```text
UI/UX is mobile first (but responsive and can properly utilize big screen) (mobile setup might need tap-to-preview if the cells are smaller than expected) so that I can play on the phone browser with a bot or another user with the server hosted on my laptop (this is just a behavior description, external hosting is not a concern for this scope). The UI should be minimal but still show all necessary information and functioning, meaning maximal information density and minimal fluff.

If I am a new user visiting the site, I should see a simple login form with a username, password, and a "login/create" button. A simple username and serverside argon2id-hased password is enough, and there is no need for a forgot password function. When logged in they can see their name and rating. Rating starts at 0 and can go negative; each win nets 30*K, a loss subtracts 30*K, and a draw nets 0, where K = 10^((R_loser-R_winner)/3000), decay scale (0-200: /500, 201-400: /1000, 401-600: /1500, 601-800: /2000, 801-1000: /2500, 1001+: /3000) favors lower ratted player (the decay constant applies full 3000 if the winner was lower, and applies the scale based on loser's rating if the winner was higher) D=min(3000,max(500,500+2.5×Rloser​)). Quitting in the middle of a best of series equals a loss for the remaining games and deducts accordingly. There is no fraud detection logic for now. They will also see a field of total W-L-D numbers and their level, which will be the total number of best of series won. 

If they click on that W-L-D field it will bring them to their match history tab where each line shows the time of play, who vs who, score line so far in the best of series, the number of full-turns, and the move history. The move history shows the first 8 full-turns and then an ellipse, and when clicking on this field it will open a playback board. It also shows what they won or lost by, such as open 4, double 3, cross 3-4, etc. which is useful for analytics later and can be programmatically derived (not the responsible of the bot engine, but a separated service doing fuzzy logic, we can work on formal definitions later) by looking at the board state at move n minus 1. There is also a grid of rooms that have matches currently in progress, a button to create my own room, and a button to setup a bot tournament (detailed in Scenario 2). If it is my own room, I can wait for another user to join or have options to play with AI. Room settings include time control selection and best of mode selection.

The host takes red first match, both have to ready, red goes first, and waits for the opponent's turn. There should be a ghost stone and the game must disallow invalid moves, such as those violating the opening rule in the rulesets. The stone should be placed inside the cell, not on the cross section. The board and the clock should be maximally visible, and all font sizes should be large. Below the board is the move history section and a separate bot logs section that displays a line after each bot turn (detailed in Implication 1.5).
```

#### Implication 1.1
3 browser instances total: 2 logged in as different users with one creating the room and the other joining, both playing a bo3 of 1+0 time control; 1 acting as a guest that can see the room with the game in progress and enter to observe the match.

#### Implication 1.2
Only one browser instance acting as a guest, seeing and joining to observe a server-hosted room with ongoing bo3 bot matches using a 1+0 time control.

#### Implication 1.3
5 browser instances total: 2 pairs participating in 2 different rooms, and a guest checking to see if there are 2 rooms on display. The guest can join and depart each room at anytime.

#### Implication 1.4
For the user account that played 2 matches, they should be able to log in, go to match history, click on the move history field to open the playback board, and step through a full match.

#### Implication 1.5
An accurate realtime stats aggregation pipeline, possibly leveraging pub-sub. For the bot log line, I should be able to clearly see the following format metrics (on UI, ebf, hf, fh1 are hidden):

```text
Format: M, , , d=, n=, nps=, ebf=, tt=, hf=, fh1=, s=, thr=, t=, alloc=, [VCF|VCT|PONDER], pv= ...
    ebf: effective branching factor, e.g. 1.5 or 3.0
    tt: transposition table efficiency, percentage
    hf: hash full percentage
    fh1: first-move fail-high rate, e.g. 90% or 95%
    s: score, e.g. +12, M5 (mate-in-5)
    thr: number of search threads used
    alloc: time budget allocated by time manager vs t which is actual elapsed
    [VCT]: victory by continuous threes, a tag indicating the move was found by the dedicated VCT solver, which looks for forced wins using sequences of 4s and 3s, rather than the standard minimax search
    pv: principal variation

examples:

healthy: M24, Red, J9, d=14, n=6.25m, nps=2.5m, ebf=2.1, tt=38%, hf=45%, fh1=93%, s=+150, thr=4, t=2.50, alloc=2.50, pv=J9 K10 K9 L9 M8 L8

vct hit: M31, Blue, G7, d=9, n=45k, nps=1.2m, ebf=1.4, tt=18%, hf=60%, fh1=88%, s=M9, thr=4, t=0.03, alloc=3.00, [VCT], pv=G7 H7 G8 G6 G9 G10 F8 E9 I8

ponder hit: M12, Red, H10, d=16, n=18m, nps=2.8m, ebf=2.0, tt=44%, hf=78%, fh1=91%, s=-25, thr=4, t=0.01, alloc=4.00, [PONDER], pv=H10 I9 J8 K7 J10
```

### Scenario 2

```text
I want to conduct a bot vs bot tournament so I click on the bot tournament button to setup one. It should let me easily setup the bot participants, their difficulty levels, the starting rating for all participants, and the best of series constraint for each encounter. Provide an option to run 2 parallel matches at a time. There are 8x2 GB RAM and 8x2 cores available to allocate in theory, but it should be less especially for lower difficulty instances. It will use the exact same room backend surface to conduct each matchup. The default league conductor is a round robin format where each pair meets twice, once where A is red first and then where B is red first. The run logs should save to both the database for analysis and locally to the disc as txt files for insights into the lines. There's also a leaderboard.
```

#### Implication 2.1
Evaluation scores should start with 1.0 as the base unit (in practice and logs it'll be an integer 1000 milliunits, not floating point numbers), could be the value of a single unblocked tempo, with mate-in-N to prevent overrun. Bot difficulty tiers are mapped accordingly: easy is single core only, no VCF or VCT, and no TT. Medium is 2 cores only, has VCF, no VCT, and has TT. Hard is 4 cores, full VCF, full VCT, and full TT. Ponder will be implemented later after the core engine is finished, so it should be easily plugged in, meaning this needs to be taken into design decisions now. Full TT is 128MB, while medium has 32MB TT. `runtime.GOMAXPROCS(N); for i:=0;i<N;i++ {go func(){runtime.LockOSThread();for {doWork()}}}`. Each bot instance is ephemeral and everything should get nuked with cache clears after each match is finished, allowing the board state to cleanly reset. Be aware that because of the different nature between chess and this specific ruleset, certain move orderings or techniques in chess programming shouldn't be blindly applied because it could cause heavy regression in evaluation quality, with quiescence search being a primary example.

#### Implication 2.2
Successfully run a headless smoke series. This is not a full tournament but a curated run that uses the same stats pipeline to output data for analytics, allowing us to analyze the log outputs. The time control is 3+2, bo3, consisting of 6 matchups (12 matches): hard vs hard, hard vs medium, medium vs medium, medium vs easy, hard vs easy, and easy vs easy. The goal is to spot crashes, timeouts, illegal moves, wild bugs, state corruption, memory leaks, race conditions, flawed implementations, engine decision-making anomalies, search anomalies. If any appear, fix them and repeat the smoke series.

#### Implication 2.3
Successfully run another headless smoke series. Similar to above, this is a curated run using the stats pipeline for analytics, on a 1+0 time control, bo3. The 6 matchups (12 matches) are: hard vs hard, hard vs medium, medium vs medium, medium vs easy, hard vs easy, and easy vs easy. Again, spot crashes, timeouts, illegal moves, wild bugs, state corruption, memory leaks, race conditions, flawed implementations, engine decision-making and search anomalies. Fix any issues and repeat.

#### Implication 2.4
When all issues are completely weeded out, successfully run a full bot tournament via the UI. The starting rate is 1000 across the board, using a time control of 2+1, structured as a twice-pair round robin. Each matchup is a bo3 evaluating 6 bots in total, with 2 instances of each difficulty tier. To spot strength inversion issues.
