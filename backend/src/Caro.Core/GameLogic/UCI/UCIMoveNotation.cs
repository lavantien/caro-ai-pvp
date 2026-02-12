using Caro.Core.Domain.Configuration;

namespace Caro.Core.GameLogic.UCI;

/// <summary>
/// Converts between UCI move notation and board coordinates.
///
/// UCI notation: double lowercase letter (column) + number (row)
/// - Columns: 'aa' through 'hd' (0-31 on a 32x32 board)
/// - Rows: '1' through '32' (0-31 internally)
///
/// Encoding: column = firstLetterIndex * 4 + secondLetterIndex
/// - First letter: 'a'-'h' (0-7), Second letter: 'a'-'d' (0-3)
/// Examples: "qg17" = center (16, 16), "aa1" = top-left (0, 0), "hd32" = bottom-right (31, 31)
/// </summary>
public static class UCIMoveNotation
{
    private const int BoardSize = GameConstants.BoardSize;

    /// <summary>
    /// Convert board coordinates to UCI notation.
    /// </summary>
    /// <param name="x">X coordinate (0-31)</param>
    /// <param name="y">Y coordinate (0-31)</param>
    /// <returns>UCI notation string (e.g., "qg17")</returns>
    public static string ToUCI(int x, int y)
    {
        if (!IsValidCoordinate(x, y))
            throw new ArgumentOutOfRangeException(nameof(x), $"Coordinates ({x}, {y}) are outside valid board bounds (0-31)");

        // Column: double letter grid format (aa-hd)
        // Encoding: column = firstLetterIndex * 4 + secondLetterIndex
        int firstLetter = x / 4;   // 0-7 maps to a-h
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

        // Always expect double-letter column format (aa-hd)
        char col1 = move[0];
        char col2 = move[1];

        if (col1 < 'a' || col1 > 'h' || col2 < 'a' || col2 > 'd')
            throw new ArgumentException($"Invalid column in UCI move: '{move}' (first letter a-h, second letter a-d)");

        int x = (col1 - 'a') * 4 + (col2 - 'a');
        string rowPart = move.Substring(2);

        if (!int.TryParse(rowPart, out int row))
            throw new ArgumentException($"Invalid row in UCI move: '{move}' (must be 1-32)");

        int y = row - 1;

        if (!IsValidCoordinate(x, y))
            throw new ArgumentException($"UCI move out of bounds: '{move}' (board is 32x32, aa1-hd32)");

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
