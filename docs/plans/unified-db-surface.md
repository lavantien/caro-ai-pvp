# unified db surface

Owner rulings 2026-10-24, fixed before any implementation: every engine activity and stat line is saved to the db, joining a room backfills the series from its start out of the db before live SSE continues, the schemas collapse into one unified db surface, a bot identity is only a name capturing a set of engine configs with instances strictly ephemeral per game, and series and tournament orchestration reference bot names never instances.

## persistence inventory

One sqlite db per deployment, opened through server.Open (cmd/caro/serve.go:60,110, cmd/caro/tourney.go:78,115,159) with WAL, synchronous normal, busy timeout, foreign keys, and immediate transactions (store.go:120-128). Schema V12 (config.go:349) holds 13 tables in two parallel families:

Casual family: users (bot accounts seeded with empty salt and hash, schema.go:163-172), sessions, series (tc, bo, red and blue user, state, winner, variant since V12), games (outcome, moves blob, full turns, won by, bot name since V5), game_stats (game id, move no, raw M-line text, schema.go:154-161), rating_events.

Tournament family: tournament_runs (label, drive pid and heartbeat lease, variant), tournament_participants (slot, name, tier), tournament_series (pairing slots, wins, winner), tournament_games (slots, outcome, moves blob), tournament_game_stats (17 structured columns per line, schema.go:188-209), tournament_standings.

Write paths today. Casual completions land in one transaction per game at the end: game row, stat rows from the in-memory r.mlines slice, rating pair unless a seat is a bot account, series finish, then the txt side writes post commit on a detached context (store.go:417-451, match.go:159-178). Tournament games append through the drive-lease-guarded transaction at game end with structured stat columns (tourney/store.go:174-297), while the conductor writes each M-line event to logs/tourny series txt as it happens plus a game summary (tourney/runner.go:410-449). Casual txt lives at logs/casual as per-series append-only files (casuallog.go:39-75, config.go:137-141).

Live delivery is SSE only. The hub keeps no replay buffer (hub.go:21-78), the room detail snapshot carries moves, turn, clocks, and playable cells with no mlines field (httpapi.go:125-132,392-418), and room.js accumulates bot log lines purely from live events. Finished games carry full lines in game_stats but the playback page renders moves only (ui_room.go:65-127). Readers outside the server: evalfit and logmine open the db directly, logstats and crossarm parse the txt trees.

## what the redesign closes

The M-line db write happens only at completion, so a process death mid game loses every line of that game held in r.mlines, a spectator joining mid series gets nothing before their subscription, and playback shows stones with no engine lines. Two stat schemas exist for the same content, raw text for casual and 17 structured columns for tournaments, forcing every miner to carry two parsers. Bot-vs-bot rooms persist never: series id 0, synthetic user ids -2 and -3 (room.go:33-36,112-147), pinned by test (botvsbot_test.go:165-174), so a restart or retire erases them while the lobby advertises the mode. The casual txt side carries the recorded defects: a recreated db resets series ids and appends a new series 1 into the old same-named file, and the per-process mutex cannot guard trimTornTail's read-write truncate across processes.

## target schema

Schema V13 collapses the two families into one surface. Existing users, sessions, rating_events, tournament_runs, tournament_participants, and tournament_standings carry over unchanged except the tournament prefix dropping from names kept as they are to avoid churn, decided below in migration.

```
series        gains run_id INTEGER NULL REFERENCES runs(id)
              and pairing_slot INTEGER NULL, plus red_name and blue_name
              TEXT NULL as display names for bot seats
games         gains red_slot and blue_slot INTEGER NULL with a check that
              exactly one of the user pair or the slot pair is set
move_stats    (game_id, move_no, line TEXT) PRIMARY KEY (game_id, move_no)
              replaces both game_stats and tournament_game_stats
outcome       on games becomes nullable until completion, checked against the
              existing tags when not null
```

Tournament series become series rows with run_id set and user seats null, referencing participants by slot. Casual series keep user seats with run_id null. move_stats stores the raw frozen-format M-line as the canonical atom for both families: the format is the parse contract logstats already owns, the structured tournament columns are derivable from it, and one parser serves db mining, backfill, and playback. Games rows are created at game start with outcome null and finalized at completion, so move_stats always has a real game id to attach to.

## per-move persistence and the join backfill

runBotTurn already renders the line string at emission (match.go:705-708). The same beat routes one insert into move_stats through the WriteQueue, which is the existing single-writer worker (writeq.go:23-48), so per-move writes serialize with completions and never contend. Human moves have no engine activity and stay in the moves blob alone. Cost under WAL with synchronous normal is one small insert per engine move, bounded by move cadence at every time control, negligible next to the search that produced the line.

Completion stays the repair pass: ApplyCompletion finalizes outcome and moves and re-inserts any move_stats rows the per-move path failed to write, idempotent under the primary key, so a transient insert failure logs loudly and costs nothing at rest.

Join backfill contract, in order: subscribe to the room stream first, then read the room detail snapshot, which gains an mlines field queried from move_stats for the live game ordered by move_no plus the count persisted at snapshot time. The client renders the backfill once, then applies live events while skipping lines whose move_no is at or below the snapshot watermark. This makes the boundary deterministic without a hub replay buffer: the subscription precedes the snapshot read, so no event between the two reads can be lost, and duplicates are dropped by watermark. The playback page reads the same query and renders lines beside the existing move list through the existing line format.

## bot model

Lifecycle audit verdict: engines are already ephemeral per game. startGameLocked closes any live engines before constructing fresh ones per seat (match.go:52-80), completeGameLocked chains into startGameLocked for the next game of a series (match.go:154), botLoop's deferred closeEnginesAtExit covers worker exit on quit and retire (match.go:612-634), and closeEnginesLocked stops both ponder lanes, closes each searcher, and releases the TT holds (match.go:567-587). Nothing engine-side survives a game, TT warmth included, by construction. The one leak found: parkLocked after an admission refusal marks the room over but never calls finishLifecycle, so the manager map entry survives until some later call pokes maybeFinish (room.go:459-466). Fixed outside this redesign by the 2026-10-24 admission hotfix, which retires the room on refusal and gates create-room on available physical memory, so the db wave carries no leak item.

Identity: a seat captures a tier plus an optional display name at room creation, the display name shown is tier plus room id for casual vs-bot seats (match.go:179-190), and no instance state outlives the game. Persistence moves to the ruling's model: CreateBotVsBot drops the synthetic -2 and -3 ids, creates a real series row whose red_user and blue_user are the seeded tier accounts, and records the display names in the new series name columns. Orchestration keeps addressing bots by name. Ratings stay excluded automatically because anyBotSeat skips rating pairs whenever a seat is a bot account (store.go:431-439,547-561). The persistence pin in botvsbot_test.go flips to assert the rows exist.

## text log fate

Casual txt retires with the writer and the format constant: the db now holds every line with strictly better durability, the per-move path removed the completion-window loss the txt side once hedged, and both recorded defects lived only in that writer. Tournament txt stays as the run-local operator evidence trail under the standing archive law, unchanged, with logstats gaining the db as an equal input beside it.

## migration from v12

V13 is a rebuild migration inside one transaction: create the new series, games, and move_stats shapes, copy casual rows as they are, fold tournament_series into series and tournament_games into games with slot seats and run references, re-render every tournament_game_stats row into the frozen line format through a one-time renderer that consumes the stored structured columns plus the run's variant geometry for cell names, drop the four superseded tables, and rename. A pre-migration file copy of the db is taken before the transaction and left beside the db, since sqlite cannot down-migrate and rollback means restoring that copy. Upgrade tests run the migration on a committed V12 fixture db and assert row counts per table plus a sampled line round trip through the logstats parser for both families. Every consumer of the old table names updates in the same wave: UserStats, MatchHistory, playback, history JSON, the migrate ceiling test, and the evalfit and logmine db readers.

## risks

The rebuild touches every reader of games and game_stats, so it lands as its own wave after the v0.29 tag and the queued engine batches, never mixed with engine work. Nullable outcomes change every query that assumed completeness, each needs an explicit is-not-null or state join. The one-time structured-to-line renderer must reproduce the frozen format exactly or historical tournament lines stop parsing, which the round trip test pins. Pre-created game rows mean abandoned or crashed games can linger with null outcome, needing a sweep or an explicit abandoned state at retire. Two server processes against one db remain unsupported, stated as a startup assertion on the db file, which is also what dissolves the cross-process txt concern for good.

## open owner decisions

Whether tier profile pages include bot-vs-bot games under the tier accounts or filter them to a bot-only view. Whether move_stats also carries the structured columns beside the raw line for cheap queries, at double write cost, or stays line-only with parsing on read. Whether casual txt keeps a derived export command for corpus handoffs or disappears entirely.
