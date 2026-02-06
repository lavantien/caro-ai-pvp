using Caro.Core.Domain.Entities;

namespace Caro.Core.GameLogic.UCI;

/// <summary>
/// Interface for UCI protocol handlers.
/// Implemented by console app and API WebSocket handler.
/// </summary>
public interface IUCIProtocolHandler
{
    /// <summary>
    /// Handle a UCI command and return the response.
    /// </summary>
    /// <param name="command">UCI command string</param>
    /// <returns>Response lines to send</returns>
    string[] HandleCommand(string command);

    /// <summary>
    /// Check if the handler is ready to accept commands.
    /// </summary>
    bool IsReady { get; }

    /// <summary>
    /// Get the current engine state.
    /// </summary>
    UCIEngineState State { get; }

    /// <summary>
    /// Reset the engine to initial state.
    /// </summary>
    void Reset();
}

/// <summary>
/// Current state of the UCI engine.
/// </summary>
public enum UCIEngineState
{
    /// <summary>
    /// Engine just started, waiting for uci command.
    /// </summary>
    Idle,

    /// <summary>
    /// UCI protocol initialized, ready for game commands.
    /// </summary>
    Ready,

    /// <summary>
    /// Currently searching for a move.
    /// </summary>
    Searching,

    /// <summary>
    /// Game over or engine stopped.
    /// </summary>
    Stopped
}

/// <summary>
/// UCI protocol message types.
/// </summary>
public enum UCIMessageType
{
    /// <summary>
    /// From GUI to engine: identify engine
    /// </summary>
    Uci,

    /// <summary>
    /// From GUI to engine: check if ready
    /// </summary>
    IsReady,

    /// <summary>
    /// From GUI to engine: start new game
    /// </summary>
    UciNewGame,

    /// <summary>
    /// From GUI to engine: set position
    /// </summary>
    Position,

    /// <summary>
    /// From GUI to engine: start search
    /// </summary>
    Go,

    /// <summary>
    /// From GUI to engine: stop search
    /// </summary>
    Stop,

    /// <summary>
    /// From GUI to engine: set option
    /// </summary>
    SetOption,

    /// <summary>
    /// From GUI to engine: quit
    /// </summary>
    Quit,

    /// <summary>
    /// From engine to GUI: engine identification
    /// </summary>
    Id,

    /// <summary>
    /// From engine to GUI: options list
    /// </summary>
    UciOk,

    /// <summary>
    /// From engine to GUI: ready confirmation
    /// </summary>
    ReadyOk,

    /// <summary>
    /// From engine to GUI: best move found
    /// </summary>
    BestMove,

    /// <summary>
    /// From engine to GUI: search information
    /// </summary>
    Info
}

/// <summary>
/// Represents a UCI message with type and content.
/// </summary>
public sealed record UCIMessage(UCIMessageType Type, string[] Content);
