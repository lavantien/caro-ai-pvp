using Caro.Core.Domain.Configuration;
using Caro.Core.Domain.Entities;
using Caro.Core.GameLogic;
using Xunit;

namespace Caro.Core.Tests.GameLogic.OpeningBook;

public class BinaryBookFormatTests
{
    [Fact]
    public void VarUInt64_RoundTrip_Zero()
    {
        var buffer = new byte[10];
        var written = BinaryBookFormat.WriteVarUInt64(buffer, 0);
        var (value, read) = BinaryBookFormat.ReadVarUInt64(buffer);

        Assert.Equal(1, written);
        Assert.Equal(1, read);
        Assert.Equal(0ul, value);
    }

    [Theory]
    [InlineData(0x7F)]      // 1 byte max
    [InlineData(0x80)]      // 2 byte min
    [InlineData(0x3FFF)]    // 2 byte max
    [InlineData(0x4000)]    // 3 byte min
    [InlineData(0xFFFFFF)]  // 3 byte max-ish
    [InlineData(ulong.MaxValue)]
    public void VarUInt64_RoundTrip_VariousValues(ulong original)
    {
        var buffer = new byte[10];
        var written = BinaryBookFormat.WriteVarUInt64(buffer, original);
        var (value, read) = BinaryBookFormat.ReadVarUInt64(buffer);

        Assert.Equal(written, read);
        Assert.Equal(original, value);
    }

    [Theory]
    [InlineData(0)]
    [InlineData(1)]
    [InlineData(-1)]
    [InlineData(long.MaxValue)]
    [InlineData(long.MinValue)]
    [InlineData(0x7FFFFFFF)]
    [InlineData(-0x80000000)]
    public void VarInt64_RoundTrip_VariousValues(long original)
    {
        var buffer = new byte[10];
        var written = BinaryBookFormat.WriteVarInt64(buffer, original);
        var (value, read) = BinaryBookFormat.ReadVarInt64(buffer);

        Assert.Equal(written, read);
        Assert.Equal(original, value);
    }

    [Theory]
    [InlineData(0)]
    [InlineData(1)]
    [InlineData(127)]
    [InlineData(128)]
    [InlineData(16383)]
    [InlineData(16384)]
    [InlineData(int.MaxValue)]
    public void VarInt32_RoundTrip_VariousValues(int original)
    {
        var buffer = new byte[10];
        var written = BinaryBookFormat.WriteVarInt32(buffer, original);
        var (value, read) = BinaryBookFormat.ReadVarInt32(buffer);

        Assert.Equal(written, read);
        Assert.Equal(original, value);
    }

    [Fact]
    public void VarInt32_Negative_Throws()
    {
        var buffer = new byte[10];
        Assert.Throws<ArgumentOutOfRangeException>(() => BinaryBookFormat.WriteVarInt32(buffer, -1));
    }

    [Fact]
    public void Header_RoundTrip()
    {
        var buffer = new byte[BinaryBookFormat.HeaderSize];
        var generatedAt = DateTime.UtcNow;

        BinaryBookFormat.WriteHeader(buffer, 1000, 5000, generatedAt.Ticks, 16);

        var (entryCount, totalMoves, generatedAtTicks, maxDepth) = BinaryBookFormat.ReadHeader(buffer);

        Assert.Equal(1000, entryCount);
        Assert.Equal(5000, totalMoves);
        Assert.Equal(generatedAt.Ticks, generatedAtTicks);
        Assert.Equal(16, maxDepth);
    }

    [Fact]
    public void Footer_RoundTrip()
    {
        var buffer = new byte[BinaryBookFormat.FooterSize];

        BinaryBookFormat.WriteFooter(buffer, 1000, 5000, 0x12345678ABCDEF00);

        var (entryCount, totalMoves, checksum) = BinaryBookFormat.ReadFooter(buffer);

        Assert.Equal(1000, entryCount);
        Assert.Equal(5000, totalMoves);
        Assert.Equal(0x12345678ABCDEF00ul, checksum);
    }

    [Fact]
    public void ComputeChecksum_Deterministic()
    {
        var data1 = new byte[] { 1, 2, 3, 4, 5 };
        var data2 = new byte[] { 1, 2, 3, 4, 5 };

        var hash1 = BinaryBookFormat.ComputeChecksum(data1);
        var hash2 = BinaryBookFormat.ComputeChecksum(data2);

        Assert.Equal(hash1, hash2);
    }

    [Fact]
    public void ComputeChecksum_DifferentData_DifferentHash()
    {
        var data1 = new byte[] { 1, 2, 3, 4, 5 };
        var data2 = new byte[] { 1, 2, 3, 4, 6 };

        var hash1 = BinaryBookFormat.ComputeChecksum(data1);
        var hash2 = BinaryBookFormat.ComputeChecksum(data2);

        Assert.NotEqual(hash1, hash2);
    }
}

public class BinaryBookExporterTests
{
    private static OpeningBookEntry CreateTestEntry(int seed = 0)
    {
        var random = new Random(seed);
        return new OpeningBookEntry
        {
            CanonicalHash = (ulong)random.NextInt64(),
            DirectHash = (ulong)random.NextInt64(),
            Depth = random.Next(20),
            Player = random.Next(2) == 0 ? Player.Red : Player.Blue,
            Symmetry = (SymmetryType)random.Next(8),
            IsNearEdge = random.Next(2) == 0,
            Moves = new[]
            {
                new BookMove
                {
                    RelativeX = random.Next(19),
                    RelativeY = random.Next(19),
                    WinRate = random.Next(101),
                    DepthAchieved = random.Next(20),
                    NodesSearched = random.NextInt64(),
                    Score = random.Next(-10000, 10000),
                    IsForcing = random.Next(2) == 0,
                    IsVerified = random.Next(2) == 0,
                    Source = (MoveSource)random.Next(3),
                    Priority = random.Next(16),
                    ScoreDelta = random.Next(-500, 500),
                    WinCount = random.Next(1000),
                    PlayCount = random.Next(1000)
                }
            }
        };
    }

    [Fact]
    public void Export_SingleEntry_RoundTrip()
    {
        var entry = CreateTestEntry(42);
        var exporter = new BinaryBookExporter();
        var importer = new BinaryBookImporter();
        var tempFile = Path.GetTempFileName();

        try
        {
            var exportResult = exporter.Export(new[] { entry }, tempFile);
            var importResult = importer.Import(tempFile);

            Assert.Single(importResult.Entries);
            var importedEntry = importResult.Entries[0];

            Assert.Equal(entry.CanonicalHash, importedEntry.CanonicalHash);
            Assert.Equal(entry.DirectHash, importedEntry.DirectHash);
            Assert.Equal(entry.Depth, importedEntry.Depth);
            Assert.Equal(entry.Player, importedEntry.Player);
            Assert.Equal(entry.Symmetry, importedEntry.Symmetry);
            Assert.Equal(entry.IsNearEdge, importedEntry.IsNearEdge);
            Assert.Single(importedEntry.Moves);

            var originalMove = entry.Moves[0];
            var importedMove = importedEntry.Moves[0];
            Assert.Equal(originalMove.RelativeX, importedMove.RelativeX);
            Assert.Equal(originalMove.RelativeY, importedMove.RelativeY);
            Assert.Equal(originalMove.WinRate, importedMove.WinRate);
            Assert.Equal(originalMove.Score, importedMove.Score);
            Assert.Equal(originalMove.IsForcing, importedMove.IsForcing);
            Assert.Equal(originalMove.Source, importedMove.Source);
        }
        finally
        {
            File.Delete(tempFile);
        }
    }

    [Fact]
    public void Export_MultipleEntries_RoundTrip()
    {
        var entries = Enumerable.Range(0, 100).Select(i => CreateTestEntry(i)).ToList();
        var exporter = new BinaryBookExporter();
        var importer = new BinaryBookImporter();
        var tempFile = Path.GetTempFileName();

        try
        {
            var exportResult = exporter.Export(entries, tempFile);
            var importResult = importer.Import(tempFile);

            Assert.Equal(100, importResult.Entries.Count);
            Assert.Equal(100, exportResult.EntriesExported);
        }
        finally
        {
            File.Delete(tempFile);
        }
    }

    [Fact]
    public void Export_ChecksumVerification_Succeeds()
    {
        var entry = CreateTestEntry();
        var exporter = new BinaryBookExporter();
        var importer = new BinaryBookImporter();
        var tempFile = Path.GetTempFileName();

        try
        {
            exporter.Export(new[] { entry }, tempFile);
            var result = importer.Import(tempFile, verifyChecksum: true);
            Assert.Single(result.Entries);
        }
        finally
        {
            File.Delete(tempFile);
        }
    }

    [Fact]
    public void Export_InvalidChecksum_Throws()
    {
        var entry = CreateTestEntry();
        var exporter = new BinaryBookExporter();
        var importer = new BinaryBookImporter();
        var tempFile = Path.GetTempFileName();

        try
        {
            exporter.Export(new[] { entry }, tempFile);

            // Corrupt the file
            var bytes = File.ReadAllBytes(tempFile);
            bytes[50] ^= 0xFF;
            File.WriteAllBytes(tempFile, bytes);

            Assert.Throws<InvalidDataException>(() => importer.Import(tempFile, verifyChecksum: true));
        }
        finally
        {
            File.Delete(tempFile);
        }
    }

    [Fact]
    public void Validate_ValidFile_Succeeds()
    {
        var entry = CreateTestEntry();
        var exporter = new BinaryBookExporter();
        var importer = new BinaryBookImporter();
        var tempFile = Path.GetTempFileName();

        try
        {
            exporter.Export(new[] { entry }, tempFile);
            var result = importer.Validate(tempFile);

            Assert.True(result.IsValid);
            Assert.Equal(1, result.EntryCount);
        }
        finally
        {
            File.Delete(tempFile);
        }
    }

    [Fact]
    public void Validate_NonexistentFile_ReturnsInvalid()
    {
        var importer = new BinaryBookImporter();
        var result = importer.Validate("nonexistent.cobook");

        Assert.False(result.IsValid);
        Assert.NotNull(result.ErrorMessage);
    }
}
