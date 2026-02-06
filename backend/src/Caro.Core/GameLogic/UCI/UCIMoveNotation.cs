namespace Caro.Core.GameLogic.UCI;

/// <summary>
/// Converts between UCI move notation and board coordinates.
/// 
/// UCI notation: lowercase letter (column) + number (row)
/// - Columns: 'a' through 's' (0-18 on a 19x19 board)
/// - Rows: '1' through '19' (0-18 internally)
/// 
/// Examples: "j10" = center (9, 9), "a1" = top-left (0, 0), "s19" = bottom-right (18, 18)
/// </summary>
public static class UCIMoveNotation
{
    private const int BoardSize = 19;

    /// <summary>
    /// Convert board coordinates to UCI notation.
    /// </summary>
    /// <param name="x">X coordinate (0-18)</param>
    /// <param name="y">Y coordinate (0-18)</param>
    /// <returns>UCI notation string (e.g., "j10")</returns>
    public static string ToUCI(int x, int y)
    {
        if (!IsValidCoordinate(x, y))
            throw new ArgumentOutOfRangeException(nameof(x), $"Coordinates ({x}, {y}) are outside valid board bounds (0-18)");

        // Column: 'a' + x, Row: y + 1 (1-indexed in UCI)
        char column = (char)('a' + x);
        int row = y + 1;

        return $"{column}{row}";
    }

    /// <summary>
    /// Convert position to UCI notation.
    /// </summary>
    public static string ToUCI(Caro.Core.Domain.Entities.Position position)
        => ToUCI(position.X, position.Y);

    /// <summary>
    /// Parse UCI notation to board coordinates.
    /// </summary>
    /// <param name="move">UCI move string (e.g., "j10")</param>
    /// <returns>Position with x, y coordinates</returns>
    public static Caro.Core.Domain.Entities.Position FromUCI(string move)
    {
        if (string.IsNullOrWhiteSpace(move) || move.Length < 2)
            throw new ArgumentException($"Invalid UCI move: '{move}'");

        move = move.ToLowerInvariant();

        char column = move[0];
        string rowPart = move.Substring(1);

        if (!char.IsLetter(column) || column < 'a' || column > 's')
            throw new ArgumentException($"Invalid column in UCI move: '{move}' (must be a-s)");

        if (!int.TryParse(rowPart, out int row))
            throw new ArgumentException($"Invalid row in UCI move: '{move}' (must be 1-19)");

        int x = column - 'a';
        int y = row - 1;

        if (!IsValidCoordinate(x, y))
            throw new ArgumentException($"UCI move out of bounds: '{move}' (board is 19x19, a1-s19)");

        return new Caro.Core.Domain.Entities.Position(x, y);
    }

    /// <summary>
    /// Check if coordinates are within valid board bounds.
    /// </summary>
    public static bool IsValidCoordinate(int x, int y)
        => x >= 0 && x < BoardSize && y >= 0 && y < BoardSize;

    /// <summary>
    /// Validate UCI move string without throwing.
    /// </summary>
    public static bool IsValidMove(string move)
    {
        try
        {
            FromUCI(move);
            return true;
        }
        catch
        {
            return false;
        }
    }

    /// <summary>
    /// Get column index from UCI column character.
    /// </summary>
    public static int ColumnFromChar(char column) => column - 'a';

    /// <summary>
    /// Get UCI column character from column index.
    /// </summary>
    public static char ColumnToChar(int x) => (char)('a' + x);
}
