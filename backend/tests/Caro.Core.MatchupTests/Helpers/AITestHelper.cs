using Caro.Core.GameLogic;
using Caro.Core.Tournament;

namespace Caro.Core.MatchupTests.Helpers;

public static class AITestHelper
{
    public static MinimaxAI CreateAI(int seed = 42, int ttSizeMb = 256)
    {
        return new MinimaxAI(random: new Random(seed), ttSizeMb: ttSizeMb);
    }

    public static TournamentEngine CreateTournamentEngine(int seed = 42, int ttSizeMb = 256)
    {
        var botA = CreateAI(seed, ttSizeMb);
        var botB = CreateAI(seed + 1, ttSizeMb);
        return new TournamentEngine(botA, botB);
    }

    public static TournamentEngine CreateNonDeterministicEngine(int ttSizeMb = 256)
    {
        return new TournamentEngine(new MinimaxAI(), new MinimaxAI());
    }
}
