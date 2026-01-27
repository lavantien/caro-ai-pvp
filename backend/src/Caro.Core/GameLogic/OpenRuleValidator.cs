namespace Caro.Core.GameLogic;

using Caro.Core.Entities;

public class OpenRuleValidator
{
    public bool IsValidSecondMove(Board board, int x, int y)
    {
        // Count total stones on board
        var stoneCount = board.Cells.Count(c => !c.IsEmpty);

        // Open Rule only applies to move #3 (Red's second move)
        if (stoneCount != 2)
            return true;

        // Check if position is in 5x5 zone around center (7,7)
        bool isInRestrictedZone = x >= 5 && x <= 9 && y >= 5 && y <= 9;

        return !isInRestrictedZone;
    }
}
