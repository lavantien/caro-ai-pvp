using Caro.Persistence;
using Microsoft.Data.Sqlite;
using Xunit;

namespace Caro.Persistence.Tests;

public sealed class TempDir : IDisposable
{
    public string Path { get; } = System.IO.Path.Combine(System.IO.Path.GetTempPath(), Guid.NewGuid().ToString("N"));

    public TempDir() => Directory.CreateDirectory(Path);

    public void Dispose()
    {
        try
        {
            Directory.Delete(Path, recursive: true);
        }
        catch (IOException)
        {
            // WAL files may still be held briefly; temp dirs are OS-cleaned.
        }
    }
}

public class MatchStoreTests
{
    [Fact]
    public void MatchStoreCreateAndRetrieveGame()
    {
        using TempDir dir = new();
        using MatchStore svc = new(System.IO.Path.Combine(dir.Path, "test.db"));

        GameRecord game = new()
        {
            ID = "abc123",
            GameMode = "aivai",
            TimeControl = "3+2",
            RedType = "bot",
            BlueType = "bot",
            RedDifficulty = 5,
            BlueDifficulty = 5,
        };
        svc.CreateGame(game);

        GameRecord? got = svc.GetGame("abc123");
        Assert.NotNull(got);
        Assert.Equal("abc123", got!.ID);
        Assert.Equal("aivai", got.GameMode);
        Assert.Equal("3+2", got.TimeControl);
        Assert.Equal("bot", got.RedType);
        Assert.Equal("bot", got.BlueType);
        Assert.Equal(5, got.RedDifficulty);
        Assert.Equal("none", got.Winner);
        Assert.Null(got.CompletedAt);
    }

    [Fact]
    public void MatchStoreRecordMoves()
    {
        using TempDir dir = new();
        using MatchStore svc = new(System.IO.Path.Combine(dir.Path, "test.db"));

        svc.CreateGame(new GameRecord
        {
            ID = "g1",
            GameMode = "pvai",
            TimeControl = "7+5",
            RedType = "human",
            BlueType = "bot",
            BlueDifficulty = 3,
        });

        svc.RecordMove(new MoveRecord
        {
            GameID = "g1",
            MoveNumber = 1,
            Player = "red",
            PosX = 8,
            PosY = 8,
            IsBot = false,
        });
        svc.RecordMove(new MoveRecord
        {
            GameID = "g1",
            MoveNumber = 2,
            Player = "blue",
            PosX = 7,
            PosY = 7,
            IsBot = true,
            Difficulty = 3,
            ThinkTimeMs = 1200,
            RemainingTimeMs = 415_000,
            SearchDepth = 42,
            NodesSearched = 54_321,
            NPS = 45267.5,
            TTHitRate = 0.35,
            SearchScore = 42,
            ThreadsUsed = 4,
            AllocatedTimeMs = 2000,
            MoveType = "exact",
        });

        List<MoveRecord> moves = svc.GetMoves("g1");
        Assert.Equal(2, moves.Count);

        Assert.Equal(1, moves[0].MoveNumber);
        Assert.Equal("red", moves[0].Player);
        Assert.Equal(8, moves[0].PosX);
        Assert.False(moves[0].IsBot);
        Assert.Null(moves[0].SearchDepth);

        Assert.Equal(2, moves[1].MoveNumber);
        Assert.Equal("blue", moves[1].Player);
        Assert.True(moves[1].IsBot);
        Assert.Equal(42, moves[1].SearchDepth);
        Assert.Equal(54_321L, moves[1].NodesSearched);
        Assert.Equal(45267.5, moves[1].NPS!.Value);
        Assert.Equal(0.35, moves[1].TTHitRate!.Value);
    }

    [Fact]
    public void MatchStoreCompleteGame()
    {
        using TempDir dir = new();
        using MatchStore svc = new(System.IO.Path.Combine(dir.Path, "test.db"));

        svc.CreateGame(new GameRecord
        {
            ID = "g2",
            GameMode = "pvp",
            TimeControl = "1+0",
            RedType = "human",
            BlueType = "human",
        });

        svc.CompleteGame("g2", "red", 27);

        GameRecord? got = svc.GetGame("g2");
        Assert.NotNull(got);
        Assert.Equal("red", got!.Winner);
        Assert.Equal(27, got.MoveCount);
        Assert.NotNull(got.CompletedAt);
    }

    [Fact]
    public void MatchStoreGetGameNotFound()
    {
        using TempDir dir = new();
        using MatchStore svc = new(System.IO.Path.Combine(dir.Path, "test.db"));

        Assert.Null(svc.GetGame("nonexistent"));
    }

    [Fact]
    public void MatchStoreCloseIdempotent()
    {
        using TempDir dir = new();
        MatchStore svc = new(System.IO.Path.Combine(dir.Path, "test.db"));
        svc.Close();
        svc.Close();
    }

    [Fact]
    public void MatchStoreDirectoryCreation()
    {
        using TempDir dir = new();
        MatchStore svc = new(System.IO.Path.Combine(dir.Path, "sub", "dir", "test.db"));
        svc.Close();
    }

    [Fact]
    public void NewMatchStoreInvalidPath()
    {
        // '|' is invalid in Windows path components; directory creation must fail.
        string badPath = System.IO.Path.Combine(System.IO.Path.GetTempPath(), $"bad|{Guid.NewGuid():N}", "test.db");
        Assert.ThrowsAny<Exception>(() => new MatchStore(badPath));
    }

    [Fact]
    public void MatchStoreGetMovesEmpty()
    {
        using TempDir dir = new();
        using MatchStore svc = new(System.IO.Path.Combine(dir.Path, "test.db"));

        svc.CreateGame(new GameRecord
        {
            ID = "g1",
            GameMode = "pvp",
            TimeControl = "1+0",
            RedType = "human",
            BlueType = "human",
        });

        List<MoveRecord> moves = svc.GetMoves("g1");
        Assert.Empty(moves);
    }

    [Fact]
    public void MatchStoreMigrationAddsColumns()
    {
        using TempDir dir = new();
        using MatchStore svc = new(System.IO.Path.Combine(dir.Path, "test.db"));

        svc.CreateGame(new GameRecord
        {
            ID = "g1",
            GameMode = "aivai",
            TimeControl = "3+2",
            RedType = "bot",
            BlueType = "bot",
        });

        svc.RecordMove(new MoveRecord
        {
            GameID = "g1",
            MoveNumber = 1,
            Player = "red",
            PosX = 8,
            PosY = 8,
            IsBot = true,
            Difficulty = 3,
        });

        List<MoveRecord> moves = svc.GetMoves("g1");
        Assert.Single(moves);
        Assert.Null(moves[0].MasterPct);
        Assert.Null(moves[0].SlaveDepth);
        Assert.Null(moves[0].SlaveNodes);
        Assert.Null(moves[0].PonderDepth);
        Assert.Null(moves[0].PonderNodes);
        Assert.Null(moves[0].VcfDepth);
        Assert.Null(moves[0].VcfNodes);
    }

    [Fact]
    public void MatchStoreRecordFutureStats()
    {
        using TempDir dir = new();
        using MatchStore svc = new(System.IO.Path.Combine(dir.Path, "test.db"));

        svc.CreateGame(new GameRecord
        {
            ID = "g1",
            GameMode = "aivai",
            TimeControl = "3+2",
            RedType = "bot",
            BlueType = "bot",
        });

        svc.RecordMove(new MoveRecord
        {
            GameID = "g1",
            MoveNumber = 1,
            Player = "red",
            PosX = 8,
            PosY = 8,
            IsBot = true,
            Difficulty = 5,
            MasterPct = 87.5,
            SlaveDepth = 10,
            SlaveNodes = 500_000,
            PonderDepth = 8,
            PonderNodes = 300_000,
            VcfDepth = 3,
            VcfNodes = 120_000,
        });

        List<MoveRecord> moves = svc.GetMoves("g1");
        Assert.Single(moves);
        Assert.Equal(87.5, moves[0].MasterPct!.Value);
        Assert.Equal(10, moves[0].SlaveDepth);
        Assert.Equal(500_000L, moves[0].SlaveNodes);
        Assert.Equal(8, moves[0].PonderDepth);
        Assert.Equal(300_000L, moves[0].PonderNodes);
        Assert.Equal(3, moves[0].VcfDepth);
        Assert.Equal(120_000L, moves[0].VcfNodes);
    }

    [Fact]
    public void MatchStoreMigrationFromOldSchema()
    {
        using TempDir dir = new();
        string dbPath = System.IO.Path.Combine(dir.Path, "test.db");

        const string oldSchema = """
            CREATE TABLE games (
                id TEXT PRIMARY KEY, game_mode TEXT NOT NULL, time_control TEXT NOT NULL,
                red_type TEXT NOT NULL, blue_type TEXT NOT NULL, red_difficulty INTEGER,
                blue_difficulty INTEGER, winner TEXT NOT NULL DEFAULT 'none',
                move_count INTEGER NOT NULL DEFAULT 0, created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
                completed_at DATETIME
            );
            CREATE TABLE moves (
                id INTEGER PRIMARY KEY AUTOINCREMENT, game_id TEXT NOT NULL REFERENCES games(id),
                move_number INTEGER NOT NULL, player TEXT NOT NULL, pos_x INTEGER NOT NULL,
                pos_y INTEGER NOT NULL, is_bot INTEGER NOT NULL DEFAULT 0, difficulty INTEGER,
                think_time_ms INTEGER, remaining_time_ms INTEGER, search_depth INTEGER,
                nodes_searched INTEGER, nps REAL, tt_hit_rate REAL, search_score INTEGER,
                threads_used INTEGER, allocated_time_ms INTEGER, move_type TEXT,
                created_at DATETIME DEFAULT CURRENT_TIMESTAMP
            );
            """;
        using (SqliteConnection db = new($"Data Source={dbPath}"))
        {
            db.Open();
            using SqliteCommand cmd = new(oldSchema, db);
            cmd.ExecuteNonQuery();
        }

        using MatchStore svc = new(dbPath);

        svc.CreateGame(new GameRecord
        {
            ID = "g1",
            GameMode = "pvp",
            TimeControl = "1+0",
            RedType = "human",
            BlueType = "human",
        });

        List<MoveRecord> moves = svc.GetMoves("g1");
        Assert.Empty(moves);
    }
}
