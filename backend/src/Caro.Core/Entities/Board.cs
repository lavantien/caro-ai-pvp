namespace Caro.Core.Entities;

public enum Player { None, Red, Blue }

public class Cell
{
    public Player Player { get; set; } = Player.None;
    public bool IsEmpty => Player == Player.None;
}

public class Board
{
    private const int Size = 15;
    private readonly Cell[,] _cells;

    public Board()
    {
        _cells = new Cell[Size, Size];
        for (int i = 0; i < Size; i++)
        {
            for (int j = 0; j < Size; j++)
            {
                _cells[i, j] = new Cell();
            }
        }
    }

    public int BoardSize => Size;

    public IEnumerable<Cell> Cells => _cells.Cast<Cell>();

    public void PlaceStone(int x, int y, Player player)
    {
        if (x < 0 || x >= Size || y < 0 || y >= Size)
            throw new ArgumentOutOfRangeException(nameof(x), "Position must be within board bounds");

        if (!_cells[x, y].IsEmpty)
            throw new InvalidOperationException("Cell is already occupied");

        _cells[x, y].Player = player;
    }

    public Cell GetCell(int x, int y)
    {
        if (x < 0 || x >= Size || y < 0 || y >= Size)
            throw new ArgumentOutOfRangeException(nameof(x), "Position must be within board bounds");

        return _cells[x, y];
    }
}
