using Caro.Domain;

namespace Caro.Engine;

public static class Evaluation
{
    public static int Evaluate(SearchBoard sb, Player player)
    {
        int playerScore = EvaluateForPlayer(sb, player);
        int opponentScore = EvaluateForPlayer(sb, player.Opponent());

        int score = playerScore - opponentScore;
        score += CenterBonus(sb, player) - CenterBonus(sb, player.Opponent());

        if (score > Constants.Score.MaxCorrectedEval)
        {
            score = Constants.Score.MaxCorrectedEval;
        }
        if (score < -Constants.Score.MaxCorrectedEval)
        {
            score = -Constants.Score.MaxCorrectedEval;
        }
        return score;
    }

    private static int EvaluateForPlayer(SearchBoard sb, Player player)
    {
        PlayerPattern4 pp = Pattern4Classifier.ClassifyBoard(sb, player);

        if (pp.Exactly5Count > 0)
        {
            return Constants.Score.FiveScore;
        }

        if (pp.Flex4Count > 0)
        {
            int score = Constants.Eval.Flex4WinBonus;
            score += pp.Block4Count * Constants.Eval.Block4Score;
            score += pp.Flex3Count * Constants.Eval.Flex3Score;
            return score;
        }

        if (pp.Block4Count >= 2)
        {
            int score = Constants.Eval.DoubleB4Bonus;
            score += pp.Block4Count * Constants.Eval.Block4Score;
            score += pp.Flex3Count * Constants.Eval.Flex3Score;
            return score;
        }

        // Cascade is strictly descending per ENGINE_FEATURES 5.3: flex4 15k,
        // double block4 14k, block4+flex3 13k, double flex3 12k. A position
        // with both a block4 and two flex3s is the higher B4+F3 category.
        if (pp.Block4Count >= 1 && pp.Flex3Count >= 1)
        {
            int score = Constants.Eval.B4F3Bonus;
            score += pp.Block4Count * Constants.Eval.Block4Score;
            score += pp.Flex3Count * Constants.Eval.Flex3Score;
            return score;
        }

        if (pp.Flex3Count >= 2)
        {
            int score = Constants.Eval.DoubleF3Bonus;
            score += pp.Block4Count * Constants.Eval.Block4Score;
            score += pp.Flex3Count * Constants.Eval.Flex3Score;
            return score;
        }

        int result = 0;
        result += pp.Flex4Count * Constants.Eval.Flex4Score;
        result += pp.Block4Count * Constants.Eval.Block4Score;
        result += pp.Flex3Count * Constants.Eval.Flex3Score;
        result += pp.Block3Count * Constants.Eval.Block3Score;
        result += pp.Flex2Count * Constants.Eval.Flex2Score;
        result += pp.Block2Count * Constants.Eval.Block2Score;

        BitBoard bits = sb.BitBoardFor(player);
        for (int x = 0; x < Constants.Board.Size; x++)
        {
            for (int y = 0; y < Constants.Board.Size; y++)
            {
                if (bits.Get(x, y))
                {
                    result += Constants.Eval.Flex1Score;
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
                    bonus += (Constants.Board.Size - dist) * Constants.Eval.CenterBonusWeight;
                }
            }
        }
        return bonus;
    }
}
