using System.Runtime.CompilerServices;
using Caro.Core.Domain.Entities;

namespace Caro.Core.GameLogic;

/// <summary>
/// Extension methods that add AI technical concerns (BitBoard, hashing) to the pure domain Board.
/// </summary>
public static class BoardExtensions
{
    private static readonly ConditionalWeakTable<Board, BoardTechnicalState> _stateCache = new();

    /// <summary>
    /// Get total cells on the board.
    /// </summary>
    public static int TotalCells(this Board board) => board.BoardSize * board.BoardSize;

    /// <summary>
    /// Get total stones placed on the board.
    /// </summary>
    public static int TotalStones(this Board board) => board.Cells.Count(c => !c.IsEmpty);

    /// <summary>
    /// Get occupied cells as enumerable of (x, y) tuples.
    /// </summary>
    public static IEnumerable<(int x, int y)> GetOccupiedCells(this Board board)
    {
        foreach (var cell in board.Cells)
        {
            if (!cell.IsEmpty)
                yield return (cell.X, cell.Y);
        }
    }

    /// <summary>
    /// Get occupied cells for a specific player as enumerable of (x, y) tuples.
    /// </summary>
    public static IEnumerable<(int x, int y)> GetOccupiedCells(this Board board, Player player)
    {
        foreach (var cell in board.Cells)
        {
            if (cell.Player == player)
                yield return (cell.X, cell.Y);
        }
    }

    /// <summary>
    /// Get the BitBoard representation for Red stones.
    /// </summary>
    [MethodImpl(MethodImplOptions.AggressiveInlining)]
    public static BitBoard GetRedBitBoard(this Board board)
    {
        var state = GetOrCreateState(board);
        state.EnsureInitialized(board);
        return state.RedBitBoard;
    }

    /// <summary>
    /// Get the BitBoard representation for Blue stones.
    /// </summary>
    [MethodImpl(MethodImplOptions.AggressiveInlining)]
    public static BitBoard GetBlueBitBoard(this Board board)
    {
        var state = GetOrCreateState(board);
        state.EnsureInitialized(board);
        return state.BlueBitBoard;
    }

    /// <summary>
    /// Get the BitBoard for a specific player.
    /// </summary>
    [MethodImpl(MethodImplOptions.AggressiveInlining)]
    public static BitBoard GetBitBoard(this Board board, Player player) =>
        player == Player.Red ? board.GetRedBitBoard() : board.GetBlueBitBoard();

    /// <summary>
    /// Get the Zobrist hash of the board position.
    /// </summary>
    [MethodImpl(MethodImplOptions.AggressiveInlining)]
    public static ulong GetHash(this Board board)
    {
        var state = GetOrCreateState(board);
        state.EnsureInitialized(board);
        return state.Hash;
    }

    [MethodImpl(MethodImplOptions.AggressiveInlining)]
    private static BoardTechnicalState GetOrCreateState(Board board)
    {
        return _stateCache.GetOrCreateValue(board);
    }

    /// <summary>
    /// Sync technical state from a source board after cloning.
    /// </summary>
    public static void SyncFromClone(this Board clone, Board source)
    {
        var sourceState = GetOrCreateState(source);
        var cloneState = GetOrCreateState(clone);
        cloneState.CopyFrom(sourceState);
    }

    /// <summary>
    /// Clone a board including its technical state (bitboards, hash).
    /// </summary>
    public static Board CloneWithState(this Board board)
    {
        var clone = board.Clone();
        clone.SyncFromClone(board);
        return clone;
    }

    /// <summary>
    /// Cached technical state for a Board instance (BitBoards and hash).
    /// </summary>
    private sealed class BoardTechnicalState
    {
        public BitBoard RedBitBoard = new();
        public BitBoard BlueBitBoard = new();
        public ulong Hash { get; private set; }
        private bool _initialized;

        public void EnsureInitialized(Board board)
        {
            if (_initialized) return;

            // Build initial state from board
            foreach (var cell in board.Cells)
            {
                if (cell.Player == Player.Red)
                    RedBitBoard.SetBit(cell.X, cell.Y);
                else if (cell.Player == Player.Blue)
                    BlueBitBoard.SetBit(cell.X, cell.Y);

                if (cell.Player != Player.None)
                    Hash ^= ZobristTables.GetKey(cell.X, cell.Y, cell.Player);
            }
            _initialized = true;
        }

        public void CopyFrom(BoardTechnicalState source)
        {
            RedBitBoard.CopyFrom(source.RedBitBoard);
            BlueBitBoard.CopyFrom(source.BlueBitBoard);
            Hash = source.Hash;
            _initialized = source._initialized;
        }
    }
}
