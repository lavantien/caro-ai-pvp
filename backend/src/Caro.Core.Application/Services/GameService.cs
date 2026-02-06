using Caro.Core.Application.DTOs;
using Caro.Core.Application.Interfaces;
using Caro.Core.Application.Mappers;
using Caro.Core.Application.Extensions;
using Caro.Core.Domain.Entities;
using Caro.Core.GameLogic;
using Caro.Core.Domain.Exceptions;
using Microsoft.Extensions.Logging;

namespace Caro.Core.Application.Services;

/// <summary>
/// Application service for game management (Stateless)
/// Orchestrates domain operations and handles business workflows
/// </summary>
public sealed class GameService : IGameService
{
    private readonly IGameRepository _gameRepository;
    private readonly IAIService _aiService;
    private readonly ITimeManagementService _timeService;
    private readonly ILogger<GameService> _logger;

    public GameService(
        IGameRepository gameRepository,
        IAIService aiService,
        ITimeManagementService timeService,
        ILogger<GameService> logger)
    {
        _gameRepository = gameRepository;
        _aiService = aiService;
        _timeService = timeService;
        _logger = logger;
    }

    /// <summary>
    /// Create a new game
    /// </summary>
    public async Task<GameResponse> CreateGameAsync(
        CreateGameRequest request,
        CancellationToken cancellationToken = default)
    {
        try
        {
            var gameId = Guid.NewGuid();
            var (initialTime, increment) = GameMapper.ParseTimeControl(request);

            var initialState = GameStateExtensions.CreateInitial(initialTime, increment);

            await _gameRepository.SaveAsync(gameId, initialState, cancellationToken);

            var response = new GameResponse
            {
                GameId = gameId,
                State = GameMapper.ToDto(initialState, gameId),
                Success = true,
                Message = "Game created successfully",
                ErrorMessage = null
            };

            // Start timer for first player
            if (request.RedPlayerType == "Human")
            {
                await _timeService.StartTimerAsync(gameId, "Red", cancellationToken);
            }

            return response;
        }
        catch (Exception ex)
        {
            _logger.LogError(ex, "Error creating game");
            return new GameResponse
            {
                GameId = Guid.Empty,
                State = null!,
                Success = false,
                ErrorMessage = ex.Message
            };
        }
    }

    /// <summary>
    /// Get a game by ID
    /// </summary>
    public async Task<GameResponse?> GetGameAsync(
        Guid gameId,
        CancellationToken cancellationToken = default)
    {
        try
        {
            var state = await _gameRepository.LoadAsync(gameId, cancellationToken);
            if (state is null)
            {
                return null;
            }

            return new GameResponse
            {
                GameId = gameId,
                State = GameMapper.ToDto(state, gameId),
                Success = true
            };
        }
        catch (Exception ex)
        {
            _logger.LogError(ex, "Error loading game {GameId}", gameId);
            return new GameResponse
            {
                GameId = gameId,
                State = null!,
                Success = false,
                ErrorMessage = ex.Message
            };
        }
    }

    /// <summary>
    /// Make a move in a game
    /// </summary>
    public async Task<GameResponse> MakeMoveAsync(
        Guid gameId,
        MakeMoveRequest request,
        CancellationToken cancellationToken = default)
    {
        try
        {
            var currentState = await _gameRepository.LoadAsync(gameId, cancellationToken);
            if (currentState is null)
            {
                return new GameResponse
                {
                    GameId = gameId,
                    State = null!,
                    Success = false,
                    ErrorMessage = "Game not found"
                };
            }

            // Stop current player's timer
            var currentPlayerStr = currentState.CurrentPlayer.ToString();
            var elapsed = await _timeService.StopTimerAsync(gameId, currentPlayerStr, cancellationToken);
            var updatedState = currentState.WithTimeRemaining(elapsed);

            // Apply move
            var movePosition = new Position(request.X, request.Y);
            if (!movePosition.IsValid())
            {
                return new GameResponse
                {
                    GameId = gameId,
                    State = GameMapper.ToDto(updatedState, gameId),
                    Success = false,
                    ErrorMessage = $"Invalid position ({request.X}, {request.Y})"
                };
            }

            if (!updatedState.Board.GetCell(request.X, request.Y).IsEmpty)
            {
                return new GameResponse
                {
                    GameId = gameId,
                    State = GameMapper.ToDto(updatedState, gameId),
                    Success = false,
                    ErrorMessage = $"Cell ({request.X}, {request.Y}) is already occupied"
                };
            }

            updatedState.RecordMove(request.X, request.Y);

            // Check for win condition
            var winningLine = CheckForWin(updatedState.Board, request.X, request.Y, currentState.CurrentPlayer);
            if (winningLine.Length > 0)
            {
                updatedState = updatedState.WithEndGame(currentState.CurrentPlayer, winningLine);
            }

            // Check for draw
            if (!updatedState.IsGameOver && updatedState.Board.IsFull())
            {
                updatedState = updatedState.WithEndGame(Player.None);
            }

            await _gameRepository.SaveAsync(gameId, updatedState, cancellationToken);

            // Start next player's timer if game not over
            if (!updatedState.IsGameOver)
            {
                await _timeService.StartTimerAsync(gameId, updatedState.CurrentPlayer.ToString(), cancellationToken);
            }

            return new GameResponse
            {
                GameId = gameId,
                State = GameMapper.ToDto(updatedState, gameId),
                Success = true,
                Message = $"Move made at ({request.X}, {request.Y})"
            };
        }
        catch (InvalidMoveException ex)
        {
            return new GameResponse
            {
                GameId = gameId,
                State = null!,
                Success = false,
                ErrorMessage = ex.Message
            };
        }
        catch (GameOverException ex)
        {
            return new GameResponse
            {
                GameId = gameId,
                State = null!,
                Success = false,
                ErrorMessage = ex.Message
            };
        }
        catch (Exception ex)
        {
            _logger.LogError(ex, "Error making move in game {GameId}", gameId);
            return new GameResponse
            {
                GameId = gameId,
                State = null!,
                Success = false,
                ErrorMessage = "An error occurred while making the move"
            };
        }
    }

    /// <summary>
    /// Undo the last move
    /// </summary>
    public async Task<GameResponse> UndoMoveAsync(
        Guid gameId,
        CancellationToken cancellationToken = default)
    {
        try
        {
            var currentState = await _gameRepository.LoadAsync(gameId, cancellationToken);
            if (currentState is null)
            {
                return new GameResponse
                {
                    GameId = gameId,
                    State = null!,
                    Success = false,
                    ErrorMessage = "Game not found"
                };
            }

            if (!currentState.CanUndo())
            {
                return new GameResponse
                {
                    GameId = gameId,
                    State = GameMapper.ToDto(currentState, gameId),
                    Success = false,
                    ErrorMessage = "Cannot undo - no moves or game is over"
                };
            }

            currentState.UndoMove();
            var undoneState = currentState;
            await _gameRepository.SaveAsync(gameId, undoneState, cancellationToken);

            return new GameResponse
            {
                GameId = gameId,
                State = GameMapper.ToDto(undoneState, gameId),
                Success = true,
                Message = "Move undone successfully"
            };
        }
        catch (Exception ex)
        {
            _logger.LogError(ex, "Error undoing move in game {GameId}", gameId);
            return new GameResponse
            {
                GameId = gameId,
                State = null!,
                Success = false,
                ErrorMessage = ex.Message
            };
        }
    }

    /// <summary>
    /// Resign from a game
    /// </summary>
    public async Task<GameResponse> ResignAsync(
        Guid gameId,
        string player,
        CancellationToken cancellationToken = default)
    {
        try
        {
            var currentState = await _gameRepository.LoadAsync(gameId, cancellationToken);
            if (currentState is null)
            {
                return new GameResponse
                {
                    GameId = gameId,
                    State = null!,
                    Success = false,
                    ErrorMessage = "Game not found"
                };
            }

            if (currentState.IsGameOver)
            {
                return new GameResponse
                {
                    GameId = gameId,
                    State = GameMapper.ToDto(currentState, gameId),
                    Success = false,
                    ErrorMessage = "Game is already over"
                };
            }

            var resigningPlayer = GameMapper.ToPlayer(player);
            var winner = resigningPlayer.Opponent();
            var endedState = currentState.WithEndGame(winner);

            await _gameRepository.SaveAsync(gameId, endedState, cancellationToken);

            return new GameResponse
            {
                GameId = gameId,
                State = GameMapper.ToDto(endedState, gameId),
                Success = true,
                Message = $"{player} resigned. {winner} wins!"
            };
        }
        catch (Exception ex)
        {
            _logger.LogError(ex, "Error resigning from game {GameId}", gameId);
            return new GameResponse
            {
                GameId = gameId,
                State = null!,
                Success = false,
                ErrorMessage = ex.Message
            };
        }
    }

    /// <summary>
    /// Request AI move calculation
    /// </summary>
    public async Task<AIMoveResponse> GetAIMoveAsync(
        Guid gameId,
        CancellationToken cancellationToken = default)
    {
        try
        {
            var state = await _gameRepository.LoadAsync(gameId, cancellationToken);
            if (state is null)
            {
                throw new InvalidOperationException("Game not found");
            }

            // Default difficulty if not specified
            var difficulty = "Medium"; // TODO: Get from game config

            return await _aiService.CalculateBestMoveAsync(state, difficulty, cancellationToken);
        }
        catch (Exception ex)
        {
            _logger.LogError(ex, "Error calculating AI move for game {GameId}", gameId);
            throw;
        }
    }

    /// <summary>
    /// Get list of active games
    /// </summary>
    public async Task<GameListDto> GetGamesAsync(CancellationToken cancellationToken = default)
    {
        var gameIds = await _gameRepository.GetAllIdsAsync(cancellationToken);
        var summaries = new List<GameSummaryDto>();

        foreach (var gameId in gameIds)
        {
            var state = await _gameRepository.LoadAsync(gameId, cancellationToken);
            if (state is not null)
            {
                summaries.Add(new GameSummaryDto
                {
                    GameId = gameId,
                    Status = state.IsGameOver ? "Completed" : "Active",
                    CurrentPlayer = state.CurrentPlayer.ToString(),
                    MoveNumber = state.MoveNumber,
                    Winner = state.Winner != Player.None ? state.Winner.ToString() : null
                });
            }
        }

        return new GameListDto
        {
            Games = summaries.ToArray()
        };
    }

    /// <summary>
    /// Delete a game
    /// </summary>
    public async Task<bool> DeleteGameAsync(Guid gameId, CancellationToken cancellationToken = default)
    {
        try
        {
            return await _gameRepository.DeleteAsync(gameId, cancellationToken);
        }
        catch (Exception ex)
        {
            _logger.LogError(ex, "Error deleting game {GameId}", gameId);
            return false;
        }
    }

    /// <summary>
    /// Check for a win condition after a move (Pure function - stateless)
    /// </summary>
    private static ReadOnlyMemory<Position> CheckForWin(
        Board board,
        int lastX,
        int lastY,
        Player player)
    {
        const int WinLength = 5;

        // Check all four directions
        var directions = new[]
        {
            (dx: 1, dy: 0),   // Horizontal
            (dx: 0, dy: 1),   // Vertical
            (dx: 1, dy: 1),   // Diagonal down-right
            (dx: 1, dy: -1)   // Diagonal up-right
        };

        foreach (var (dx, dy) in directions)
        {
            var line = GetLine(board, lastX, lastY, dx, dy, WinLength, player);
            if (line.Length >= WinLength)
            {
                return line;
            }
        }

        return ReadOnlyMemory<Position>.Empty;
    }

    /// <summary>
    /// Get a line of stones in a direction (Pure function - stateless)
    /// </summary>
    private static ReadOnlyMemory<Position> GetLine(
        Board board,
        int startX,
        int startY,
        int dx,
        int dy,
        int targetLength,
        Player player)
    {
        var positions = new List<Position>();

        // Count in positive direction
        var x = startX + dx;
        var y = startY + dy;
        var boardSize = board.BoardSize;
        while (x >= 0 && x < boardSize && y >= 0 && y < boardSize &&
               board.GetCell(x, y).Player == player)
        {
            positions.Add(new Position(x, y));
            x += dx;
            y += dy;
        }

        // Add starting position
        positions.Add(new Position(startX, startY));

        // Count in negative direction
        x = startX - dx;
        y = startY - dy;
        while (x >= 0 && x < boardSize && y >= 0 && y < boardSize &&
               board.GetCell(x, y).Player == player)
        {
            positions.Add(new Position(x, y));
            x -= dx;
            y -= dy;
        }

        return positions.Count >= targetLength
            ? positions.ToArray().AsMemory()
            : ReadOnlyMemory<Position>.Empty;
    }
}
