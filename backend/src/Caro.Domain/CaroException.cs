namespace Caro.Domain;

public class CaroException(string message) : Exception(message);

public sealed class CellOccupiedException()
    : CaroException("cell already occupied");

public sealed class PositionBoundsException()
    : CaroException("position out of bounds");

public sealed class GameOverException()
    : CaroException("game is over");

public sealed class OpenRuleException()
    : CaroException("open rule violation");

public sealed class GameNotFoundException()
    : CaroException("game not found");

public sealed class TooManyGamesException()
    : CaroException("too many concurrent games");

public sealed class InvalidLevelException()
    : CaroException($"difficulty must be {Constants.Difficulty.MinLevel}-{Constants.Difficulty.MaxLevel}");

public sealed class NoMovesException()
    : CaroException("no moves to undo");

public sealed class NotPlayerTurnException()
    : CaroException("move rejected: it is not the human player's turn");
