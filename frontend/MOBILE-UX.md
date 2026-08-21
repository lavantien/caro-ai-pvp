# Mobile UX Features - Test Checklist

## Implemented Features:

### 1. Ghost Stone Offset (lifted above the hovered cell)

- **Location**: `src/lib/components/Board.svelte` (touch handlers on the board wrapper)
- **Implementation**:
  - `ontouchmove` resolves the cell under the finger via `elementFromPoint`
  - Ghost stone appears at that cell's center, shifted up by `0.78 * cellSize` (`calculateGhostStonePosition`)
  - Ghost stone is a dashed border circle with "?" symbol
  - Removed on `ontouchend` and `ontouchcancel`

**Manual Test**:

1. Open game on mobile device or browser dev tools (device mode)
2. Touch and drag on the board
3. Observe: Ghost stone snaps to the cell under your finger, lifted above it
4. Expected: You can see where you're placing the stone

### 2. Haptic Feedback

- **Location**: `src/lib/utils/haptics.ts`
- **Implementation**:
  - Valid move: 10ms short pulse
  - Invalid move: 30-50-30ms error pattern
  - Safely checks `navigator.vibrate` availability

**Manual Test**:

1. Open game on mobile device with vibration support
2. Tap an empty cell → Should feel short vibration (valid move)
3. Tap an occupied cell → Should feel triple vibration pattern (invalid move)
4. Expected: Different feedback for valid vs invalid moves

### 3. Touch Gesture Isolation on the Board

- **Location**: `src/lib/components/Board.svelte` (grid container)
- **Implementation**:
  - `touch-none` class on the grid (touch-action: none) blocks scroll and pinch gestures over the board so placement taps stay accurate
  - Page pinch-to-zoom still works outside the board area
  - Cell size is responsive, not fixed: `computeCellSize` targets 95% of the container width clamped to 18-64px, updated by a ResizeObserver

**Manual Test**:

1. Open game on mobile device
2. Use two fingers to pinch over the board
3. Expected: The board does not zoom or scroll (by design); zooming outside the board area works

## Verification Status:

| Feature            | Status         | Notes                                      |
| ------------------ | -------------- | ------------------------------------------ |
| Ghost stone offset | ✅ Code review | Cell-center snap, lifted 0.78 x cellSize   |
| Haptic feedback    | ✅ Code review | Valid: 10ms, Invalid: 30-50-30ms           |
| Touch isolation    | ✅ Code review | touch-none on the grid, zoom outside works |

## Automated Testing Limitations:

Playwright cannot test:

- Touch gestures (ontouchmove) accurately in headless mode
- Haptic feedback (navigator.vibrate not available)
- Actual pinch-to-zoom gestures

**Recommendation**: Manual testing on actual mobile device required for full verification.

## Browser Compatibility:

- **Ghost stone**: Works on all touch-enabled browsers
- **Haptics**: Chrome Android, Edge Android, Firefox Android (partial)
- **Touch isolation**: touch-action support in all modern browsers

## Code Quality:

- ✅ All features follow mobile UX best practices
- ✅ Graceful degradation (vibrate checks availability)
- ✅ Board grid opts out of touch gestures (touch-none) for accurate placement; page zoom works outside the board
- ✅ Proper cleanup (ghost stone removed on touch end and cancel)
