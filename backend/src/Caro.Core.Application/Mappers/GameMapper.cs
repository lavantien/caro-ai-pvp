using Caro.Core.Application.DTOs;
using Caro.Core.Domain.Entities;
using Caro.Core.Domain.ValueObjects;

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
            RedTimeRemaining = state.RedTimeRemaining.ToString(@"mm\:ss"),
            BlueTimeRemaining = state.BlueTimeRemaining.ToString(@"mm\:ss"),
            MoveHistory = ToMoveDtos(state.MoveHistory)
        };
    }

    /// <summary>
    /// Convert Board to BoardDto
    /// </summary>
    public static BoardDto ToBoardDto(Board board)
    {
        var cells = new string[Board.BoardSize * Board.BoardSize];
        for (int y = 0; y < Board.BoardSize; y++)
        {
            for (int x = 0; x < Board.BoardSize; x++)
            {
                cells[y * Board.BoardSize + x] = board.GetCell(x, y).ToString();
            }
        }

        return new BoardDto
        {
            Cells = cells,
            Hash = board.Hash
        };
    }

    /// <summary>
    /// Convert ReadOnlyMemory<Position> to PositionDto array
    /// </summary>
    public static PositionDto[] ToPositionDtos(ReadOnlyMemory<Position> positions)
    {
        if (positions.IsEmpty) return Array.Empty<PositionDto>();

        var result = new PositionDto[positions.Length];
        var span = positions.Span;
        for (int i = 0; i < positions.Length; i++)
        {
            result[i] = new PositionDto(span[i].X, span[i].Y);
        }
        return result;
    }

    /// <summary>
    /// Convert ReadOnlyMemory<AnnotatedMove> to MoveDto array
    /// </summary>
    public static MoveDto[] ToMoveDtos(ReadOnlyMemory<AnnotatedMove> moves)
    {
        if (moves.IsEmpty) return Array.Empty<MoveDto>();

        var result = new MoveDto[moves.Length];
        var span = moves.Span;
        for (int i = 0; i < moves.Length; i++)
        {
            result[i] = new MoveDto
            {
                X = span[i].Move.X,
                Y = span[i].Move.Y,
                Player = span[i].Move.Player.ToString(),
                MoveNumber = span[i].MoveNumber
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
