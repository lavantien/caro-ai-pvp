using Caro.Core.Domain.Entities;

namespace Caro.Core.GameLogic;

/// <summary>
/// ThreatDetector partial class - Fast BitKey-based detection methods.
/// O(1) pattern lookup for threat detection.
/// </summary>
public partial class ThreatDetector
{
    /// <summary>
    /// Fast threat detection using BitKey pattern system.
    /// Returns threats detected at a specific position using O(1) pattern lookup.
    /// </summary>
    public List<Threat> DetectThreatsAt(BitKeyBoard bitKeyBoard, int x, int y, Player player)
    {
        var threats = new List<Threat>();
        var (combined, threatCount) = BitKeyPatternTable.GetCombinedPattern(bitKeyBoard, x, y);

        if (combined == Pattern4Evaluator.CaroPattern4.None)
            return threats;

        // Create threat based on pattern type
        var threat = combined switch
        {
            Pattern4Evaluator.CaroPattern4.Exactly5 => CreateFiveThreat(x, y, player),
            Pattern4Evaluator.CaroPattern4.Flex4 => CreateOpenFourThreat(bitKeyBoard, x, y, player),
            Pattern4Evaluator.CaroPattern4.Block4 => CreateClosedFourThreat(bitKeyBoard, x, y, player),
            Pattern4Evaluator.CaroPattern4.Flex3 => CreateOpenThreeThreat(bitKeyBoard, x, y, player),
            Pattern4Evaluator.CaroPattern4.DoubleFlex3 => CreateDoubleThreeThreat(bitKeyBoard, x, y, player),
            Pattern4Evaluator.CaroPattern4.Flex4Flex3 => CreateDoubleThreat(bitKeyBoard, x, y, player),
            _ => null
        };

        if (threat != null)
            threats.Add(threat);

        return threats;
    }

    /// <summary>
    /// Check if a position is a winning move using BitKey O(1) lookup.
    /// </summary>
    public bool IsWinningMoveFast(BitKeyBoard bitKeyBoard, int x, int y, Player player)
    {
        return BitKeyPatternTable.IsWinningMove(bitKeyBoard, x, y, player);
    }

    /// <summary>
    /// Check if a position creates a double threat using BitKey O(1) lookup.
    /// </summary>
    public bool IsDoubleThreatFast(BitKeyBoard bitKeyBoard, int x, int y, Player player)
    {
        return BitKeyPatternTable.IsDoubleThreatMove(bitKeyBoard, x, y, player);
    }

    /// <summary>
    /// Get the pattern classification at a position using BitKey O(1) lookup.
    /// </summary>
    public (Pattern4Evaluator.CaroPattern4 Pattern, int ThreatCount) GetPatternAt(BitKeyBoard bitKeyBoard, int x, int y)
    {
        return BitKeyPatternTable.GetCombinedPattern(bitKeyBoard, x, y);
    }

    /// <summary>
    /// Evaluate a position using BitKey O(1) pattern scoring.
    /// </summary>
    public int EvaluatePosition(BitKeyBoard bitKeyBoard, int x, int y)
    {
        return BitKeyPatternTable.EvaluatePosition(bitKeyBoard, x, y);
    }

    /// <summary>
    /// Find all threat moves using BitKey fast pattern detection.
    /// </summary>
    public List<(int x, int y, int Score)> FindThreatMovesFast(Board board, Player player)
    {
        var threatMoves = new List<(int x, int y, int Score)>();
        var bitKeyBoard = new BitKeyBoard(board);

        for (int x = 0; x < BitBoard.Size; x++)
        {
            for (int y = 0; y < BitBoard.Size; y++)
            {
                if (!board.GetCell(x, y).IsEmpty)
                    continue;

                // Create test board with the move
                var testBoard = bitKeyBoard.Clone();
                testBoard.SetBit(x, y, player);

                var (pattern, threatCount) = BitKeyPatternTable.GetCombinedPattern(testBoard, x, y);

                if (pattern >= Pattern4Evaluator.CaroPattern4.Flex3)
                {
                    int score = BitKeyPatternTable.EvaluatePosition(testBoard, x, y);
                    threatMoves.Add((x, y, score));
                }
            }
        }

        // Sort by score descending
        threatMoves.Sort((a, b) => b.Score.CompareTo(a.Score));
        return threatMoves;
    }
}
