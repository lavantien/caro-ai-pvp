namespace Caro.Core.Application.DTOs;

/// <summary>
/// Data transfer object for game state
/// </summary>
public sealed record GameDto
{
    public Guid GameId { get; init; }
    public BoardDto Board { get; init; } = null!;
    public string CurrentPlayer { get; init; } = null!;
    public int MoveNumber { get; init; }
    public bool IsGameOver { get; init; }
    public string? Winner { get; init; }
    public PositionDto[]? WinningLine { get; init; }
    public string RedTimeRemaining { get; init; } = null!;
    public string BlueTimeRemaining { get; init; } = null!;
    public MoveDto[] MoveHistory { get; init; } = null!;
}

/// <summary>
/// Data transfer object for board state
/// </summary>
public sealed record BoardDto
{
    public string[] Cells { get; init; } = null!;
    public ulong Hash { get; init; }
}

/// <summary>
/// Data transfer object for a position
/// </summary>
public sealed record PositionDto(int X, int Y);

/// <summary>
/// Data transfer object for a move
/// </summary>
public sealed record MoveDto
{
    public int X { get; init; }
    public int Y { get; init; }
    public string Player { get; init; } = null!;
    public int MoveNumber { get; init; }
}

/// <summary>
/// Data transfer object for creating a new game
/// </summary>
public sealed record CreateGameRequest
{
    public string TimeControl { get; init; } = null!;
    public int InitialTimeMinutes { get; init; } = 7;
    public int IncrementSeconds { get; init; } = 5;
    public string RedPlayerType { get; init; } = null!;
    public string BluePlayerType { get; init; } = null!;
    public string? RedAIDifficulty { get; init; }
    public string? BlueAIDifficulty { get; init; }
}

/// <summary>
/// Data transfer object for making a move
/// </summary>
public sealed record MakeMoveRequest
{
    public int X { get; init; }
    public int Y { get; init; }
}

/// <summary>
/// Data transfer object for undoing a move
/// </summary>
public sealed record UndoMoveRequest;

/// <summary>
/// Data transfer object for game response
/// </summary>
public sealed record GameResponse
{
    public Guid GameId { get; init; }
    public GameDto State { get; init; } = null!;
    public bool Success { get; init; }
    public string? Message { get; init; }
    public string? ErrorMessage { get; init; }
}

/// <summary>
/// Data transfer object for AI move result
/// </summary>
public sealed record AIMoveResponse
{
    public int X { get; init; }
    public int Y { get; init; }
    public int DepthAchieved { get; init; }
    public long NodesSearched { get; init; }
    public double NodesPerSecond { get; init; }
    public long TimeTakenMs { get; init; }
    public int Score { get; init; }
    public bool PonderingActive { get; init; }
}

/// <summary>
/// Data transfer object for game list
/// </summary>
public sealed record GameListDto
{
    public GameSummaryDto[] Games { get; init; } = null!;
}

/// <summary>
/// Data transfer object for game summary
/// </summary>
public sealed record GameSummaryDto
{
    public Guid GameId { get; init; }
    public string Status { get; init; } = null!;
    public string CurrentPlayer { get; init; } = null!;
    public int MoveNumber { get; init; }
    public string? Winner { get; init; }
}
