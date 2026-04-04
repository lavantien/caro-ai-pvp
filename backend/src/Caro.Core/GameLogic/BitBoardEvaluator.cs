using System.Runtime.CompilerServices;
using Caro.Core.Domain.Configuration;
using Caro.Core.Domain.Entities;

namespace Caro.Core.GameLogic;

/// <summary>
/// High-performance board evaluator using BitBoard operations
/// Leverages bitwise operations and hardware POPCNT for fast pattern detection
/// </summary>
public static partial class BitBoardEvaluator
{
    // Import scoring weights from centralized constants
    // Local aliases for readability within this file (used by default overloads)
    private const int FiveInRowScore = EvaluationConstants.FiveInRowScore;
    private const int OpenFourScore = EvaluationConstants.OpenFourScore;
    private const int ClosedFourScore = EvaluationConstants.ClosedFourScore;
    private const int OpenThreeScore = EvaluationConstants.OpenThreeScore;
    private const int ClosedThreeScore = EvaluationConstants.ClosedThreeScore;
    private const int OpenTwoScore = EvaluationConstants.OpenTwoScore;
    private const int CenterBonus = EvaluationConstants.CenterBonus;

    /// <summary>
    /// Defense multiplier for asymmetric scoring.
    /// In Caro, blocking opponent threats is MORE important than creating your own.
    /// This multiplier ensures opponent threats are weighted higher than equivalent player threats.
    /// Rationale: In fast time controls, safer to be "paranoid" and block early than miss a VCF.
    /// Effect: Opponent Open 4 = -15,000, My Open 4 = +10,000 -> AI prioritizes blocking.
    ///
    /// NOTE: Reduced from 2.2x to 1.5x to prevent second-mover (Blue) advantage.
    /// 2.2x was too aggressive and caused Blue to consistently win regardless of difficulty difference.
    /// </summary>
    private const float DefenseMultiplier = (float)EvaluationConstants.DefenseMultiplierNumerator / EvaluationConstants.DefenseMultiplierDenominator;

    // Reuse centralized direction vectors
    private static readonly (int dx, int dy)[] Directions = GameConstants.CardinalDirections;

    /// <summary>
    /// Evaluate the board for a given player using BitBoard operations
    /// </summary>
    [MethodImpl(MethodImplOptions.AggressiveInlining)]
    public static int Evaluate(Board board, Player player)
    {
        if (player == Player.None)
            throw new ArgumentException("Player cannot be None");

        var opponent = player == Player.Red ? Player.Blue : Player.Red;
        var playerBoard = board.GetBitBoard(player);
        var opponentBoard = board.GetBitBoard(opponent);

        return EvaluateBitBoard(playerBoard, opponentBoard);
    }

    /// <summary>
    /// Evaluate the SearchBoard for a given player using BitBoard operations.
    /// High-performance path for search that avoids immutable Board overhead.
    /// </summary>
    [MethodImpl(MethodImplOptions.AggressiveInlining)]
    public static int Evaluate(SearchBoard board, Player player)
    {
        if (player == Player.None)
            throw new ArgumentException("Player cannot be None");

        var opponent = player == Player.Red ? Player.Blue : Player.Red;
        var playerBoard = board.GetBitBoard(player);
        var opponentBoard = board.GetBitBoard(opponent);

        return EvaluateBitBoard(playerBoard, opponentBoard);
    }

    /// <summary>
    /// Evaluate using only BitBoard operations (fastest path)
    /// </summary>
    [MethodImpl(MethodImplOptions.AggressiveInlining)]
    public static int EvaluateBitBoard(BitBoard playerBoard, BitBoard opponentBoard)
    {
        var score = 0;
        var occupied = playerBoard | opponentBoard;

        // Evaluate all directions using shift-based pattern detection
        score += EvaluateDirection(playerBoard, occupied, 1, 0);   // Horizontal
        score += EvaluateDirection(playerBoard, occupied, 0, 1);   // Vertical
        score += EvaluateDirection(playerBoard, occupied, 1, 1);   // Diagonal
        score += EvaluateDirection(playerBoard, occupied, 1, -1);  // Anti-diagonal

        // Subtract opponent's threats with DefenseMultiplier (asymmetric scoring)
        // In Caro, blocking opponent threats is MORE important than creating your own attacks
        // Use integer math to avoid floating-point precision issues
        int defenseNumer = EvaluationConstants.DefenseMultiplierNumerator;
        int defenseDenom = EvaluationConstants.DefenseMultiplierDenominator;

        var oppHorizontal = EvaluateDirection(opponentBoard, occupied, 1, 0);
        var oppVertical = EvaluateDirection(opponentBoard, occupied, 0, 1);
        var oppDiagonal = EvaluateDirection(opponentBoard, occupied, 1, 1);
        var oppAntiDiagonal = EvaluateDirection(opponentBoard, occupied, 1, -1);

        score -= (oppHorizontal * defenseNumer) / defenseDenom;
        score -= (oppVertical * defenseNumer) / defenseDenom;
        score -= (oppDiagonal * defenseNumer) / defenseDenom;
        score -= (oppAntiDiagonal * defenseNumer) / defenseDenom;

        // Add center control bonus
        score += EvaluateCenterControl(playerBoard);

        return score;
    }

    #region Parameterized Evaluation (for SPSA tuning)

    /// <summary>
    /// Evaluate the board for a given player using custom parameters
    /// </summary>
    [MethodImpl(MethodImplOptions.AggressiveInlining)]
    public static int EvaluateWithParameters(Board board, Player player, TunableParameters parameters)
    {
        if (player == Player.None)
            throw new ArgumentException("Player cannot be None");

        var opponent = player == Player.Red ? Player.Blue : Player.Red;
        var playerBoard = board.GetBitBoard(player);
        var opponentBoard = board.GetBitBoard(opponent);

        return EvaluateBitBoardWithParameters(playerBoard, opponentBoard, parameters);
    }

    /// <summary>
    /// Evaluate the SearchBoard for a given player using custom parameters.
    /// High-performance path for search that avoids immutable Board overhead.
    /// </summary>
    [MethodImpl(MethodImplOptions.AggressiveInlining)]
    public static int EvaluateWithParameters(SearchBoard board, Player player, TunableParameters parameters)
    {
        if (player == Player.None)
            throw new ArgumentException("Player cannot be None");

        var opponent = player == Player.Red ? Player.Blue : Player.Red;
        var playerBoard = board.GetBitBoard(player);
        var opponentBoard = board.GetBitBoard(opponent);

        return EvaluateBitBoardWithParameters(playerBoard, opponentBoard, parameters);
    }

    /// <summary>
    /// Evaluate using only BitBoard operations with custom parameters (for SPSA tuning)
    /// </summary>
    [MethodImpl(MethodImplOptions.AggressiveInlining)]
    public static int EvaluateBitBoardWithParameters(BitBoard playerBoard, BitBoard opponentBoard, TunableParameters parameters)
    {
        var score = 0;
        var occupied = playerBoard | opponentBoard;

        // Extract parameters (convert to int for scoring)
        var fiveInRow = (int)parameters.FiveInRowScore;
        var openFour = (int)parameters.OpenFourScore;
        var closedFour = (int)parameters.ClosedFourScore;
        var openThree = (int)parameters.OpenThreeScore;
        var closedThree = (int)parameters.ClosedThreeScore;
        var openTwo = (int)parameters.OpenTwoScore;
        var centerBonus = (int)parameters.CenterBonus;

        // Defense multiplier as rational for integer math
        var defMultNumer = (int)Math.Round(parameters.DefenseMultiplier * 100);
        var defMultDenom = 100;

        // Evaluate all directions
        score += EvaluateDirectionWithParams(playerBoard, occupied, 1, 0, fiveInRow, openFour, closedFour, openThree, closedThree, openTwo);
        score += EvaluateDirectionWithParams(playerBoard, occupied, 0, 1, fiveInRow, openFour, closedFour, openThree, closedThree, openTwo);
        score += EvaluateDirectionWithParams(playerBoard, occupied, 1, 1, fiveInRow, openFour, closedFour, openThree, closedThree, openTwo);
        score += EvaluateDirectionWithParams(playerBoard, occupied, 1, -1, fiveInRow, openFour, closedFour, openThree, closedThree, openTwo);

        // Opponent's threats with defense multiplier
        var oppHorizontal = EvaluateDirectionWithParams(opponentBoard, occupied, 1, 0, fiveInRow, openFour, closedFour, openThree, closedThree, openTwo);
        var oppVertical = EvaluateDirectionWithParams(opponentBoard, occupied, 0, 1, fiveInRow, openFour, closedFour, openThree, closedThree, openTwo);
        var oppDiagonal = EvaluateDirectionWithParams(opponentBoard, occupied, 1, 1, fiveInRow, openFour, closedFour, openThree, closedThree, openTwo);
        var oppAntiDiagonal = EvaluateDirectionWithParams(opponentBoard, occupied, 1, -1, fiveInRow, openFour, closedFour, openThree, closedThree, openTwo);

        score -= (oppHorizontal * defMultNumer) / defMultDenom;
        score -= (oppVertical * defMultNumer) / defMultDenom;
        score -= (oppDiagonal * defMultNumer) / defMultDenom;
        score -= (oppAntiDiagonal * defMultNumer) / defMultDenom;

        // Center control bonus
        score += EvaluateCenterControlWithParams(playerBoard, centerBonus);

        return score;
    }

    /// <summary>
    /// Evaluate center control with custom bonus value
    /// </summary>
    [MethodImpl(MethodImplOptions.AggressiveInlining)]
    private static int EvaluateCenterControlWithParams(BitBoard playerBoard, int centerBonus)
    {
        var score = 0;

        for (int x = 5; x <= 9; x++)
        {
            for (int y = 5; y <= 9; y++)
            {
                if (playerBoard.GetBit(x, y))
                {
                    var distanceToCenter = Math.Abs(x - 7) + Math.Abs(y - 7);
                    score += centerBonus - (distanceToCenter * 5);
                }
            }
        }

        return score;
    }

    #endregion

    /// <summary>
    /// Evaluate center control using BitBoard
    /// </summary>
    [MethodImpl(MethodImplOptions.AggressiveInlining)]
    private static int EvaluateCenterControl(BitBoard playerBoard)
    {
        var score = 0;

        // Center zone: 5x5 area from (5,5) to (9,9)
        for (int x = 5; x <= 9; x++)
        {
            for (int y = 5; y <= 9; y++)
            {
                if (playerBoard.GetBit(x, y))
                {
                    // Center cell (7,7) gets highest bonus
                    var distanceToCenter = Math.Abs(x - 7) + Math.Abs(y - 7);
                    score += CenterBonus - (distanceToCenter * 5);
                }
            }
        }

        return score;
    }
}
