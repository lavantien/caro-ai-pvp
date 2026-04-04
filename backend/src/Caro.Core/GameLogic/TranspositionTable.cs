using System.Runtime.CompilerServices;
using System.Runtime.InteropServices;
using Caro.Core.Domain.Entities;

namespace Caro.Core.GameLogic;

/// <summary>
/// Transposition table with Zobrist hashing for caching search results
/// Multi-entry cluster design for improved hit rates (40% -> 60% target)
/// Provides 2-5x speedup by avoiding re-computation of identical positions
/// </summary>
public partial class TranspositionTable
{
    /// <summary>
    /// Entry bounds for transposition table scores
    /// </summary>
    public enum EntryFlag : byte
    {
        Exact = 0,       // Score is exact (alpha < score < beta)
        LowerBound = 1,  // Score is at least this value (beta cutoff)
        UpperBound = 2   // Score is at most this value (alpha cutoff)
    }

    /// <summary>
    /// Packed transposition table entry (10 bytes)
    /// Uses packed fields for cache efficiency
    /// IMPORTANT: sizeof(TTEntry) must be exactly 10 bytes
    /// </summary>
    [StructLayout(LayoutKind.Explicit, Size = 10)]
    public struct TTEntry
    {
        // Hash key (16 bits - stored separately from full 64-bit hash for verification)
        [FieldOffset(0)] public ushort Key16;

        // Score value (16 bits)
        [FieldOffset(2)] public short Value;

        // Search depth (8 bits, signed)
        [FieldOffset(4)] public sbyte Depth8;

        // Bound (2 bits) + Age (6 bits) packed into single byte
        [FieldOffset(5)] public byte BoundAndAge;

        // Best move packed into 16 bits (8 bits x + 8 bits y)
        [FieldOffset(6)] public ushort Move16;

        // Static evaluation (16 bits)
        [FieldOffset(8)] public short Eval16;

        /// <summary>
        /// Extract bound from BoundAndAge byte
        /// </summary>
        [MethodImpl(MethodImplOptions.AggressiveInlining)]
        public readonly EntryFlag GetBound() => (EntryFlag)(BoundAndAge & 0x03);

        /// <summary>
        /// Extract age from BoundAndAge byte
        /// </summary>
        [MethodImpl(MethodImplOptions.AggressiveInlining)]
        public readonly byte GetAge() => (byte)(BoundAndAge >> 2);

        /// <summary>
        /// Create BoundAndAge byte from bound and age
        /// </summary>
        [MethodImpl(MethodImplOptions.AggressiveInlining)]
        public static byte MakeBoundAndAge(EntryFlag bound, byte age)
            => (byte)((byte)bound | (age << 2));

        /// <summary>
        /// Get the best move as a tuple
        /// </summary>
        [MethodImpl(MethodImplOptions.AggressiveInlining)]
        public readonly (int x, int y)? GetMove()
        {
            int x = Move16 & 0xFF;
            int y = Move16 >> 8;
            // Use 0xFF as sentinel for "no move"
            if (x == 0xFF || y == 0xFF)
                return null;
            return (x, y);
        }

        /// <summary>
        /// Pack move coordinates into 16 bits
        /// </summary>
        [MethodImpl(MethodImplOptions.AggressiveInlining)]
        public static ushort PackMove(int x, int y)
        {
            if (x < 0 || y < 0)
                return 0xFFFF; // Sentinel for no move
            return (ushort)((y << 8) | (x & 0xFF));
        }

        /// <summary>
        /// Calculate replacement value: depth - 8 * age
        /// Higher values preferred for keeping
        /// </summary>
        [MethodImpl(MethodImplOptions.AggressiveInlining)]
        public readonly int ReplacementValue() => Depth8 - 8 * GetAge();
    }

    /// <summary>
    /// Cluster of 3 TT entries (32 bytes for cache-line alignment)
    /// Uses byte array with manual offset calculation for TTEntry access
    /// </summary>
    [StructLayout(LayoutKind.Explicit, Size = 32)]
    public unsafe struct Cluster
    {
        // Fixed array of 30 bytes (3 * 10 bytes per TTEntry)
        [FieldOffset(0)] private fixed byte _bytes[30];

        private const int EntrySize = 10; // sizeof(TTEntry)

        /// <summary>
        /// Get entry at index (0-2)
        /// </summary>
        [MethodImpl(MethodImplOptions.AggressiveInlining)]
        public TTEntry GetEntry(int index)
        {
            if (index < 0 || index >= 3)
                throw new ArgumentOutOfRangeException(nameof(index));

            fixed (byte* p = _bytes)
            {
                TTEntry* entryPtr = (TTEntry*)(p + index * EntrySize);
                return *entryPtr;
            }
        }

        /// <summary>
        /// Set entry at index (0-2)
        /// </summary>
        [MethodImpl(MethodImplOptions.AggressiveInlining)]
        public void SetEntry(int index, in TTEntry entry)
        {
            if (index < 0 || index >= 3)
                throw new ArgumentOutOfRangeException(nameof(index));

            fixed (byte* p = _bytes)
            {
                TTEntry* entryPtr = (TTEntry*)(p + index * EntrySize);
                *entryPtr = entry;
            }
        }

        /// <summary>
        /// Find index of entry with lowest replacement value
        /// </summary>
        [MethodImpl(MethodImplOptions.AggressiveInlining)]
        public int FindLowestValueIndex()
        {
            int lowestIndex = 0;
            int lowestValue = int.MaxValue;

            for (int i = 0; i < 3; i++)
            {
                TTEntry entry = GetEntry(i);
                int value = entry.ReplacementValue();
                if (value < lowestValue)
                {
                    lowestValue = value;
                    lowestIndex = i;
                }
            }

            return lowestIndex;
        }

        /// <summary>
        /// Get the internal entries array pointer (for direct access)
        /// </summary>
        [MethodImpl(MethodImplOptions.AggressiveInlining)]
        public TTEntry* GetEntriesPtr()
        {
            fixed (byte* p = _bytes)
            {
                return (TTEntry*)p;
            }
        }
    }

    private readonly Cluster[] _table;
    private readonly int _size;
    private byte _currentAge;

    /// <summary>
    /// Create a transposition table with specified size in MB
    /// Default 256MB provides ~8M clusters (~24M entries with 3-way)
    /// Uses shared ZobristTables for hash key consistency
    /// </summary>
    public TranspositionTable(int sizeMB = 256)
    {
        // Each cluster is 32 bytes
        _size = (sizeMB * 1024 * 1024) / 32;
        _table = new Cluster[_size];
        _currentAge = 1;
    }

    /// <summary>
    /// Calculate Zobrist hash for the current board position
    /// Uses Board.GetHash() for O(1) access (incremental hash maintained by Board)
    /// </summary>
    public ulong CalculateHash(Board board)
    {
        return board.GetHash();
    }

    /// Increment age counter for replacement strategy
    public void IncrementAge()
    {
        _currentAge++;
        if (_currentAge == 64) // Age uses 6 bits (0-63)
        {
            _currentAge = 1;
        }
    }

    /// Clear the entire table
    public void Clear()
    {
        Array.Clear(_table, 0, _table.Length);
        _currentAge = 1;
    }
}
