using Caro.Core.Application.DTOs;
using Caro.Core.Domain.Entities;
using Caro.Core.GameLogic;

namespace Caro.Core.Application.Mappers;

/// <summary>
/// Static mapper for converting between domain entities and DTOs
/// Stateless pure functions for immutability
/// </summary>
public static class GameMapper
{
    /// <summary>
    /// Convert GameState to GameDto
    /// </summary>
    public static GameDto ToDto(GameState state, Guid gameId)
    {
        return new GameDto
        {
            GameId = gameId,
            Board = ToBoardDto(state.Board),
            CurrentPlayer = state.CurrentPlayer.ToString(),
            MoveNumber = state.MoveNumber,
            IsGameOver = state.IsGameOver,
            Winner = state.Winner == Player.None ? null : state.Winner.ToString(),
            WinningLine = ToPositionDtos(state.WinningLine),
            RedTimeRemaining = "03:00",
            BlueTimeRemaining = "03:00",
            MoveHistory = ToMoveDtos(state.MoveHistory.ToList())
        };
    }

    /// <summary>
    /// Convert Board to BoardDto
    /// </summary>
    public static BoardDto ToBoardDto(Board board)
    {
        var boardSize = board.BoardSize;
        var cells = new string[boardSize * boardSize];
        for (int y = 0; y < boardSize; y++)
        {
            for (int x = 0; x < boardSize; x++)
            {
                cells[y * boardSize + x] = board.GetCell(x, y).ToString();
            }
        }

        return new BoardDto
        {
            Cells = cells,
            Hash = board.GetHash()
        };
    }

    /// <summary>
    /// Convert IList<Position> to PositionDto array
    /// </summary>
    public static PositionDto[] ToPositionDtos(IList<Position> positions)
    {
        if (positions.Count == 0) return Array.Empty<PositionDto>();

        var result = new PositionDto[positions.Count];
        for (int i = 0; i < positions.Count; i++)
        {
            result[i] = new PositionDto(positions[i].X, positions[i].Y);
        }
        return result;
    }

    /// <summary>
    /// Convert IList<Position> to MoveDto array
    /// </summary>
    public static MoveDto[] ToMoveDtos(IList<Position> moves)
    {
        if (moves.Count == 0) return Array.Empty<MoveDto>();

        var result = new MoveDto[moves.Count];
        for (int i = 0; i < moves.Count; i++)
        {
            result[i] = new MoveDto
            {
                X = moves[i].X,
                Y = moves[i].Y,
                Player = "",  // Filled by caller
                MoveNumber = i + 1
            };
        }
        return result;
    }

    /// <summary>
    /// Convert PositionDto to Position
    /// </summary>
    public static Position ToPosition(PositionDto dto) => new(dto.X, dto.Y);

    /// <summary>
    /// Convert string to Player
    /// </summary>
    public static Player ToPlayer(string playerStr) =>
        Enum.TryParse<Player>(playerStr, ignoreCase: true, out var player) && player.IsValid()
            ? player
            : Player.None;

    /// <summary>
    /// Parse time control string to TimeSpan
    /// </summary>
    public static (TimeSpan initialTime, TimeSpan increment) ParseTimeControl(CreateGameRequest request)
    {
        var initialTime = TimeSpan.FromMinutes(request.InitialTimeMinutes);
        var increment = TimeSpan.FromSeconds(request.IncrementSeconds);
        return (initialTime, increment);
    }

    /// <summary>
    /// Create AIMoveResponse from search result
    /// </summary>
    public static AIMoveResponse ToAIMoveResponse(
        int x, int y,
        int depth,
        long nodes,
        double nps,
        long timeMs,
        int score,
        bool pondering)
    {
        return new AIMoveResponse
        {
            X = x,
            Y = y,
            DepthAchieved = depth,
            NodesSearched = nodes,
            NodesPerSecond = nps,
            TimeTakenMs = timeMs,
            Score = score,
            PonderingActive = pondering
        };
    }
}
