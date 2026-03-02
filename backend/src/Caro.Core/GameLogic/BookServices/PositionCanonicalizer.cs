using Caro.Core.Domain.Configuration;
using Caro.Core.Domain.Entities;

namespace Caro.Core.GameLogic;

/// <summary>
/// Canonicalizes board positions using 8-way symmetry reduction.
/// Edge positions (within 5 cells of border) use absolute coordinates.
/// Center positions use relative coordinates with symmetry reduction.
/// </summary>
public sealed class PositionCanonicalizer : IPositionCanonicalizer
{
    private const int BoardSize = GameConstants.BoardSize;
    private const int EdgeThreshold = 5;
    private const int Center = GameConstants.CenterPosition;

    /// <inheritdoc/>
    public CanonicalPosition Canonicalize(Board board)
    {
        var redBitBoard = board.GetBitBoard(Player.Red);
        var blueBitBoard = board.GetBitBoard(Player.Blue);

        // Determine current player by counting stones
        int redCount = redBitBoard.CountBits();
        int blueCount = blueBitBoard.CountBits();
        Player currentPlayer = redCount == blueCount ? Player.Red : Player.Blue;

        // Check if position is near edge
        bool isNearEdge = IsNearEdge(redBitBoard, blueBitBoard);

        ulong canonicalHash;
        SymmetryType symmetryApplied;

        if (isNearEdge)
        {
            // Edge positions: use absolute coordinates, no symmetry
            canonicalHash = board.GetHash();
            symmetryApplied = SymmetryType.Identity;
        }
        else
        {
            // Center positions: apply symmetry reduction
            (canonicalHash, symmetryApplied) = FindCanonicalSymmetry(redBitBoard, blueBitBoard, currentPlayer);
        }

        return new CanonicalPosition(
            CanonicalHash: canonicalHash,
            SymmetryApplied: symmetryApplied,
            IsNearEdge: isNearEdge,
            Player: currentPlayer
        );
    }

    /// <inheritdoc/>
    public (int x, int y) TransformToActual((int relX, int relY) relative, SymmetryType symmetry, Board board)
    {
        // For edge positions or identity symmetry, no transformation needed
        if (symmetry == SymmetryType.Identity)
            return relative;

        // Apply inverse symmetry transformation
        return ApplyInverseSymmetry(relative.relX, relative.relY, symmetry);
    }

    /// <inheritdoc/>
    public (int x, int y) ApplySymmetry(int x, int y, SymmetryType symmetry)
    {
        return symmetry switch
        {
            SymmetryType.Identity => (x, y),
            SymmetryType.Rotate90 => (y, BoardSize - 1 - x),
            SymmetryType.Rotate180 => (BoardSize - 1 - x, BoardSize - 1 - y),
            SymmetryType.Rotate270 => (BoardSize - 1 - y, x),
            SymmetryType.FlipHorizontal => (BoardSize - 1 - x, y),
            SymmetryType.FlipVertical => (x, BoardSize - 1 - y),
            SymmetryType.DiagonalA => (y, x),
            SymmetryType.DiagonalB => (BoardSize - 1 - y, BoardSize - 1 - x),
            _ => (x, y)
        };
    }

    /// <inheritdoc/>
    public (int x, int y) ApplyInverseSymmetry(int x, int y, SymmetryType symmetry)
    {
        // The inverse of each symmetry transformation
        // Most transformations are their own inverse, except rotations
        return symmetry switch
        {
            SymmetryType.Identity => (x, y),
            SymmetryType.Rotate90 => (BoardSize - 1 - y, x),        // Inverse of Rotate90 is Rotate270
            SymmetryType.Rotate180 => (BoardSize - 1 - x, BoardSize - 1 - y), // Self-inverse
            SymmetryType.Rotate270 => (y, BoardSize - 1 - x),       // Inverse of Rotate270 is Rotate90
            SymmetryType.FlipHorizontal => (BoardSize - 1 - x, y),  // Self-inverse
            SymmetryType.FlipVertical => (x, BoardSize - 1 - y),    // Self-inverse
            SymmetryType.DiagonalA => (y, x),                      // Self-inverse
            SymmetryType.DiagonalB => (BoardSize - 1 - y, BoardSize - 1 - x), // Self-inverse
            _ => (x, y)
        };
    }

    /// <inheritdoc/>
    public bool IsNearEdge(BitBoard redBitBoard, BitBoard blueBitBoard)
    {
        // Check if any stone is within EdgeThreshold of border
        // A position is "near edge" if any stone satisfies:
        // x < EdgeThreshold OR x >= BoardSize - EdgeThreshold
        // y < EdgeThreshold OR y >= BoardSize - EdgeThreshold

        // Get all set positions from both boards
        var redPositions = redBitBoard.GetSetPositions();
        var bluePositions = blueBitBoard.GetSetPositions();

        var allPositions = redPositions.Concat(bluePositions);

        foreach (var (x, y) in allPositions)
        {
            if (x < EdgeThreshold || x >= BoardSize - EdgeThreshold ||
                y < EdgeThreshold || y >= BoardSize - EdgeThreshold)
            {
                return true;
            }
        }

        return false;
    }

    /// <inheritdoc/>
    public ulong ComputeCanonicalHash(BitBoard redBitBoard, BitBoard blueBitBoard, Player player)
    {
        bool isNearEdge = IsNearEdge(redBitBoard, blueBitBoard);

        if (isNearEdge)
        {
            // For edge positions, compute a simple combined hash
            // This is a simplified version - full implementation would use Zobrist
            return ComputeSimpleHash(redBitBoard, blueBitBoard, player);
        }

        // For center positions, try all 8 symmetries and return minimum hash
        ulong minHash = ulong.MaxValue;
        var symmetries = Enum.GetValues<SymmetryType>();

        foreach (var symmetry in symmetries)
        {
            var (transformedRed, transformedBlue) = ApplyBitBoardSymmetry(redBitBoard, blueBitBoard, symmetry);
            ulong hash = ComputeSimpleHash(transformedRed, transformedBlue, player);
            if (hash < minHash)
            {
                minHash = hash;
            }
        }

        return minHash;
    }

    /// <summary>
    /// Find the canonical symmetry by trying all 8 transformations and returning the minimum hash.
    /// </summary>
    private static (ulong Hash, SymmetryType Symmetry) FindCanonicalSymmetry(
        BitBoard redBitBoard,
        BitBoard blueBitBoard,
        Player player)
    {
        ulong minHash = ulong.MaxValue;
        SymmetryType canonicalSymmetry = SymmetryType.Identity;

        var symmetries = Enum.GetValues<SymmetryType>();

        foreach (var symmetry in symmetries)
        {
            var (transformedRed, transformedBlue) = ApplyBitBoardSymmetry(redBitBoard, blueBitBoard, symmetry);
            ulong hash = ComputeSimpleHash(transformedRed, transformedBlue, player);

            if (hash < minHash)
            {
                minHash = hash;
                canonicalSymmetry = symmetry;
            }
        }

        return (minHash, canonicalSymmetry);
    }

    /// <summary>
    /// Apply a symmetry transformation to BitBoards.
    /// </summary>
    private static (BitBoard Red, BitBoard Blue) ApplyBitBoardSymmetry(
        BitBoard redBitBoard,
        BitBoard blueBitBoard,
        SymmetryType symmetry)
    {
        if (symmetry == SymmetryType.Identity)
        {
            return (redBitBoard, blueBitBoard);
        }

        var newRed = new BitBoard();
        var newBlue = new BitBoard();

        // Transform each set bit
        for (int y = 0; y < BoardSize; y++)
        {
            for (int x = 0; x < BoardSize; x++)
            {
                var (tx, ty) = symmetry switch
                {
                    SymmetryType.Identity => (x, y),
                    SymmetryType.Rotate90 => (y, BoardSize - 1 - x),
                    SymmetryType.Rotate180 => (BoardSize - 1 - x, BoardSize - 1 - y),
                    SymmetryType.Rotate270 => (BoardSize - 1 - y, x),
                    SymmetryType.FlipHorizontal => (BoardSize - 1 - x, y),
                    SymmetryType.FlipVertical => (x, BoardSize - 1 - y),
                    SymmetryType.DiagonalA => (y, x),
                    SymmetryType.DiagonalB => (BoardSize - 1 - y, BoardSize - 1 - x),
                    _ => (x, y)
                };

                if (redBitBoard.GetBit(x, y))
                    newRed.SetBit(tx, ty);
                if (blueBitBoard.GetBit(x, y))
                    newBlue.SetBit(tx, ty);
            }
        }

        return (newRed, newBlue);
    }

    /// <summary>
    /// Compute a simple hash from BitBoards and player.
    /// This is a placeholder - full implementation would use Zobrist tables properly.
    /// </summary>
    private static ulong ComputeSimpleHash(BitBoard redBitBoard, BitBoard blueBitBoard, Player player)
    {
        var (rb0, rb1, rb2, rb3) = redBitBoard.GetRawValues();
        var (bb0, bb1, bb2, bb3) = blueBitBoard.GetRawValues();

        // Combine all values with XOR and mixing
        ulong hash = rb0 ^ (rb1 << 1) ^ (rb2 << 2) ^ (rb3 << 3);
        hash ^= bb0 ^ (bb1 << 1) ^ (bb2 << 2) ^ (bb3 << 3);
        hash ^= (ulong)player * 0x9e3779b97f4a7c15UL; // Golden ratio prime

        // Final avalanche mix
        hash ^= hash >> 33;
        hash *= 0xff51afd7ed558ccdUL;
        hash ^= hash >> 33;
        hash *= 0xc4ceb9fe1a85ec53UL;
        hash ^= hash >> 33;

        return hash;
    }
}
