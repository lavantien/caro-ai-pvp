using Caro.Core.Domain.Configuration;

namespace Caro.Core.GameLogic.UCI;

/// <summary>
/// Converts between UCI move notation and board coordinates.
///
/// UCI notation: double lowercase letter (column) + number (row)
/// - Columns: 'aa' through 'dd' (0-15 on a 16x16 board)
/// - Rows: '1' through '16' (0-15 internally)
///
/// Encoding: column = firstLetterIndex * 4 + secondLetterIndex
/// - First letter: 'a'-'d' (0-3), Second letter: 'a'-'d' (0-3)
/// Examples: "bb9" = center (7, 8), "aa1" = top-left (0, 0), "dd16" = bottom-right (15, 15)
/// </summary>
public static class UCIMoveNotation
{
    private const int BoardSize = GameConstants.BoardSize;

    /// <summary>
    /// Convert board coordinates to UCI notation.
    /// </summary>
    /// <param name="x">X coordinate (0-15)</param>
    /// <param name="y">Y coordinate (0-15)</param>
    /// <returns>UCI notation string (e.g., "bb9")</returns>
    public static string ToUCI(int x, int y)
    {
        if (!IsValidCoordinate(x, y))
            throw new ArgumentOutOfRangeException(nameof(x), $"Coordinates ({x}, {y}) are outside valid board bounds (0-15)");

        // Column: double letter grid format (aa-dd)
        // Encoding: column = firstLetterIndex * 4 + secondLetterIndex
        int firstLetter = x / 4;   // 0-3 maps to a-d
        int secondLetter = x % 4;  // 0-3 maps to a-d
        string column = $"{(char)('a' + firstLetter)}{(char)('a' + secondLetter)}";
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
    /// <param name="move">UCI move string (e.g., "qg17")</param>
    /// <returns>Position with x, y coordinates</returns>
    public static Caro.Core.Domain.Entities.Position FromUCI(string move)
    {
        if (string.IsNullOrWhiteSpace(move) || move.Length < 3)
            throw new ArgumentException($"Invalid UCI move: '{move}' (expected double-letter column)");

        move = move.ToLowerInvariant();

        // Always expect double-letter column format (aa-dd)
        char col1 = move[0];
        char col2 = move[1];

        if (col1 < 'a' || col1 > 'd' || col2 < 'a' || col2 > 'd')
            throw new ArgumentException($"Invalid column in UCI move: '{move}' (first letter a-d, second letter a-d)");

        int x = (col1 - 'a') * 4 + (col2 - 'a');
        string rowPart = move.Substring(2);

        if (!int.TryParse(rowPart, out int row))
            throw new ArgumentException($"Invalid row in UCI move: '{move}' (must be 1-16)");

        int y = row - 1;

        if (!IsValidCoordinate(x, y))
            throw new ArgumentException($"UCI move out of bounds: '{move}' (board is 16x16, aa1-dd16)");

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
    /// Get column index from UCI double-letter column.
    /// </summary>
    public static int ColumnFromDoubleChar(char first, char second)
        => (first - 'a') * 4 + (second - 'a');

    /// <summary>
    /// Get UCI double-letter column from column index.
    /// </summary>
    public static string ColumnToDoubleChar(int x)
        => $"{(char)('a' + x / 4)}{(char)('a' + x % 4)}";
}
