namespace Caro.Engine;

/// <summary>
/// Values reported in <see cref="SearchStats.MoveType"/> and persisted with
/// moves; the statline format depends on these exact strings.
/// </summary>
public static class MoveTypes
{
    public const string Vcf = "vcf";
    public const string Exact = "exact";
    public const string TimeoutFallback = "timeout-fallback";
}
