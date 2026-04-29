using Caro.Core.GameLogic;
using Microsoft.Extensions.Logging;

namespace Caro.Core.IntegrationTests.Helpers;

public static class AITestHelper
{
    public static MinimaxAI CreateAI(int ttSizeMb = 256, ILogger<MinimaxAI>? logger = null)
    {
        return CreateAI(random: null, ttSizeMb, logger);
    }

    public static MinimaxAI CreateAI(Random? random, int ttSizeMb = 256, ILogger<MinimaxAI>? logger = null)
    {
        return new MinimaxAI(ttSizeMb, logger, random);
    }

    public static MinimaxAI CreateDeterministicAI(int seed = 42, int ttSizeMb = 256, ILogger<MinimaxAI>? logger = null)
    {
        return CreateAI(new Random(seed), ttSizeMb, logger);
    }

    public static void WithAI(Action<MinimaxAI> action, int ttSizeMb = 256, ILogger<MinimaxAI>? logger = null)
    {
        using var ai = CreateAI(ttSizeMb, logger);
        action(ai);
    }

    public static T WithAI<T>(Func<MinimaxAI, T> func, int ttSizeMb = 256, ILogger<MinimaxAI>? logger = null)
    {
        using var ai = CreateAI(ttSizeMb, logger);
        return func(ai);
    }
}
