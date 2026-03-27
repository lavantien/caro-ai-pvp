using Caro.Core.GameLogic;
using Caro.Core.Tournament;

namespace Caro.Core.MatchupTests.Helpers;

public static class TournamentEngineTestFactory
{
    public static TournamentEngine Create()
    {
        return new TournamentEngine(
            new MinimaxAI(),
            new MinimaxAI()
        );
    }
}
