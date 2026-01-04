using System.Runtime.Intrinsics;
using System.Runtime.Intrinsics.X86;
using System.Runtime.Intrinsics.Arm;

namespace Caro.Core.GameLogic;

/// <summary>
/// Platform-specific intrinsic detection and utilities
/// Provides SIMD capability detection for x64 (AVX2) and ARM64 (AdvSimd)
/// </summary>
public static class PlatformIntrinsics
{
    /// <summary>
    /// Whether AVX2 (Advanced Vector Extensions) is supported on x64
    /// </summary>
    public static bool SupportsAvx2 => Avx2.IsSupported;

    /// <summary>
    /// Whether SSE3 is supported on x64
    /// </summary>
    public static bool SupportsSse3 => Sse3.IsSupported;

    /// <summary>
    /// Whether ARM64 AdvSimd (Advanced SIMD) is supported
    /// </summary>
    public static bool SupportsAdvSimd => AdvSimd.IsSupported;

    /// <summary>
    /// Whether ARM64 NEON is supported (alias for AdvSimd)
    /// </summary>
    public static bool SupportsNeon => AdvSimd.IsSupported;

    /// <summary>
    /// Whether BMI2 (Bit Manipulation Instruction Set 2) is supported
    /// Useful for parallel bit deposit/extract operations
    /// </summary>
    public static bool SupportsBmi2 => Bmi2.X64.IsSupported;

    /// <summary>
    /// Whether POPCNT (Population Count) instruction is supported
    /// </summary>
    public static bool SupportsPopcnt => X86Base.IsSupported;

    /// <summary>
    /// Whether LZCNT (Leading Zero Count) instruction is supported
    /// </summary>
    public static bool SupportsLzcnt => Lzcnt.X64.IsSupported;

    /// <summary>
    /// Get the best available SIMD acceleration level
    /// </summary>
    public static SIMDLevel GetSIMDLevel()
    {
        if (SupportsAvx2)
            return SIMDLevel.AVX2;

        if (SupportsAdvSimd)
            return SIMDLevel.NEON;

        if (SupportsSse3)
            return SIMDLevel.SSE3;

        return SIMDLevel.Scalar;
    }

    /// <summary>
    /// Human-readable description of available SIMD support
    /// </summary>
    public static string GetSIMDDescription()
    {
        var level = GetSIMDLevel();
        return level switch
        {
            SIMDLevel.AVX2 => $"AVX2 (256-bit), BMI2: {SupportsBmi2}, POPCNT: {SupportsPopcnt}",
            SIMDLevel.NEON => $"ARM64 NEON (128-bit)",
            SIMDLevel.SSE3 => $"SSE3 (128-bit), POPCNT: {SupportsPopcnt}",
            SIMDLevel.Scalar => "Scalar (no SIMD acceleration)",
            _ => "Unknown"
        };
    }

    /// <summary>
    /// Count the number of set bits (population count) in a ulong
    /// Uses hardware POPCNT instruction if available
    /// </summary>
    [System.Runtime.CompilerServices.MethodImpl(System.Runtime.CompilerServices.MethodImplOptions.AggressiveInlining)]
    public static int PopCount(ulong value)
    {
        // Use System.Numerics.BitOperations which uses hardware POPCNT when available
        return System.Numerics.BitOperations.PopCount(value);
    }

    /// <summary>
    /// Count the number of trailing zeros in a ulong
    /// </summary>
    [System.Runtime.CompilerServices.MethodImpl(System.Runtime.CompilerServices.MethodImplOptions.AggressiveInlining)]
    public static int TrailingZeroCount(ulong value)
    {
        if (value == 0)
            return 64;

        // Use System.Numerics.BitOperations which uses hardware TZCNT when available
        return System.Numerics.BitOperations.TrailingZeroCount(value);
    }

    /// <summary>
    /// Get the index of the most significant set bit
    /// </summary>
    [System.Runtime.CompilerServices.MethodImpl(System.Runtime.CompilerServices.MethodImplOptions.AggressiveInlining)]
    public static int Log2(ulong value)
    {
        // Use System.Numerics.BitOperations which uses hardware LZCNT/BMI1 when available
        return System.Numerics.BitOperations.Log2(value);
    }

    /// <summary>
    /// Reverse the bits in a ulong
    /// </summary>
    public static ulong ReverseBits(ulong value)
    {
        value = ((value >> 1) & 0x5555555555555555UL) | ((value & 0x5555555555555555UL) << 1);
        value = ((value >> 2) & 0x3333333333333333UL) | ((value & 0x3333333333333333UL) << 2);
        value = ((value >> 4) & 0x0F0F0F0F0F0F0F0FUL) | ((value & 0x0F0F0F0F0F0F0F0FUL) << 4);
        value = ((value >> 8) & 0x00FF00FF00FF00FFUL) | ((value & 0x00FF00FF00FF00FFUL) << 8);
        value = ((value >> 16) & 0x0000FFFF0000FFFFUL) | ((value & 0x0000FFFF0000FFFFUL) << 16);
        value = (value >> 32) | (value << 32);
        return value;
    }
}

/// <summary>
/// SIMD acceleration levels
/// </summary>
public enum SIMDLevel
{
    /// <summary>No SIMD acceleration available</summary>
    Scalar = 0,

    /// <summary>SSE3 (128-bit vectors)</summary>
    SSE3 = 1,

    /// <summary>AVX2 (256-bit vectors)</summary>
    AVX2 = 2,

    /// <summary>ARM64 NEON (128-bit vectors)</summary>
    NEON = 3
}
