using Caro.Core.GameLogic;
using Caro.Core.Tournament;
using Microsoft.Extensions.Logging.Abstractions;

namespace Caro.TournamentRunner;

public static class TournamentEngineFactory
{
    public static TournamentEngine Create()
    {
        return new TournamentEngine(
            new MinimaxAI(),
            new MinimaxAI()
        );
    }
}
