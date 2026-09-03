using Caro.Domain;

namespace Caro.Engine.Tests;

/// <summary>
/// Rebuilds positions from the round-robin tournament archives: the seeded
/// two-stone opening (red, then blue) followed by the recorded moves in
/// order, alternating starting with red. Move lists come straight from the
/// matches.db rows (see docs/artifacts/tournaments/ANOMALIES.md).
/// </summary>
internal static class TournamentReplay
{
    /// <param name="seed">Game seed; regenerates the opening placements.</param>
    /// <param name="moves">Recorded moves "x,y;x,y;..." starting at move 2 (red).</param>
    /// <param name="count">How many recorded moves to apply.</param>
    /// <param name="nextMover">The player to move after the applied prefix.</param>
    public static Board BoardAt(long seed, string moves, int count, out Player nextMover)
    {
        ((int rx, int ry), (int bx, int by)) = Opening.SeededPlacements(seed, Constants.Opening.SpreadRadius);
        Board b = Board.NewBoard().PlaceStone(rx, ry, Player.Red).PlaceStone(bx, by, Player.Blue);

        string[] cells = moves.Split(';', StringSplitOptions.RemoveEmptyEntries);
        if (count > cells.Length)
        {
            throw new ArgumentException($"requested {count} moves but only {cells.Length} recorded");
        }
        for (int i = 0; i < count; i++)
        {
            string[] xy = cells[i].Split(',');
            int x = int.Parse(xy[0], System.Globalization.CultureInfo.InvariantCulture);
            int y = int.Parse(xy[1], System.Globalization.CultureInfo.InvariantCulture);
            if (b.GetPlayerAt(x, y) != Player.None)
            {
                throw new ArgumentException($"recorded move {i + 1} ({x},{y}) plays an occupied cell");
            }
            b = b.PlaceStone(x, y, i % 2 == 0 ? Player.Red : Player.Blue);
        }
        nextMover = count % 2 == 0 ? Player.Red : Player.Blue;
        return b;
    }
}
