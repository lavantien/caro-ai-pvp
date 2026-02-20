using System.Runtime.CompilerServices;
using Caro.Core.Domain.Configuration;
using Caro.Core.Domain.Entities;

namespace Caro.Core.GameLogic;

/// <summary>
/// Mutable, high-performance board representation for search algorithms.
/// Uses make/unmake pattern instead of copy-make for zero-allocation move execution.
///
/// Performance characteristics:
/// - MakeMove: O(1), no allocations
/// - UnmakeMove: O(1), no allocations
/// - GetBitBoard: O(1), returns copy of internal bitboard
/// - Hash: Incrementally maintained, O(1) access
///
/// Memory layout:
/// - 2 BitBoards (8 ulongs total) for piece positions
/// - 1 ulong for hash
/// - Total: ~72 bytes on stack
/// </summary>
public sealed class SearchBoard
{
    private const int Size = GameConstants.BoardSize;

    // BitBoard representation - 4 ulongs per player (256 bits for 16x16)
    private BitBoard _redBits;
    private BitBoard _blueBits;

    // Incrementally maintained hash (same formula as immutable Board for compatibility)
    private ulong _hash;

    // Hash constants for piece keys (must match Board.PlaceStone)
    private const ulong RedHashMask = 0xAAAAAAAAAAAAAAAAUL;
    private const ulong BlueHashMask = 0x5555555555555555UL;

    /// <summary>
    /// Create an empty SearchBoard.
    /// </summary>
    public SearchBoard()
    {
        _redBits = new BitBoard();
        _blueBits = new BitBoard();
        _hash = 0;
    }

    /// <summary>
    /// Create a SearchBoard from an existing immutable Board.
    /// Copies the position data into this mutable representation.
    /// </summary>
    public SearchBoard(Board board)
    {
        _redBits = board.GetBitBoard(Player.Red);
        _blueBits = board.GetBitBoard(Player.Blue);
        _hash = board.GetHash();
    }

    /// <summary>
    /// Board size (always 16 for Caro).
    /// </summary>
    public int BoardSize => Size;

    /// <summary>
    /// Get the current Zobrist hash for transposition table lookup.
    /// </summary>
    [MethodImpl(MethodImplOptions.AggressiveInlining)]
    public ulong GetHash() => _hash;

    /// <summary>
    /// Get the BitBoard representation for a player.
    /// Returns a copy to prevent external mutation.
    /// </summary>
    [MethodImpl(MethodImplOptions.AggressiveInlining)]
    public BitBoard GetBitBoard(Player player) =>
        player == Player.Red ? _redBits : _blueBits;

    /// <summary>
    /// Get the combined occupancy BitBoard (all stones).
    /// </summary>
    [MethodImpl(MethodImplOptions.AggressiveInlining)]
    public BitBoard GetOccupancy() => _redBits | _blueBits;

    /// <summary>
    /// Check if a position is empty.
    /// </summary>
    [MethodImpl(MethodImplOptions.AggressiveInlining)]
    public bool IsEmpty(int x, int y)
    {
        if (x < 0 || x >= Size || y < 0 || y >= Size)
            return false;
        return !_redBits.GetBit(x, y) && !_blueBits.GetBit(x, y);
    }

    /// <summary>
    /// Get the player occupying a position.
    /// </summary>
    [MethodImpl(MethodImplOptions.AggressiveInlining)]
    public Player GetPlayerAt(int x, int y)
    {
        if (x < 0 || x >= Size || y < 0 || y >= Size)
            return Player.None;

        if (_redBits.GetBit(x, y))
            return Player.Red;
        if (_blueBits.GetBit(x, y))
            return Player.Blue;
        return Player.None;
    }

    /// <summary>
    /// Make a move on the board. Mutates internal state.
    /// Returns undo information for UnmakeMove.
    /// </summary>
    /// <param name="x">X coordinate (0-15)</param>
    /// <param name="y">Y coordinate (0-15)</param>
    /// <param name="player">Player making the move</param>
    /// <returns>Undo information to pass to UnmakeMove</returns>
    [MethodImpl(MethodImplOptions.AggressiveInlining)]
    public MoveUndo MakeMove(int x, int y, Player player)
    {
        // Store undo info (the captured stone is always None for valid moves)
        var undo = new MoveUndo(x, y, player);

        // Set bit in appropriate bitboard
        if (player == Player.Red)
            _redBits.SetBit(x, y);
        else
            _blueBits.SetBit(x, y);

        // Update hash incrementally (same formula as Board.PlaceStone)
        ulong pieceKey = (ulong)((x << 8) | y) ^ (player == Player.Red ? RedHashMask : BlueHashMask);
        _hash ^= pieceKey;

        return undo;
    }

    /// <summary>
    /// Unmake a move, restoring the board to its previous state.
    /// </summary>
    /// <param name="undo">Undo information from MakeMove</param>
    [MethodImpl(MethodImplOptions.AggressiveInlining)]
    public void UnmakeMove(MoveUndo undo)
    {
        // Clear bit in appropriate bitboard
        if (undo.Player == Player.Red)
            _redBits.ClearBit(undo.X, undo.Y);
        else
            _blueBits.ClearBit(undo.X, undo.Y);

        // Restore hash (XOR is its own inverse - same formula as make)
        ulong pieceKey = (ulong)((undo.X << 8) | undo.Y) ^ (undo.Player == Player.Red ? RedHashMask : BlueHashMask);
        _hash ^= pieceKey;
    }

    /// <summary>
    /// Copy the state from another SearchBoard.
    /// Useful for creating snapshots or parallel search.
    /// </summary>
    [MethodImpl(MethodImplOptions.AggressiveInlining)]
    public void CopyFrom(SearchBoard source)
    {
        _redBits.CopyFrom(source._redBits);
        _blueBits.CopyFrom(source._blueBits);
        _hash = source._hash;
    }

    /// <summary>
    /// Create a deep copy of this SearchBoard.
    /// </summary>
    public SearchBoard Clone()
    {
        var clone = new SearchBoard();
        clone._redBits.CopyFrom(_redBits);
        clone._blueBits.CopyFrom(_blueBits);
        clone._hash = _hash;
        return clone;
    }

    /// <summary>
    /// Convert to immutable Board representation.
    /// Used when returning results from search.
    /// </summary>
    public Board ToBoard()
    {
        var board = new Board();
        for (int x = 0; x < Size; x++)
        {
            for (int y = 0; y < Size; y++)
            {
                var player = GetPlayerAt(x, y);
                if (player != Player.None)
                {
                    board = board.PlaceStone(x, y, player);
                }
            }
        }
        return board;
    }

    /// <summary>
    /// Reset the board to empty state.
    /// </summary>
    public void Clear()
    {
        _redBits = new BitBoard();
        _blueBits = new BitBoard();
        _hash = 0;
    }

    /// <summary>
    /// Count total stones on the board.
    /// </summary>
    [MethodImpl(MethodImplOptions.AggressiveInlining)]
    public int TotalStones() => _redBits.CountBits() + _blueBits.CountBits();

    /// <summary>
    /// Check if the board is empty.
    /// </summary>
    public bool IsEmpty() => _redBits.IsEmpty && _blueBits.IsEmpty;

    /// <summary>
    /// Get all occupied cells as (x, y, player) tuples.
    /// </summary>
    public IEnumerable<(int x, int y, Player player)> GetOccupiedCells()
    {
        for (int x = 0; x < Size; x++)
        {
            for (int y = 0; y < Size; y++)
            {
                var player = GetPlayerAt(x, y);
                if (player != Player.None)
                {
                    yield return (x, y, player);
                }
            }
        }
    }
}

/// <summary>
/// Undo information for a move on SearchBoard.
/// Small struct (16 bytes) that can be stack-allocated.
/// </summary>
public readonly record struct MoveUndo(int X, int Y, Player Player);
