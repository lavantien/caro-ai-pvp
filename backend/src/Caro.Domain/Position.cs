namespace Caro.Domain;

public readonly record struct Position(int X, int Y)
{
    public bool IsValid() =>
        X >= 0 && X < Constants.BoardSize && Y >= 0 && Y < Constants.BoardSize;

    public Position Offset(int dx, int dy) => new(X + dx, Y + dy);
}

public readonly record struct Cell(int X, int Y, Player Player)
{
    public bool IsEmpty() => Player == Player.None;
}
