using System.Globalization;
using Microsoft.Data.Sqlite;

namespace Caro.Persistence;

public sealed class GameRecord
{
    public string ID { get; set; } = "";
    public string GameMode { get; set; } = "";
    public string TimeControl { get; set; } = "";
    public string RedType { get; set; } = "";
    public string BlueType { get; set; } = "";
    public int? RedDifficulty { get; set; }
    public int? BlueDifficulty { get; set; }
    public string Winner { get; set; } = "";
    public int MoveCount { get; set; }
    public DateTime CreatedAt { get; set; }
    public DateTime? CompletedAt { get; set; }
}

public sealed class MoveRecord
{
    public string GameID { get; set; } = "";
    public int MoveNumber { get; set; }
    public string Player { get; set; } = "";
    public int PosX { get; set; }
    public int PosY { get; set; }
    public bool IsBot { get; set; }
    public int? Difficulty { get; set; }
    public long? ThinkTimeMs { get; set; }
    public long? RemainingTimeMs { get; set; }
    public int? SearchDepth { get; set; }
    public long? NodesSearched { get; set; }
    public double? NPS { get; set; }
    public double? TTHitRate { get; set; }
    public int? SearchScore { get; set; }
    public int? ThreadsUsed { get; set; }
    public long? AllocatedTimeMs { get; set; }
    public string? MoveType { get; set; }
    public double? MasterPct { get; set; }
    public int? SlaveDepth { get; set; }
    public long? SlaveNodes { get; set; }
    public int? PonderDepth { get; set; }
    public long? PonderNodes { get; set; }
    public int? VcfDepth { get; set; }
    public long? VcfNodes { get; set; }
}

/// <summary>
/// SQLite-backed match archive. One serialized connection: every access is a
/// quick indexed statement, so a lock beats pooling complexity.
/// </summary>
public sealed class MatchStore : IDisposable
{
    private readonly object _gate = new();
    private readonly SqliteConnection _db;

    private const string MatchSchema = """
        CREATE TABLE IF NOT EXISTS games (
            id TEXT PRIMARY KEY,
            game_mode TEXT NOT NULL,
            time_control TEXT NOT NULL,
            red_type TEXT NOT NULL,
            blue_type TEXT NOT NULL,
            red_difficulty INTEGER,
            blue_difficulty INTEGER,
            winner TEXT NOT NULL DEFAULT 'none',
            move_count INTEGER NOT NULL DEFAULT 0,
            created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
            completed_at DATETIME
        );

        CREATE TABLE IF NOT EXISTS moves (
            id INTEGER PRIMARY KEY AUTOINCREMENT,
            game_id TEXT NOT NULL REFERENCES games(id),
            move_number INTEGER NOT NULL,
            player TEXT NOT NULL,
            pos_x INTEGER NOT NULL,
            pos_y INTEGER NOT NULL,
            is_bot INTEGER NOT NULL DEFAULT 0,
            difficulty INTEGER,
            think_time_ms INTEGER,
            remaining_time_ms INTEGER,
            search_depth INTEGER,
            nodes_searched INTEGER,
            nps REAL,
            tt_hit_rate REAL,
            search_score INTEGER,
            threads_used INTEGER,
            allocated_time_ms INTEGER,
            move_type TEXT,
            created_at DATETIME DEFAULT CURRENT_TIMESTAMP
        );

        CREATE INDEX IF NOT EXISTS idx_moves_game_id ON moves(game_id);
        CREATE INDEX IF NOT EXISTS idx_games_created_at ON games(created_at);
        """;

    private const int DefaultTimeoutSeconds = 5;

    public MatchStore(string dbPath)
    {
        string? dir = Path.GetDirectoryName(dbPath);
        if (!string.IsNullOrEmpty(dir))
        {
            Directory.CreateDirectory(dir);
        }

        _db = new SqliteConnection($"Data Source={dbPath};Default Timeout={DefaultTimeoutSeconds}");
        _db.Open();

        Exec("PRAGMA journal_mode=WAL;");
        Exec(MatchSchema);
        Migrate();
    }

    private void Exec(string sql)
    {
        using SqliteCommand cmd = new(sql, _db);
        cmd.ExecuteNonQuery();
    }

    private void Migrate()
    {
        (string Column, string Type)[] newCols =
        [
            ("master_pct", "REAL"),
            ("slave_depth", "INTEGER"),
            ("slave_nodes", "INTEGER"),
            ("ponder_depth", "INTEGER"),
            ("ponder_nodes", "INTEGER"),
            ("vcf_depth", "INTEGER"),
            ("vcf_nodes", "INTEGER"),
        ];

        HashSet<string> existing = [];
        using (SqliteCommand cmd = new("PRAGMA table_info(moves)", _db))
        using (SqliteDataReader reader = cmd.ExecuteReader())
        {
            while (reader.Read())
            {
                existing.Add(reader.GetString(1));
            }
        }

        foreach ((string column, string type) in newCols)
        {
            if (!existing.Contains(column))
            {
                Exec($"ALTER TABLE moves ADD COLUMN {column} {type}");
            }
        }
    }

    public void CreateGame(GameRecord g)
    {
        const string sql = """
            INSERT INTO games (id, game_mode, time_control, red_type, blue_type, red_difficulty, blue_difficulty)
            VALUES (@id, @mode, @tc, @rt, @bt, @rd, @bd)
            """;
        lock (_gate)
        {
            using SqliteCommand cmd = new(sql, _db);
            cmd.Parameters.AddWithValue("@id", g.ID);
            cmd.Parameters.AddWithValue("@mode", g.GameMode);
            cmd.Parameters.AddWithValue("@tc", g.TimeControl);
            cmd.Parameters.AddWithValue("@rt", g.RedType);
            cmd.Parameters.AddWithValue("@bt", g.BlueType);
            cmd.Parameters.AddWithValue("@rd", Param(g.RedDifficulty));
            cmd.Parameters.AddWithValue("@bd", Param(g.BlueDifficulty));
            cmd.ExecuteNonQuery();
        }
    }

    public void RecordMove(MoveRecord m)
    {
        const string sql = """
            INSERT INTO moves (game_id, move_number, player, pos_x, pos_y, is_bot, difficulty,
                think_time_ms, remaining_time_ms, search_depth, nodes_searched, nps, tt_hit_rate,
                search_score, threads_used, allocated_time_ms, move_type,
                master_pct, slave_depth, slave_nodes, ponder_depth, ponder_nodes, vcf_depth, vcf_nodes)
            VALUES (@gid, @mn, @pl, @px, @py, @ib, @di,
                @tt, @rt, @sd, @ns, @nps, @ttu,
                @ss, @th, @at, @mt,
                @mp, @sld, @sln, @pd, @pn, @vd, @vn)
            """;
        lock (_gate)
        {
            using SqliteCommand cmd = new(sql, _db);
            cmd.Parameters.AddWithValue("@gid", m.GameID);
            cmd.Parameters.AddWithValue("@mn", m.MoveNumber);
            cmd.Parameters.AddWithValue("@pl", m.Player);
            cmd.Parameters.AddWithValue("@px", m.PosX);
            cmd.Parameters.AddWithValue("@py", m.PosY);
            cmd.Parameters.AddWithValue("@ib", m.IsBot ? 1 : 0);
            cmd.Parameters.AddWithValue("@di", Param(m.Difficulty));
            cmd.Parameters.AddWithValue("@tt", Param(m.ThinkTimeMs));
            cmd.Parameters.AddWithValue("@rt", Param(m.RemainingTimeMs));
            cmd.Parameters.AddWithValue("@sd", Param(m.SearchDepth));
            cmd.Parameters.AddWithValue("@ns", Param(m.NodesSearched));
            cmd.Parameters.AddWithValue("@nps", Param(m.NPS));
            cmd.Parameters.AddWithValue("@ttu", Param(m.TTHitRate));
            cmd.Parameters.AddWithValue("@ss", Param(m.SearchScore));
            cmd.Parameters.AddWithValue("@th", Param(m.ThreadsUsed));
            cmd.Parameters.AddWithValue("@at", Param(m.AllocatedTimeMs));
            cmd.Parameters.AddWithValue("@mt", Param(m.MoveType));
            cmd.Parameters.AddWithValue("@mp", Param(m.MasterPct));
            cmd.Parameters.AddWithValue("@sld", Param(m.SlaveDepth));
            cmd.Parameters.AddWithValue("@sln", Param(m.SlaveNodes));
            cmd.Parameters.AddWithValue("@pd", Param(m.PonderDepth));
            cmd.Parameters.AddWithValue("@pn", Param(m.PonderNodes));
            cmd.Parameters.AddWithValue("@vd", Param(m.VcfDepth));
            cmd.Parameters.AddWithValue("@vn", Param(m.VcfNodes));
            cmd.ExecuteNonQuery();
        }
    }

    public void CompleteGame(string gameID, string winner, int moveCount)
    {
        const string sql =
            "UPDATE games SET winner = @w, move_count = @mc, completed_at = CURRENT_TIMESTAMP WHERE id = @id";
        lock (_gate)
        {
            using SqliteCommand cmd = new(sql, _db);
            cmd.Parameters.AddWithValue("@w", winner);
            cmd.Parameters.AddWithValue("@mc", moveCount);
            cmd.Parameters.AddWithValue("@id", gameID);
            cmd.ExecuteNonQuery();
        }
    }

    public GameRecord? GetGame(string gameID)
    {
        const string sql = """
            SELECT id, game_mode, time_control, red_type, blue_type, red_difficulty, blue_difficulty,
                   winner, move_count, created_at, completed_at
            FROM games WHERE id = @id
            """;
        lock (_gate)
        {
            using SqliteCommand cmd = new(sql, _db);
            cmd.Parameters.AddWithValue("@id", gameID);
            using SqliteDataReader reader = cmd.ExecuteReader();
            if (!reader.Read())
            {
                return null;
            }
            return new GameRecord
            {
                ID = reader.GetString(0),
                GameMode = reader.GetString(1),
                TimeControl = reader.GetString(2),
                RedType = reader.GetString(3),
                BlueType = reader.GetString(4),
                RedDifficulty = NullableInt(reader, 5),
                BlueDifficulty = NullableInt(reader, 6),
                Winner = reader.GetString(7),
                MoveCount = reader.GetInt32(8),
                CreatedAt = ParseSqliteTimestamp(GetString(reader, 9)),
                CompletedAt = reader.IsDBNull(10)
                    ? null
                    : ParseSqliteTimestamp(reader.GetString(10)),
            };
        }
    }

    public List<MoveRecord> GetMoves(string gameID)
    {
        const string sql = """
            SELECT move_number, player, pos_x, pos_y, is_bot, difficulty,
                   think_time_ms, remaining_time_ms, search_depth, nodes_searched,
                   nps, tt_hit_rate, search_score, threads_used, allocated_time_ms, move_type,
                   master_pct, slave_depth, slave_nodes, ponder_depth, ponder_nodes,
                   vcf_depth, vcf_nodes
            FROM moves WHERE game_id = @id ORDER BY move_number
            """;
        lock (_gate)
        {
            using SqliteCommand cmd = new(sql, _db);
            cmd.Parameters.AddWithValue("@id", gameID);
            using SqliteDataReader reader = cmd.ExecuteReader();

            List<MoveRecord> moves = [];
            while (reader.Read())
            {
                moves.Add(new MoveRecord
                {
                    GameID = gameID,
                    MoveNumber = reader.GetInt32(0),
                    Player = reader.GetString(1),
                    PosX = reader.GetInt32(2),
                    PosY = reader.GetInt32(3),
                    IsBot = reader.GetInt64(4) != 0,
                    Difficulty = NullableInt(reader, 5),
                    ThinkTimeMs = NullableLong(reader, 6),
                    RemainingTimeMs = NullableLong(reader, 7),
                    SearchDepth = NullableInt(reader, 8),
                    NodesSearched = NullableLong(reader, 9),
                    NPS = NullableDouble(reader, 10),
                    TTHitRate = NullableDouble(reader, 11),
                    SearchScore = NullableInt(reader, 12),
                    ThreadsUsed = NullableInt(reader, 13),
                    AllocatedTimeMs = NullableLong(reader, 14),
                    MoveType = NullableString(reader, 15),
                    MasterPct = NullableDouble(reader, 16),
                    SlaveDepth = NullableInt(reader, 17),
                    SlaveNodes = NullableLong(reader, 18),
                    PonderDepth = NullableInt(reader, 19),
                    PonderNodes = NullableLong(reader, 20),
                    VcfDepth = NullableInt(reader, 21),
                    VcfNodes = NullableLong(reader, 22),
                });
            }
            return moves;
        }
    }

    public void Close()
    {
        lock (_gate)
        {
            _db.Close();
        }
    }

    public void Dispose() => Close();

    private static object Param(object? value) => value ?? DBNull.Value;

    private static int? NullableInt(SqliteDataReader reader, int ordinal) =>
        reader.IsDBNull(ordinal) ? null : reader.GetInt32(ordinal);

    private static long? NullableLong(SqliteDataReader reader, int ordinal) =>
        reader.IsDBNull(ordinal) ? null : reader.GetInt64(ordinal);

    private static double? NullableDouble(SqliteDataReader reader, int ordinal) =>
        reader.IsDBNull(ordinal) ? null : reader.GetDouble(ordinal);

    private static string? NullableString(SqliteDataReader reader, int ordinal) =>
        reader.IsDBNull(ordinal) ? null : reader.GetString(ordinal);

    private static string GetString(SqliteDataReader reader, int ordinal) =>
        reader.IsDBNull(ordinal) ? "" : reader.GetString(ordinal);

    private static DateTime ParseSqliteTimestamp(string s) =>
        DateTime.TryParseExact(s, "yyyy-MM-dd HH:mm:ss", CultureInfo.InvariantCulture,
            DateTimeStyles.None, out DateTime t) ? t : default;
}
