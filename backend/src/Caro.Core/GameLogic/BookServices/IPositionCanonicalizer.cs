using Caro.Core.Domain.Entities;

namespace Caro.Core.GameLogic;

/// <summary>
/// Interface for position canonicalization with symmetry reduction.
/// Converts board positions to a canonical form to handle symmetries.
/// </summary>
public interface IPositionCanonicalizer
{
    /// <summary>
    /// Transform board position to canonical form using symmetry reduction.
    /// Edge positions (within 5 cells of border) use absolute coordinates.
    /// Center positions use 8-way symmetry reduction.
    /// </summary>
    /// <returns>Canonical position with hash and transformation applied</returns>
    CanonicalPosition Canonicalize(Board board);

    /// <summary>
    /// Transform relative coordinates from canonical position back to actual board coordinates.
    /// Applies the inverse of the symmetry transformation used during canonicalization.
    /// </summary>
    /// <param name="relative">Relative coordinates from canonical position</param>
    /// <param name="symmetry">Symmetry transformation to reverse</param>
    /// <param name="board">Original board for reference</param>
    /// <returns>Actual board coordinates (x, y)</returns>
    (int x, int y) TransformToActual((int relX, int relY) relative, SymmetryType symmetry, Board board);

    /// <summary>
    /// Apply a symmetry transformation to a coordinate.
    /// </summary>
    /// <param name="x">Original X coordinate (0-18)</param>
    /// <param name="y">Original Y coordinate (0-18)</param>
    /// <param name="symmetry">Symmetry to apply</param>
    /// <returns>Transformed coordinates</returns>
    (int x, int y) ApplySymmetry(int x, int y, SymmetryType symmetry);

    /// <summary>
    /// Apply the inverse of a symmetry transformation to a coordinate.
    /// </summary>
    /// <param name="x">Transformed X coordinate</param>
    /// <param name="y">Transformed Y coordinate</param>
    /// <param name="symmetry">Symmetry to reverse</param>
    /// <returns>Original coordinates before transformation</returns>
    (int x, int y) ApplyInverseSymmetry(int x, int y, SymmetryType symmetry);

    /// <summary>
    /// Check if a position is near the edge of the board.
    /// Near-edge positions (within 5 cells of border) use absolute coordinates.
    /// </summary>
    bool IsNearEdge(BitBoard redBitBoard, BitBoard blueBitBoard);

    /// <summary>
    /// Compute canonical hash for a position using symmetry reduction.
    /// Returns the minimum hash across all 8 symmetries for center positions.
    /// </summary>
    ulong ComputeCanonicalHash(BitBoard redBitBoard, BitBoard blueBitBoard, Player player);
}
