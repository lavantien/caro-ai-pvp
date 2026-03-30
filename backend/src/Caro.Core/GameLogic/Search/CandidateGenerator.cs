using Caro.Core.Domain.Configuration;
using Caro.Core.Domain.Entities;

namespace Caro.Core.GameLogic.Search;

/// <summary>
/// Generates candidate moves (empty cells near existing stones).
/// Zero-allocation implementation using stackalloc for tracking.
/// Prioritizes moves near center of mass and board center to avoid distraction from isolated stones.
/// </summary>
public static class CandidateGenerator
{
    private const int BoardSize = GameConstants.BoardSize;

    /// <summary>
    /// Get candidate moves from an immutable Board.
    /// Returns empty cells within searchRadius of any existing stone.
    /// CRITICAL: Prioritizes moves near center of mass to avoid distraction from isolated stones.
    /// </summary>
    public static List<(int x, int y)> GetCandidateMoves(Board board, int searchRadius = SearchConstants.MaxSearchRadius)
    {
        const int boardSize = BoardSize;
        const int cellCount = boardSize * boardSize;

        // Use stackalloc for considered tracking (zero allocation)
        Span<bool> considered = stackalloc bool[cellCount];

        // Pre-allocate with reasonable capacity to avoid resizing
        var candidates = new List<(int x, int y)>(64);

        // Count stones to determine game phase
        int stoneCount = 0;
        int sumX = 0, sumY = 0;
        for (int x = 0; x < boardSize; x++)
        {
            for (int y = 0; y < boardSize; y++)
            {
                if (board.GetCell(x, y).Player != Player.None)
                {
                    stoneCount++;
                    sumX += x;
                    sumY += y;
                }
            }
        }

        // Empty board - return center-area moves for opening
        if (stoneCount == 0)
        {
            int center = boardSize / 2;
            for (int x = center - 1; x <= center + 1; x++)
            {
                for (int y = center - 1; y <= center + 1; y++)
                {
                    candidates.Add((x, y));
                }
            }
            return candidates;
        }

        // Calculate center of mass of all stones
        // This prevents being distracted by isolated opponent stones in corners
        int centerX = sumX / stoneCount;
        int centerY = sumY / stoneCount;
        int centerPos = boardSize / 2;

        // CRITICAL: Always add moves near center of mass FIRST
        // This ensures the main area of play gets priority
        const int CenterRadius = 3;
        for (int dx = -CenterRadius; dx <= CenterRadius; dx++)
        {
            for (int dy = -CenterRadius; dy <= CenterRadius; dy++)
            {
                int x = centerX + dx;
                int y = centerY + dy;
                if (x >= 0 && x < boardSize && y >= 0 && y < boardSize)
                {
                    int idx = x * boardSize + y;
                    if (!considered[idx] && board.GetCell(x, y).Player == Player.None)
                    {
                        candidates.Add((x, y));
                        considered[idx] = true;
                    }
                }
            }
        }

        // Add moves near center of board if not already included
        for (int dx = -2; dx <= 2; dx++)
        {
            for (int dy = -2; dy <= 2; dy++)
            {
                int x = centerPos + dx;
                int y = centerPos + dy;
                if (x >= 0 && x < boardSize && y >= 0 && y < boardSize)
                {
                    int idx = x * boardSize + y;
                    if (!considered[idx] && board.GetCell(x, y).Player == Player.None)
                    {
                        candidates.Add((x, y));
                        considered[idx] = true;
                    }
                }
            }
        }

        // Add moves near existing stones (lower priority)
        for (int x = 0; x < boardSize; x++)
        {
            for (int y = 0; y < boardSize; y++)
            {
                var cell = board.GetCell(x, y);
                if (cell.Player != Player.None)
                {
                    for (int dx = -searchRadius; dx <= searchRadius; dx++)
                    {
                        for (int dy = -searchRadius; dy <= searchRadius; dy++)
                        {
                            var nx = x + dx;
                            var ny = y + dy;

                            if (nx >= 0 && nx < boardSize && ny >= 0 && ny < boardSize)
                            {
                                int idx = nx * boardSize + ny;
                                if (!considered[idx])
                                {
                                    considered[idx] = true;
                                    if (board.GetCell(nx, ny).Player == Player.None)
                                    {
                                        candidates.Add((nx, ny));
                                    }
                                }
                            }
                        }
                    }
                }
            }
        }

        return candidates;
    }

    /// <summary>
    /// Get candidate moves from a SearchBoard (high-performance path).
    /// Returns empty cells within searchRadius of any existing stone.
    /// CRITICAL: Prioritizes moves near center of mass to avoid distraction from isolated stones.
    /// </summary>
    public static List<(int x, int y)> GetCandidateMoves(SearchBoard board, int searchRadius = SearchConstants.MaxSearchRadius)
    {
        const int boardSize = BoardSize;
        const int cellCount = boardSize * boardSize;

        // Use stackalloc for considered tracking (zero allocation)
        Span<bool> considered = stackalloc bool[cellCount];

        // Pre-allocate with reasonable capacity to avoid resizing
        var candidates = new List<(int x, int y)>(64);

        // Count stones to determine game phase
        int stoneCount = 0;
        for (int x = 0; x < boardSize; x++)
        {
            for (int y = 0; y < boardSize; y++)
            {
                if (!board.IsEmpty(x, y))
                {
                    stoneCount++;
                }
            }
        }

        // Empty board - return center-area moves for opening
        if (stoneCount == 0)
        {
            int center = boardSize / 2;
            for (int x = center - 1; x <= center + 1; x++)
            {
                for (int y = center - 1; y <= center + 1; y++)
                {
                    candidates.Add((x, y));
                }
            }
            return candidates;
        }

        int centerPos = boardSize / 2;

        // PRIORITY 1: Add moves near center of board FIRST
        // This ensures we control the center regardless of opponent's random moves
        const int CenterRadius = 4;
        for (int dx = -CenterRadius; dx <= CenterRadius; dx++)
        {
            for (int dy = -CenterRadius; dy <= CenterRadius; dy++)
            {
                int x = centerPos + dx;
                int y = centerPos + dy;
                if (x >= 0 && x < boardSize && y >= 0 && y < boardSize)
                {
                    int idx = x * boardSize + y;
                    if (!considered[idx] && board.IsEmpty(x, y))
                    {
                        candidates.Add((x, y));
                        considered[idx] = true;
                    }
                }
            }
        }

        // PRIORITY 2: Add moves near existing stones
        for (int x = 0; x < boardSize; x++)
        {
            for (int y = 0; y < boardSize; y++)
            {
                if (!board.IsEmpty(x, y))
                {
                    for (int dx = -searchRadius; dx <= searchRadius; dx++)
                    {
                        for (int dy = -searchRadius; dy <= searchRadius; dy++)
                        {
                            var nx = x + dx;
                            var ny = y + dy;

                            if (nx >= 0 && nx < boardSize && ny >= 0 && ny < boardSize)
                            {
                                int idx = nx * boardSize + ny;
                                if (!considered[idx])
                                {
                                    considered[idx] = true;
                                    if (board.IsEmpty(nx, ny))
                                    {
                                        candidates.Add((nx, ny));
                                    }
                                }
                            }
                        }
                    }
                }
            }
        }

        return candidates;
    }
}
