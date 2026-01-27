# Phase 2 Requirements Contract
## Prototype Polish + ELO + Bot Implementation

**Date:** 2025-12-28
**Status:** Planning Phase

---

## 1. Goal

Enhance the Caro game prototype with quality-of-life improvements and advanced features:

### A. Prototype Polish (Quality of Life)
- Undo move functionality
- Move history display
- Sound effects for stone placement
- Winning line animation

### B. ELO/Ranking System
- Track player ratings using the ELO algorithm
- Persistent ranking leaderboard
- Rating adjustments based on game outcomes

### C. Bot/AI Opponent
- Single-player mode against computer
- Multiple difficulty levels (Easy, Medium, Hard)
- Intelligent move selection using minimax algorithm

---

## 2. Acceptance Criteria

### A. Undo Move Functionality
- [ ] Players can undo their last move (button in UI)
- [ ] Undo works alternately (can't undo opponent's move)
- [ ] Game state correctly restored after undo (board, timer, turn)
- [ ] Undo is disabled after game over
- [ ] Can undo multiple moves (back to game start)
- [ ] Backend validates undo requests (prevent cheating)
- [ ] Unit tests for undo logic
- [ ] E2E test for undo flow

### B. Move History Display
- [ ] Visual list of all moves made (e.g., "Red: (7,7)", "Blue: (7,8)")
- [ ] Move numbers displayed (1, 2, 3, ...)
- [ ] Scrollable if history is long
- [ ] Current move highlighted
- [ ] Responsive on mobile
- [ ] Unit tests for history tracking
- [ ] Visual regression test for history display

### C. Sound Effects
- [ ] Sound plays when stone is placed
- [ ] Different sound for Red vs Blue
- [ ] Sound plays on win
- [ ] Volume control (mute button)
- [ ] Sounds are short (<0.5s) and non-intrusive
- [ ] No sound on first visit (auto-play policy)
- [ ] Works on mobile (iOS/Android audio constraints)
- [ ] Test for audio element creation

### D. Winning Line Animation
- [ ] Winning 5-in-a-row highlighted with animation
- [ ] Animation draws line from first to last stone
- [ ] Color-coded by winner (Red/Blue glow)
- [ ] Animation duration ~1s
- [ ] Only triggers on win (not draw/timeout)
- [ ] Unit test for win detection stores winning line
- [ ] E2E test verifies animation CSS class

### E. ELO/Ranking System
- [ ] ELO rating calculated using standard formula with difficulty multiplier:
  ```
  R_new = R_old + K * multiplier * (S - E)
  Where:
  - K = 32 (standard for games)
  - multiplier = 0.5 (Easy), 1.0 (Medium), 1.5 (Hard), 1.0 (PvP)
  - S = 1 (win), 0.5 (draw), 0 (loss)
  - E = 1 / (1 + 10^((R_opponent - R_self) / 400))
  - R_bot = 1200 (all bots have fixed rating)
  ```
- [ ] Players identified by unique ID (generated on first visit)
- [ ] Initial rating = 1200 (standard)
- [ ] Ratings stored in localStorage (prototype persistence)
- [ ] Leaderboard shows top 10 players
- [ ] Player can see their current rating
- [ ] Rating changes displayed after game (shows multiplier applied)
- [ ] ELO applies to both PvP and bot games
- [ ] Unit tests for ELO calculation (edge cases: equal ratings, large differences, multipliers)
- [ ] Integration test for rating persistence
- [ ] E2E test for complete game flow with rating update

### F. Bot/AI Opponent
- [ ] Single-player mode selection (vs Human / vs Bot)
- [ ] Five difficulty levels:
  - **Braindead**: Random valid moves, minimal thinking
  - **Easy**: Time-budgeted search (10% of time), depth 2-4
  - **Medium**: Time-budgeted search (30% of time), depth 3-5
  - **Hard**: Time-budgeted search (70% of time), depth 4-7
  - **Grandmaster**: Time-budgeted search (100% of time), depth 5-9+
- [ ] Bot respects Open Rule (enforced on move 3)
- [ ] No artificial delay (instant move for better UX)
- [ ] Bot plays as designated color (default: Blue)
- [ ] Evaluation function considers:
  - Consecutive stones (2, 3, 4 in a row)
  - Open vs closed ends
  - Center control (7,7 is valuable)
  - Blocking opponent's winning threats
- [ ] Depth scales automatically with machine performance (time-budget formula)
- [ ] Unit tests for minimax algorithm (known positions)
- [ ] Unit tests for evaluation function
- [ ] Integration test for bot API endpoint
- [ ] E2E test for complete game vs bot (each difficulty)
- [ ] Performance: Grandmaster responds within time control (7+5 format)

---

## 3. Definition of Done (Scope Boundaries)

### What We WILL Build
- Undo functionality (client + server validation)
- Move history UI component
- Client-side sound effects (using Web Audio API or HTML5 Audio)
- CSS-based winning line animation
- ELO calculation in backend + localStorage persistence with difficulty multipliers
- Four bot difficulties with minimax (Easy/Medium/Hard/Expert)
- Leaderboard display (top 10)
- ELO applies to both PvP and bot games

### What We WILL NOT Build (Out of Scope)
- ❌ Database persistence (MongoDB, PostgreSQL, etc.)
- ❌ User authentication/login
- ❌ Real-time multiplayer (SignalR)
- ❌ Online matchmaking
- ❌ Chat system
- ❌ Replay system (saved games)
- ❌ Custom board sizes (always 15x15)
- ❌ Custom time controls
- ❌ Tournament mode
- ❌ Mobile apps (React Native, etc.)
- ❌ Admin panel
- ❌ Anti-cheat beyond basic validation
- ❌ Payment systems
- ❌ Social features (friends, profiles, etc.)

### Files We WILL NOT Modify (Prevent Scope Creep)
- Core game logic (win detection, board rules) - already battle-tested
- Existing test infrastructure (unless adding new test utilities)
- Tailwind/PostCSS configuration (working fine)

---

## 4. Non-goals & Constraints

### Constraints
- **TDD is mandatory**: Every feature must have failing tests first
- **No external dependencies**: Use vanilla JS/TS, no AI libraries
- **Performance**: Bot must respond within 10s (Expert mode)
- **Mobile-friendly**: All features must work on touch devices
- **Browser compatibility**: Chrome, Firefox, Safari, Edge (last 2 versions)

### Non-goals (What We're Not Optimizing For)
- Perfect AI play (Hard mode should be challenging but not unbeatable)
- Enterprise-scale ELO (fine for hundreds of players, not millions)
- Sound design perfection (simple, pleasant sounds are sufficient)
- Fancy animations (keep them lightweight, CSS-only)

---

## 5. Verification Plan

### Unit Tests (Backend)
```bash
dotnet test backend/tests/Caro.Core.Tests
# Target: 50+ tests (undo logic, ELO calculation, bot algorithm)
```

### Unit Tests (Frontend)
```bash
npm run test
# Target: 15+ tests (sound effects, history component, ELO display)
```

### Integration Tests
```bash
node test-phase2.mjs
# Target: 20+ tests (undo flow, bot API, ELO updates)
```

### E2E Tests (Playwright)
```bash
npx playwright test
# Target: 25+ scenarios (complete games vs bot, undo, ELO updates)
```

### Manual Testing Checklist
- [ ] Undo works in single-player and two-player modes
- [ ] Sound plays on first move after user interaction
- [ ] Easy bot is beatable, Expert bot is very challenging
- [ ] ELO ratings update correctly with multipliers (Easy=0.5x, Medium=1x, Hard=1.5x, Expert=2x)
- [ ] Leaderboard displays in correct order
- [ ] All features work on mobile Chrome/Safari

### Performance Benchmarks
- Bot move time: <10s on 15x15 board (Expert mode)
- Page load: <2s on 3G
- Bundle size: <500KB (total JS)

---

## 6. Security Review

### Input Validation
- **Undo requests**: Validate game ID, player turn, move count
- **Bot difficulty**: Whitelist allowed values (easy, medium, hard)
- **ELO submissions**: Sanitize player names, validate rating ranges (0-3000)

### Data Storage
- **localStorage**: Mark as "prototype-only" (not secure, can be cleared)
- **Player IDs**: Use UUID v4, not sequential (prevent enumeration)
- **No PII**: Don't store emails, names, IPs

### Client-Side Security
- **ELO calculations**: Server-side only (trust no client values)
- **Undo logic**: Server validates state changes (prevent rollback exploits)
- **Bot moves**: Server-side only (prevent manipulation)

### Known Limitations (Acceptable for Prototype)
- localStorage can be edited by user (fine for prototype)
- No rate limiting on bot API (single-user prototype)
- No CSRF protection (no authentication anyway)

---

## 7. Implementation Order (TDD Approach)

### Sprint 1: Foundation (Day 1)
1. **Sound effects** - Independent feature, low risk
2. **Move history** - Backend tracking, frontend display
3. **Winning line animation** - Extends existing win detection

### Sprint 2: Game Logic (Day 2)
4. **Undo functionality** - Core logic changes, highest risk
5. **ELO system** - Backend calculation with difficulty multipliers

### Sprint 3: Bot Implementation (Day 3-4)
6. **Bot (Easy)** - Random moves, sets up API structure
7. **Bot (Medium)** - Minimax depth 3
8. **Bot (Hard)** - Minimax depth 5 + evaluation
9. **Bot (Expert)** - Minimax depth 7 + advanced evaluation

### Sprint 4: Polish & Testing (Day 5)
10. **Leaderboard** - Display ELO rankings
11. **Comprehensive testing** - All suites passing
12. **Documentation** - Update README, add screenshots

---

## 8. Success Metrics

- **Test coverage**: >80% for new code
- **All tests passing**: 100+ tests total
- **Manual verification**: All features work as expected
- **Code quality**: No linter warnings
- **Performance**: Bot responds in <5s
- **User experience**: No regressions in existing features

---

## 9. Open Questions (To Resolve Before Starting)

1. **Bot colors**: Should bot always play Blue, or let user choose?
   - **Decision**: Bot plays Blue by default (user can choose in UI)

2. **ELO vs bots**: Should playing vs bot affect ELO?
   - **Decision**: Yes, ELO applies to bot games with difficulty multipliers
   - **Braindead**: 0.1x multiplier (minimal risk/reward)
   - **Easy**: 0.5x multiplier (lower risk/reward)
   - **Medium**: 1.0x multiplier (standard)
   - **Hard**: 1.5x multiplier (higher risk/reward)
   - **Grandmaster**: 2.0x multiplier (maximum risk/reward)

3. **Undo limit**: Should we limit undo attempts (e.g., max 3 per game)?
   - **Decision**: No limit, can undo to start of game

4. **Sound on by default**: Should sounds be muted initially?
   - **Decision**: Muted by default, user opts-in (browser autoplay policy)

---

## 10. Sign-off

**Requirements Contract Approved**: [x] Yes

**Ready to Start Implementation**: [x] Yes

**First Feature to Implement**: Sound Effects (Sprint 1, Task 1)

---

*Last Updated: 2025-12-28*
