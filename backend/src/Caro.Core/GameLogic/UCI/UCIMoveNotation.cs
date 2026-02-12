namespace Caro.Core.GameLogic.UCI;

/// <summary>
/// Converts between UCI move notation and board coordinates.
///
/// UCI notation: lowercase letter (column) + number (row)
/// - Columns: 'a' through 'af' (0-31 on a 32x32 board)
/// - Rows: '1' through '32' (0-31 internally)
///
/// Examples: "q17" = center (16, 16), "a1" = top-left (0, 0), "af32" = bottom-right (31, 31)
/// For columns beyond 'z' (26+), use double letters: aa=26, ab=27, ..., af=31
/// </summary>
public static class UCIMoveNotation
{
    private const int BoardSize = 32;

    /// <summary>
    /// Convert board coordinates to UCI notation.
    /// </summary>
    /// <param name="x">X coordinate (0-31)</param>
    /// <param name="y">Y coordinate (0-31)</param>
    /// <returns>UCI notation string (e.g., "q17")</returns>
    public static string ToUCI(int x, int y)
    {
        if (!IsValidCoordinate(x, y))
            throw new ArgumentOutOfRangeException(nameof(x), $"Coordinates ({x}, {y}) are outside valid board bounds (0-31)");

        // Column: single letter 'a'-'z' for 0-25, double letter 'aa'-'af' for 26-31
        string column = x < 26 ? ((char)('a' + x)).ToString() : "a" + (char)('a' + x - 26);
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
    /// <param name="move">UCI move string (e.g., "q17")</param>
    /// <returns>Position with x, y coordinates</returns>
    public static Caro.Core.Domain.Entities.Position FromUCI(string move)
    {
        if (string.IsNullOrWhiteSpace(move) || move.Length < 2)
            throw new ArgumentException($"Invalid UCI move: '{move}'");

        move = move.ToLowerInvariant();

        // Parse column (single letter 'a'-'z' or double letter 'aa'-'af')
        int x;
        string rowPart;

        if (move.Length >= 3 && char.IsLetter(move[1]) && !char.IsDigit(move[1]))
        {
            // Double letter column (aa-af for columns 26-31)
            char col1 = move[0];
            char col2 = move[1];
            if (col1 != 'a' || col2 < 'a' || col2 > 'f')
                throw new ArgumentException($"Invalid column in UCI move: '{move}' (double-letter columns must be aa-af)");
            x = 26 + (col2 - 'a');
            rowPart = move.Substring(2);
        }
        else
        {
            // Single letter column (a-z for columns 0-25)
            char column = move[0];
            if (!char.IsLetter(column) || column < 'a' || column > 'z')
                throw new ArgumentException($"Invalid column in UCI move: '{move}' (must be a-z or aa-af)");
            x = column - 'a';
            rowPart = move.Substring(1);
        }

        if (!int.TryParse(rowPart, out int row))
            throw new ArgumentException($"Invalid row in UCI move: '{move}' (must be 1-32)");

        int y = row - 1;

        if (!IsValidCoordinate(x, y))
            throw new ArgumentException($"UCI move out of bounds: '{move}' (board is 32x32, a1-af32)");

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
