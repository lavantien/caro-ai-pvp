using Caro.Core.Domain.Entities;
using Caro.Core.GameLogic;

namespace Caro.Core.Tests.Helpers;

/// <summary>
/// Simple implementation of IPositionCanonicalizer for testing.
/// Always returns Identity symmetry (no transformation) for predictable test behavior.
/// </summary>
public sealed class MockPositionCanonicalizer : IPositionCanonicalizer
{
    /// <summary>
    /// Create a canonical position with Identity symmetry.
    /// </summary>
    public CanonicalPosition Canonicalize(Board board)
    {
        // For simplicity, always use Identity symmetry
        // This means canonical coordinates = actual coordinates
        var redBitBoard = board.GetBitBoard(Player.Red);
        var blueBitBoard = board.GetBitBoard(Player.Blue);

        ulong hash = ComputeCanonicalHash(redBitBoard, blueBitBoard, Player.Red);

        // Determine current player
        int redCount = redBitBoard.CountBits();
        int blueCount = blueBitBoard.CountBits();
        Player currentPlayer = redCount == blueCount ? Player.Red : Player.Blue;

        return new CanonicalPosition(
            CanonicalHash: hash,
            SymmetryApplied: SymmetryType.Identity,
            IsNearEdge: false,  // Always false for simplicity
            Player: currentPlayer
        );
    }

    /// <summary>
    /// Transform relative coordinates back to actual (returns unchanged for Identity).
    /// </summary>
    public (int x, int y) TransformToActual((int relX, int relY) relative, SymmetryType symmetry, Board board)
    {
        // For Identity symmetry, relative = actual
        return (relative.relX, relative.relY);
    }

    /// <summary>
    /// Apply symmetry transformation (returns unchanged for Identity).
    /// </summary>
    public (int x, int y) ApplySymmetry(int x, int y, SymmetryType symmetry)
    {
        // For Identity symmetry, coordinates stay the same
        return (x, y);
    }

    /// <summary>
    /// Apply inverse symmetry transformation (returns unchanged for Identity).
    /// </summary>
    public (int x, int y) ApplyInverseSymmetry(int x, int y, SymmetryType symmetry)
    {
        // For Identity symmetry, coordinates stay the same
        return (x, y);
    }

    /// <summary>
    /// Check if position is near edge (always false for simplicity).
    /// </summary>
    public bool IsNearEdge(BitBoard redBitBoard, BitBoard blueBitBoard)
    {
        // Always return false for simplicity - tests can override if needed
        return false;
    }

    /// <summary>
    /// Compute canonical hash (simple XOR-based hash for testing).
    /// </summary>
    public ulong ComputeCanonicalHash(BitBoard redBitBoard, BitBoard blueBitBoard, Player player)
    {
        // Simple hash for testing: combine red and blue board hashes
        var (r0, r1, r2, r3, r4, r5) = redBitBoard.GetRawValues();
        var (b0, b1, b2, b3, b4, b5) = blueBitBoard.GetRawValues();

        // Combine all values with XOR
        ulong hash = r0 ^ r1 ^ r2 ^ r3 ^ r4 ^ r5;
        hash ^= b0 ^ b1 ^ b2 ^ b3 ^ b4 ^ b5;
        hash ^= (ulong)player;

        return hash;
    }
}
