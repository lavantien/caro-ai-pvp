using Caro.Domain;

namespace Caro.Engine;

public static class Evaluation
{
    internal const int FiveScore = Constants.Score.WinScore;
    internal const int Flex4WinBonus = 15_000;
    private const int DoubleB4Bonus = 14_000;
    private const int B4F3Bonus = 13_000;
    private const int DoubleF3Bonus = 12_000;
    private const int Flex4Score = 10_000;
    private const int Block4Score = 5_000;
    internal const int Flex3Score = 1_000;
    private const int Block3Score = 100;
    private const int Flex2Score = 100;
    private const int Block2Score = 30;
    private const int Flex1Score = 10;
    private const int CenterBonusWeight = 2;

    internal const int MaxCorrectedEval = Constants.Score.MaxEval;

    public static int Evaluate(SearchBoard sb, Player player)
    {
        int playerScore = EvaluateForPlayer(sb, player);
        int opponentScore = EvaluateForPlayer(sb, player.Opponent());

        int score = playerScore - opponentScore;
        score += CenterBonus(sb, player) - CenterBonus(sb, player.Opponent());

        if (score > MaxCorrectedEval)
        {
            score = MaxCorrectedEval;
        }
        if (score < -MaxCorrectedEval)
        {
            score = -MaxCorrectedEval;
        }
        return score;
    }

    private static int EvaluateForPlayer(SearchBoard sb, Player player)
    {
        PlayerPattern4 pp = Pattern4Classifier.ClassifyBoard(sb, player);

        if (pp.Exactly5Count > 0)
        {
            return FiveScore;
        }

        if (pp.Flex4Count > 0)
        {
            int score = Flex4WinBonus;
            score += pp.Block4Count * Block4Score;
            score += pp.Flex3Count * Flex3Score;
            return score;
        }

        if (pp.Block4Count >= 2)
        {
            int score = DoubleB4Bonus;
            score += pp.Block4Count * Block4Score;
            score += pp.Flex3Count * Flex3Score;
            return score;
        }

        // Cascade is strictly descending per ENGINE_FEATURES 5.3: flex4 15k,
        // double block4 14k, block4+flex3 13k, double flex3 12k. A position
        // with both a block4 and two flex3s is the higher B4+F3 category.
        if (pp.Block4Count >= 1 && pp.Flex3Count >= 1)
        {
            int score = B4F3Bonus;
            score += pp.Block4Count * Block4Score;
            score += pp.Flex3Count * Flex3Score;
            return score;
        }

        if (pp.Flex3Count >= 2)
        {
            int score = DoubleF3Bonus;
            score += pp.Block4Count * Block4Score;
            score += pp.Flex3Count * Flex3Score;
            return score;
        }

        int result = 0;
        result += pp.Flex4Count * Flex4Score;
        result += pp.Block4Count * Block4Score;
        result += pp.Flex3Count * Flex3Score;
        result += pp.Block3Count * Block3Score;
        result += pp.Flex2Count * Flex2Score;
        result += pp.Block2Count * Block2Score;

        BitBoard bits = sb.BitBoardFor(player);
        for (int x = 0; x < Constants.Board.Size; x++)
        {
            for (int y = 0; y < Constants.Board.Size; y++)
            {
                if (bits.Get(x, y))
                {
                    result += Flex1Score;
                }
            }
        }

        return result;
    }

    private static int CenterBonus(SearchBoard sb, Player player)
    {
        int center = Constants.Board.Size / 2;
        int bonus = 0;
        BitBoard bits = sb.BitBoardFor(player);
        for (int x = 0; x < Constants.Board.Size; x++)
        {
            for (int y = 0; y < Constants.Board.Size; y++)
            {
                if (bits.Get(x, y))
                {
                    int dist = EngineMath.Abs(x - center) + EngineMath.Abs(y - center);
                    bonus += (Constants.Board.Size - dist) * CenterBonusWeight;
                }
            }
        }
        return bonus;
    }
}
