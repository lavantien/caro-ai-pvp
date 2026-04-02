# Checkpoint: v2.9.0

## Summary

Frontend audit and alignment with backend after major backend changes (16x16 board, API changes, tournament removal). Fixed critical board indexing bug, added missing grid lines, corrected rating logic, and polished UX.

## Changes

### Bug Fixes
- Board indexing: frontend used y-major (index = y * 16 + x), backend uses x-major (index = x * 16 + y)
- Grid lines: cells had no visible borders, added `border border-amber-300`
- Rating update: `previousPlayer` (captured pre-API) replaces `store.currentPlayer` (already switched)
- AI side labels: value/label mismatch for PvAI mode
- Timer sync: removed broken periodic server sync (backend returns hardcoded 0s)
- Open Rule description: "center 3x3 zone" -> "at least 3 intersections away"
- app.html/app.pcss: restored from clobbered-empty state

### UX Additions
- Last-move highlighting (colored ring on most recent stone)
- "New Game" button after game over
- Inline error banner replacing alert() dialogs
- "Start Playing" call-to-action on landing page

### Technical
- moveInProgress guard prevents double-click race conditions
- ExecuteUnderLock renamed to MutateUnderLock
- E2E tests updated for 16x16 board

## Verification

| Check | Result |
|-------|--------|
| Frontend unit tests | Pass |
| E2E tests (17 tests) | Pass |
| svelte-check | Pass |
| Visual board rendering | Verified |

## Version

- Target: v2.9.0
- Previous: v2.8.1
