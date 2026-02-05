namespace Caro.Core.Domain.Entities;

/// <summary>
/// Interface for the game board, allowing Domain layer to reference Board
/// without depending on Core implementation details (BitBoard, ZobristTables).
/// </summary>
public interface IBoard
{
    /// <summary>Size of the board</summary>
    int BoardSize { get; }

    /// <summary>Current Zobrist hash of the position</summary>
    ulong Hash { get; }

    /// <summary>Place a stone at the given position</summary>
    void PlaceStone(int x, int y, Player player);

    /// <summary>Get the cell at the given position</summary>
    ICell GetCell(int x, int y);

    /// <summary>Create a deep copy of the board</summary>
    IBoard Clone();
}

/// <summary>
/// Interface for a cell on the board
/// </summary>
public interface ICell
{
    /// <summary>Player occupying this cell</summary>
    Player Player { get; }

    /// <summary>Whether the cell is empty</summary>
    bool IsEmpty { get; }
}
