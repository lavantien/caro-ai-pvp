using Caro.Core.Domain.Entities;
using Caro.Core.GameLogic;

namespace Caro.Core.Tests.Helpers;

/// <summary>
/// Fluent builder for creating OpeningBookEntry test data.
/// Provides a clean, readable API for constructing test entries.
/// </summary>
public sealed class OpeningBookEntryBuilder
{
    private ulong _hash;
    private int _depth;
    private Player _player = Player.Red;
    private SymmetryType _symmetry = SymmetryType.Identity;
    private bool _isNearEdge = false;
    private readonly List<BookMove> _moves = new();

    /// <summary>
    /// Create a new builder with default values.
    /// </summary>
    public OpeningBookEntryBuilder()
    {
        _hash = 1; // Default hash
        _depth = 0;
    }

    /// <summary>
    /// Set the canonical hash for the entry.
    /// </summary>
    public OpeningBookEntryBuilder WithHash(ulong hash)
    {
        _hash = hash;
        return this;
    }

    /// <summary>
    /// Set the ply depth for the entry.
    /// </summary>
    public OpeningBookEntryBuilder WithDepth(int depth)
    {
        _depth = depth;
        return this;
    }

    /// <summary>
    /// Set the player whose turn it is.
    /// </summary>
    public OpeningBookEntryBuilder WithPlayer(Player player)
    {
        _player = player;
        return this;
    }

    /// <summary>
    /// Set the symmetry type applied.
    /// </summary>
    public OpeningBookEntryBuilder WithSymmetry(SymmetryType symmetry)
    {
        _symmetry = symmetry;
        return this;
    }

    /// <summary>
    /// Set whether this is a near-edge position.
    /// </summary>
    public OpeningBookEntryBuilder WithNearEdge(bool isNearEdge)
    {
        _isNearEdge = isNearEdge;
        return this;
    }

    /// <summary>
    /// Add a book move to this entry.
    /// </summary>
    public OpeningBookEntryBuilder WithMove(BookMove move)
    {
        _moves.Add(move);
        return this;
    }

    /// <summary>
    /// Add multiple book moves to this entry.
    /// </summary>
    public OpeningBookEntryBuilder WithMoves(IEnumerable<BookMove> moves)
    {
        _moves.AddRange(moves);
        return this;
    }

    /// <summary>
    /// Add multiple book moves to this entry.
    /// </summary>
    public OpeningBookEntryBuilder WithMoves(params BookMove[] moves)
    {
        _moves.AddRange(moves);
        return this;
    }

    /// <summary>
    /// Add a simple move with just coordinates.
    /// Other properties are set to sensible defaults.
    /// </summary>
    public OpeningBookEntryBuilder WithSimpleMove(int x, int y)
    {
        _moves.Add(new BookMove
        {
            RelativeX = x,
            RelativeY = y,
            WinRate = 50,
            DepthAchieved = 10,
            NodesSearched = 1000,
            Score = 0,
            IsForcing = false,
            Priority = _moves.Count + 1,
            IsVerified = true
        });
        return this;
    }

    /// <summary>
    /// Clear all moves from this entry.
    /// </summary>
    public OpeningBookEntryBuilder ClearMoves()
    {
        _moves.Clear();
        return this;
    }

    /// <summary>
    /// Build the OpeningBookEntry with the configured values.
    /// </summary>
    public OpeningBookEntry Build()
    {
        return new OpeningBookEntry
        {
            CanonicalHash = _hash,
            Depth = _depth,
            Player = _player,
            Symmetry = _symmetry,
            IsNearEdge = _isNearEdge,
            Moves = _moves.ToArray()
        };
    }

    /// <summary>
    /// Create a builder pre-configured with empty board defaults.
    /// </summary>
    public static OpeningBookEntryBuilder ForEmptyBoard()
    {
        return new OpeningBookEntryBuilder()
            .WithHash(0)
            .WithDepth(0)
            .WithPlayer(Player.Red)
            .WithSymmetry(SymmetryType.Identity);
    }

    /// <summary>
    /// Create a builder pre-configured for a center position.
    /// </summary>
    public static OpeningBookEntryBuilder ForCenterPosition(int depth = 1)
    {
        return new OpeningBookEntryBuilder()
            .WithHash(42) // Arbitrary hash for center
            .WithDepth(depth)
            .WithPlayer(depth % 2 == 0 ? Player.Red : Player.Blue)
            .WithSymmetry(SymmetryType.Identity);
    }
}
