using Caro.Core.Domain.Entities;

namespace Caro.Core.GameLogic;

/// <summary>
/// ThreatDetector partial class - Threat creation factory methods.
/// Creates threat objects from detected patterns.
/// </summary>
public partial class ThreatDetector
{
    private Threat CreateFiveThreat(int x, int y, Player player)
    {
        return new Threat
        {
            Type = ThreatType.StraightFour,  // Five is the ultimate threat
            Owner = player,
            GainSquares = new List<(int x, int y)> { (x, y) },
            StonePositions = new List<(int x, int y)> { (x, y) },
            Direction = (1, 0)
        };
    }

    private Threat CreateOpenFourThreat(BitKeyBoard board, int x, int y, Player player)
    {
        var gainSquares = FindOpenFourGains(board, x, y, player);
        return new Threat
        {
            Type = ThreatType.StraightFour,
            Owner = player,
            GainSquares = gainSquares,
            StonePositions = new List<(int x, int y)> { (x, y) },
            Direction = (1, 0)  // Direction determined by pattern
        };
    }

    private Threat CreateClosedFourThreat(BitKeyBoard board, int x, int y, Player player)
    {
        var gainSquares = FindClosedFourGains(board, x, y, player);
        return new Threat
        {
            Type = ThreatType.BrokenFour,
            Owner = player,
            GainSquares = gainSquares,
            StonePositions = new List<(int x, int y)> { (x, y) },
            Direction = (1, 0)
        };
    }

    private Threat CreateOpenThreeThreat(BitKeyBoard board, int x, int y, Player player)
    {
        var gainSquares = FindOpenThreeGains(board, x, y, player);
        return new Threat
        {
            Type = ThreatType.StraightThree,
            Owner = player,
            GainSquares = gainSquares,
            StonePositions = new List<(int x, int y)> { (x, y) },
            Direction = (1, 0)
        };
    }

    private Threat CreateDoubleThreeThreat(BitKeyBoard board, int x, int y, Player player)
    {
        return new Threat
        {
            Type = ThreatType.StraightThree,  // Double three is very strong
            Owner = player,
            GainSquares = new List<(int x, int y)> { (x, y) },
            StonePositions = new List<(int x, int y)> { (x, y) },
            Direction = (1, 0)
        };
    }

    private Threat CreateDoubleThreat(BitKeyBoard board, int x, int y, Player player)
    {
        return new Threat
        {
            Type = ThreatType.StraightFour,  // Four + Three is winning
            Owner = player,
            GainSquares = new List<(int x, int y)> { (x, y) },
            StonePositions = new List<(int x, int y)> { (x, y) },
            Direction = (1, 0)
        };
    }

    private List<(int x, int y)> FindOpenFourGains(BitKeyBoard board, int x, int y, Player player)
    {
        var gains = new List<(int x, int y)>();
        // Find the two ends of the open four
        for (int dx = -1; dx <= 1; dx++)
        {
            for (int dy = -1; dy <= 1; dy++)
            {
                if (dx == 0 && dy == 0) continue;
                if (dx != 0 && dy != 0 && dx != dy && dx != -dy) continue;

                // Check both directions
                for (int dir = -1; dir <= 1; dir += 2)
                {
                    int nx = x + dx * dir, ny = y + dy * dir;
                    if (nx >= 0 && nx < 32 && ny >= 0 && ny < 32)
                    {
                        var cellPlayer = board.GetPlayerAt(nx, ny);
                        if (cellPlayer == Player.None)
                            gains.Add((nx, ny));
                    }
                }
            }
        }
        return gains.Distinct().ToList();
    }

    private List<(int x, int y)> FindClosedFourGains(BitKeyBoard board, int x, int y, Player player)
    {
        return FindOpenFourGains(board, x, y, player);
    }

    private List<(int x, int y)> FindOpenThreeGains(BitKeyBoard board, int x, int y, Player player)
    {
        return FindOpenFourGains(board, x, y, player);
    }
}
