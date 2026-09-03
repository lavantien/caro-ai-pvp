using System.Text.Json.Serialization;

namespace Caro.Api;

public sealed class CreateGameRequest
{
    [JsonPropertyName("timeControl")]
    public string TimeControl { get; set; } = "";

    [JsonPropertyName("gameMode")]
    public string GameMode { get; set; } = "";

    [JsonPropertyName("difficulty")]
    public int? Difficulty { get; set; }

    [JsonPropertyName("redDifficulty")]
    public int? RedDifficulty { get; set; }

    [JsonPropertyName("blueDifficulty")]
    public int? BlueDifficulty { get; set; }

    [JsonPropertyName("randomOpening")]
    public bool RandomOpening { get; set; }

    [JsonPropertyName("seed")]
    public long Seed { get; set; }
}

public sealed class MoveRequest
{
    [JsonPropertyName("x")]
    public int X { get; set; }

    [JsonPropertyName("y")]
    public int Y { get; set; }
}

public sealed class GameResponse
{
    [JsonPropertyName("board")]
    public List<CellResponse> Board { get; set; } = [];

    [JsonPropertyName("currentPlayer")]
    public string CurrentPlayer { get; set; } = "";

    [JsonPropertyName("moveNumber")]
    public int MoveNumber { get; set; }

    [JsonPropertyName("isGameOver")]
    public bool IsGameOver { get; set; }

    [JsonPropertyName("winner")]
    public string Winner { get; set; } = "";

    [JsonPropertyName("endReason")]
    public string EndReason { get; set; } = "";

    [JsonPropertyName("winningLine")]
    public List<PositionResponse>? WinningLine { get; set; }

    [JsonPropertyName("redTimeRemaining")]
    public double RedTimeRemaining { get; set; }

    [JsonPropertyName("blueTimeRemaining")]
    public double BlueTimeRemaining { get; set; }

    [JsonPropertyName("timeControl")]
    public string TimeControl { get; set; } = "";

    [JsonPropertyName("initialTime")]
    public int InitialTime { get; set; }

    [JsonPropertyName("increment")]
    public int Increment { get; set; }

    [JsonPropertyName("gameMode")]
    public string GameMode { get; set; } = "";

    [JsonPropertyName("redDifficulty")]
    public int? RedDifficulty { get; set; }

    [JsonPropertyName("blueDifficulty")]
    public int? BlueDifficulty { get; set; }
}

public readonly record struct CellResponse(
    [property: JsonPropertyName("x")] int X,
    [property: JsonPropertyName("y")] int Y,
    [property: JsonPropertyName("player")] string Player);

public readonly record struct PositionResponse(
    [property: JsonPropertyName("x")] int X,
    [property: JsonPropertyName("y")] int Y);

public sealed class EngineStatsResponse
{
    [JsonPropertyName("depth")]
    public int Depth { get; set; }

    [JsonPropertyName("nodes")]
    public long Nodes { get; set; }

    [JsonPropertyName("nps")]
    public double NPS { get; set; }

    [JsonPropertyName("ttHitRate")]
    public double TTHitRate { get; set; }

    [JsonPropertyName("score")]
    public int Score { get; set; }

    [JsonPropertyName("threads")]
    public int Threads { get; set; }

    [JsonPropertyName("allocatedTimeMs")]
    public long AllocatedTimeMs { get; set; }

    [JsonPropertyName("moveType")]
    public string MoveType { get; set; } = "";

    /// <summary>Null when no ponder search preceded the move (L5-only feature).</summary>
    [JsonPropertyName("ponderDepth")]
    public int? PonderDepth { get; set; }

    [JsonPropertyName("ponderNodes")]
    public long? PonderNodes { get; set; }

    /// <summary>Forced-chain length and solver nodes when the move came from the VCF solver.</summary>
    [JsonPropertyName("vcfDepth")]
    public int? VcfDepth { get; set; }

    [JsonPropertyName("vcfNodes")]
    public long? VcfNodes { get; set; }
}

public sealed class MoveDetailResponse
{
    [JsonPropertyName("moveNumber")]
    public int MoveNumber { get; set; }

    [JsonPropertyName("player")]
    public string Player { get; set; } = "";

    [JsonPropertyName("pos")]
    public string Pos { get; set; } = "";

    [JsonPropertyName("statline")]
    public string Statline { get; set; } = "";

    [JsonPropertyName("thinkTimeMs")]
    public long ThinkTimeMs { get; set; }

    [JsonPropertyName("remainingTimeMs")]
    public long RemainingTimeMs { get; set; }

    /// <summary>Null when no ponder search preceded the move; false = pondered and missed.</summary>
    [JsonPropertyName("ponderHit")]
    public bool? PonderHit { get; set; }

    [JsonPropertyName("engineStats")]
    public EngineStatsResponse EngineStats { get; set; } = new();
}

public sealed class ErrorResponse
{
    [JsonPropertyName("error")]
    public string Error { get; set; } = "";

    [JsonPropertyName("message")]
    public string Message { get; set; } = "";
}

public sealed record NewGameResponse(
    [property: JsonPropertyName("gameId")] string GameId,
    [property: JsonPropertyName("state")] GameResponse State);

public sealed record StateResponse(
    [property: JsonPropertyName("state")] GameResponse State);

public sealed record MoveResponse(
    [property: JsonPropertyName("state")] GameResponse State,
    [property: JsonPropertyName("lastMove")] MoveDetailResponse LastMove);

public sealed record DeletedResponse(
    [property: JsonPropertyName("deleted")] bool Deleted);
