using Caro.Domain;

namespace Caro.Engine;

/// <summary>
/// Four-word bitboard over the 16x16 board. Value-type semantics like the
/// Go original: Set/Clear mutate the instance they are called on.
/// </summary>
public struct BitBoard
{
    private ulong _w0;
    private ulong _w1;
    private ulong _w2;
    private ulong _w3;

    private readonly ulong Word(int i) => i switch
    {
        0 => _w0,
        1 => _w1,
        2 => _w2,
        _ => _w3,
    };

    private void SetWord(int i, ulong v)
    {
        switch (i)
        {
            case 0: _w0 = v; break;
            case 1: _w1 = v; break;
            case 2: _w2 = v; break;
            default: _w3 = v; break;
        }
    }

    internal static (int Word, int Offset) BitIndex(int x, int y)
    {
        int idx = y * Constants.Board.Size + x;
        return (idx >> 6, idx & 63);
    }

    public void Set(int x, int y)
    {
        (int i, int off) = BitIndex(x, y);
        SetWord(i, Word(i) | (1UL << off));
    }

    public void Clear(int x, int y)
    {
        (int i, int off) = BitIndex(x, y);
        SetWord(i, Word(i) & ~(1UL << off));
    }

    public readonly bool Get(int x, int y)
    {
        (int i, int off) = BitIndex(x, y);
        return (Word(i) & (1UL << off)) != 0;
    }

    public readonly BitBoard Or(in BitBoard other) => new()
    {
        _w0 = _w0 | other._w0,
        _w1 = _w1 | other._w1,
        _w2 = _w2 | other._w2,
        _w3 = _w3 | other._w3,
    };

    public readonly BitBoard And(in BitBoard other) => new()
    {
        _w0 = _w0 & other._w0,
        _w1 = _w1 & other._w1,
        _w2 = _w2 & other._w2,
        _w3 = _w3 & other._w3,
    };

    public readonly BitBoard Xor(in BitBoard other) => new()
    {
        _w0 = _w0 ^ other._w0,
        _w1 = _w1 ^ other._w1,
        _w2 = _w2 ^ other._w2,
        _w3 = _w3 ^ other._w3,
    };

    public readonly BitBoard Not() => new()
    {
        _w0 = ~_w0,
        _w1 = ~_w1,
        _w2 = ~_w2,
        _w3 = ~_w3,
    };

    public readonly bool IsZero() => _w0 == 0 && _w1 == 0 && _w2 == 0 && _w3 == 0;

    public readonly int Count() =>
        (int)(ulong.PopCount(_w0) + ulong.PopCount(_w1) + ulong.PopCount(_w2) + ulong.PopCount(_w3));

    public readonly BitBoard Dilate()
    {
        const int W = Constants.Board.Size;
        BitBoard result = default;

        // Iterate over all 256 bits, for each set bit set all 8 neighbors
        for (int y = 0; y < W; y++)
        {
            for (int x = 0; x < W; x++)
            {
                if (!Get(x, y))
                {
                    continue;
                }
                for (int dy = -1; dy <= 1; dy++)
                {
                    for (int dx = -1; dx <= 1; dx++)
                    {
                        int nx = x + dx;
                        int ny = y + dy;
                        if (nx >= 0 && nx < W && ny >= 0 && ny < W)
                        {
                            result.Set(nx, ny);
                        }
                    }
                }
            }
        }
        return result;
    }

    public static (BitBoard Red, BitBoard Blue) BitBoardsFromDomain(Board b)
    {
        ulong[] rBits = b.BitBoardBits(Player.Red);
        ulong[] bBits = b.BitBoardBits(Player.Blue);
        return (
            new BitBoard { _w0 = rBits[0], _w1 = rBits[1], _w2 = rBits[2], _w3 = rBits[3] },
            new BitBoard { _w0 = bBits[0], _w1 = bBits[1], _w2 = bBits[2], _w3 = bBits[3] });
    }
}
