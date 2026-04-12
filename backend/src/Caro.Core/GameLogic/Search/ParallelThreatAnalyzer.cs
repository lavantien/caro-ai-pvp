using Caro.Core.Domain.Entities;

namespace Caro.Core.GameLogic.Search;

/// <summary>
/// Stateless threat analysis for the parallel search path.
/// Provides opponent threat detection, critical threat identification,
/// open three blocking, and Open Rule validation.
/// </summary>
public static class ParallelThreatAnalyzer
{
    /// <summary>
    /// Check if a move is valid per the Open Rule for Red's second move (move #3).
    /// The Open Rule requires Red's second move to be at least 3 intersections away
    /// from the first red stone (Chebyshev distance >= 3).
    /// </summary>
    public static bool IsValidPerOpenRule(Board board, int x, int y)
    {
        int stoneCount = 0;
        (int firstX, int firstY) firstRed = (-1, -1);

        for (int bx = 0; bx < board.BoardSize; bx++)
        {
            for (int by = 0; by < board.BoardSize; by++)
            {
                var cell = board.GetCell(bx, by);
                if (cell.Player != Player.None)
                {
                    stoneCount++;
                    if (cell.Player == Player.Red && firstRed.firstX < 0)
                    {
                        firstRed = (bx, by);
                    }
                }
            }
        }

        if (stoneCount != 2)
            return true;

        if (firstRed.firstX < 0)
            return true;

        int dx = Math.Abs(x - firstRed.firstX);
        int dy = Math.Abs(y - firstRed.firstY);
        return Math.Max(dx, dy) >= 3;
    }

    /// <summary>
    /// Get opponent's threat moves (squares we should consider blocking).
    /// Priority order:
    /// 1. Five in row (immediate win)
    /// 2. Semi-open four (StraightFour)
    /// 3. Open four (XXXX - both ends open)
    /// 4. Broken four (XXX_X - can create double threat)
    /// </summary>
    public static List<(int x, int y)> GetOpponentThreatMoves(Board board, Player opponent, WinDetector winDetector)
    {
        var threats = new List<(int x, int y)>();

        for (int x = 0; x < BitBoard.Size; x++)
        {
            for (int y = 0; y < BitBoard.Size; y++)
            {
                if (!board.GetCell(x, y).IsEmpty)
                    continue;

                var testBoard = board.PlaceStone(x, y, opponent);
                bool isWinningMove = winDetector.CheckWin(testBoard).HasWinner;

                if (isWinningMove)
                {
                    threats.Add((x, y));
                }
            }
        }

        if (threats.Count > 0)
            return threats;

        var detector = new ThreatDetector();
        var opponentThreats = detector.DetectThreats(board, opponent);

        foreach (var threat in opponentThreats)
        {
            if (threat.Type == ThreatType.StraightFour)
            {
                foreach (var gainSquare in threat.GainSquares)
                {
                    if (board.GetCell(gainSquare.x, gainSquare.y).IsEmpty && !threats.Contains(gainSquare))
                    {
                        threats.Add(gainSquare);
                    }
                }

                if (threat.GainSquares.Count == 1 && threats.Count > 0)
                {
                    return threats;
                }
            }
        }

        foreach (var threat in opponentThreats)
        {
            if (threat.Type == ThreatType.BrokenFour)
            {
                foreach (var gainSquare in threat.GainSquares)
                {
                    if (board.GetCell(gainSquare.x, gainSquare.y).IsEmpty && !threats.Contains(gainSquare))
                    {
                        threats.Add(gainSquare);
                    }
                }
            }
        }

        return threats;
    }

    /// <summary>
    /// Find opponent's CRITICAL threat moves that MUST be blocked immediately.
    /// Only returns threats where blocking is mandatory - does NOT include BrokenFours.
    /// Priority order:
    /// 1. Five in row (immediate win)
    /// 2. Straight four (open four or semi-open four)
    /// </summary>
    public static List<(int x, int y)> GetCriticalThreatMoves(Board board, Player opponent, WinDetector winDetector)
    {
        var threats = new List<(int x, int y)>();

        for (int x = 0; x < BitBoard.Size; x++)
        {
            for (int y = 0; y < BitBoard.Size; y++)
            {
                if (!board.GetCell(x, y).IsEmpty)
                    continue;

                var testBoard = board.PlaceStone(x, y, opponent);
                bool isWinningMove = winDetector.CheckWin(testBoard).HasWinner;

                if (isWinningMove)
                {
                    threats.Add((x, y));
                }
            }
        }

        if (threats.Count > 0)
            return threats;

        var detector = new ThreatDetector();
        var opponentThreats = detector.DetectThreats(board, opponent);

        foreach (var threat in opponentThreats)
        {
            if (threat.Type == ThreatType.StraightFour || threat.Type == ThreatType.BrokenFour)
            {
                foreach (var gainSquare in threat.GainSquares)
                {
                    if (board.GetCell(gainSquare.x, gainSquare.y).IsEmpty && !threats.Contains(gainSquare))
                    {
                        threats.Add(gainSquare);
                    }
                }
            }
        }

        return threats;
    }

    /// <summary>
    /// Get open three blocking squares (StraightThree threats).
    /// These are developing threats that should be prioritized but don't require
    /// mandatory blocking like StraightFour.
    /// </summary>
    public static List<(int x, int y)> GetOpenThreeBlocks(Board board, Player opponent)
    {
        var blocks = new List<(int x, int y)>();
        var detector = new ThreatDetector();
        var opponentThreats = detector.DetectThreats(board, opponent);

        foreach (var threat in opponentThreats)
        {
            if (threat.Type == ThreatType.StraightThree)
            {
                foreach (var gainSquare in threat.GainSquares)
                {
                    if (board.GetCell(gainSquare.x, gainSquare.y).IsEmpty && !blocks.Contains(gainSquare))
                    {
                        blocks.Add(gainSquare);
                    }
                }
            }
        }

        return blocks;
    }
}
