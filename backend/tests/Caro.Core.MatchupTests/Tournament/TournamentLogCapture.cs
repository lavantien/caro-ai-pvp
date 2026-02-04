using Caro.Core.GameLogic;
using Caro.Core.Tournament;
using System.Text;
using System.Text.Json;
using System.Text.RegularExpressions;

namespace Caro.Core.MatchupTests.Tournament;

/// <summary>
/// Snapshot data structures for tournament integration tests
/// These are JSON-serializable for saving to disk and regression testing
/// </summary>

public class TournamentSnapshot
{
    public string TestName { get; set; } = string.Empty;
    public DateTime TimestampUtc { get; set; }
    public List<GameSnapshot> Games { get; set; } = new();
    public TournamentSummary Summary { get; set; } = new();
}

public class GameSnapshot
{
    public string GameId { get; set; } = string.Empty;
    public string RedBot { get; set; } = string.Empty;
    public string BlueBot { get; set; } = string.Empty;
    public AIDifficulty RedDifficulty { get; set; }
    public AIDifficulty BlueDifficulty { get; set; }
    public List<MoveLogEntry> MoveLogs { get; set; } = new();
    public List<string> RawLogs { get; set; } = new();  // Full log text for inspection
    public GameResult Result { get; set; } = new();
}

public class MoveLogEntry
{
    public int MoveNumber { get; set; }
    public string Player { get; set; } = string.Empty;  // "red" or "blue"
    public int X { get; set; }
    public int Y { get; set; }
    public long TimeMs { get; set; }
    public int DepthAchieved { get; set; }
    public long NodesSearched { get; set; }
    public double NodesPerSecond { get; set; }
    public int VCFDepth { get; set; }
    public long VCFNodes { get; set; }
    public bool PonderingActive { get; set; }
}

public class GameResult
{
    public string Winner { get; set; } = string.Empty;  // "red", "blue", or "draw"
    public string WinnerDifficulty { get; set; } = string.Empty;
    public int TotalMoves { get; set; }
    public long DurationMs { get; set; }
    public bool EndedByTimeout { get; set; }
    public bool HadIllegalMove { get; set; }
}

public class TournamentSummary
{
    public int TotalGames { get; set; }
    public int RedWins { get; set; }
    public int BlueWins { get; set; }
    public int Draws { get; set; }
    public int IllegalMoves { get; set; }
    public Dictionary<string, int> WinsByDifficulty { get; set; } = new();
}

/// <summary>
/// Captures tournament game logs and creates serializable snapshots
/// for integration testing and regression detection
/// </summary>
public class TournamentLogCapture : IDisposable
{
    private readonly List<GameSnapshot> _games = new();
    private GameSnapshot? _currentGame;
    private readonly List<string> _rawLogs = new();
    private bool _hadIllegalMove;

    private static readonly JsonSerializerOptions JsonOptions = new()
    {
        WriteIndented = true,
        PropertyNamingPolicy = JsonNamingPolicy.CamelCase
    };

    // Regex to parse move logs like: "Move #5: (7,8) | 123ms [pondering] | Depth: 6 | Nodes: 12,345 | NPS: 100,000 | VCF: 4d/500n"
    private static readonly Regex MoveLogRegex = new(
        @"Move #(?<move>\d+):\s+\((?<x>\d+),(?<y>\d+)\)\s*\|\s*(?<time>\d+)ms(\s*\[pondering\])?\s*\|\s*Depth:\s*(?<depth>\d+)\s*\|\s*Nodes:\s*(?<nodes>[\d,]+)\s*\|\s*NPS:\s*(?<nps>[\d,]+)(?:\s*\|\s*VCF:\s*(?<vcfDepth>\d+)d/(?<vcfNodes>[\d,]+)n)?",
        RegexOptions.Compiled | RegexOptions.IgnoreCase
    );

    /// <summary>
    /// Get a LogCallback for a new game
    /// </summary>
    public LogCallback GetCallback(string gameId, string redBot, string blueBot,
                                   AIDifficulty redDiff, AIDifficulty blueDiff)
    {
        _currentGame = new GameSnapshot
        {
            GameId = gameId,
            RedBot = redBot,
            BlueBot = blueBot,
            RedDifficulty = redDiff,
            BlueDifficulty = blueDiff,
            MoveLogs = new(),
            RawLogs = new()
        };
        _rawLogs.Clear();
        _hadIllegalMove = false;

        return (level, source, message) =>
        {
            _rawLogs.Append($"{DateTime.UtcNow:O} [{level.ToUpper()}] {source}: {message}");

            // Check for illegal move
            if (message.Contains("ILLEGAL MOVE", StringComparison.OrdinalIgnoreCase))
            {
                _hadIllegalMove = true;
            }

            // Try to parse move statistics
            var match = MoveLogRegex.Match(message);
            if (match.Success)
            {
                var entry = new MoveLogEntry
                {
                    MoveNumber = int.Parse(match.Groups["move"].Value),
                    X = int.Parse(match.Groups["x"].Value),
                    Y = int.Parse(match.Groups["y"].Value),
                    TimeMs = long.Parse(match.Groups["time"].Value),
                    DepthAchieved = int.Parse(match.Groups["depth"].Value),
                    NodesSearched = long.Parse(match.Groups["nodes"].Value.Replace(",", "")),
                    NodesPerSecond = double.Parse(match.Groups["nps"].Value.Replace(",", "")),
                    Player = source,
                    PonderingActive = message.Contains("[pondering]", StringComparison.OrdinalIgnoreCase)
                };

                if (match.Groups["vcfDepth"].Success)
                {
                    entry.VCFDepth = int.Parse(match.Groups["vcfDepth"].Value);
                    entry.VCFNodes = long.Parse(match.Groups["vcfNodes"].Value.Replace(",", ""));
                }

                _currentGame.MoveLogs.Add(entry);
            }
        };
    }

    /// <summary>
    /// Finalize a game with its result
    /// </summary>
    public void FinalizeGame(MatchResult result)
    {
        if (_currentGame == null)
            throw new InvalidOperationException("No game in progress. Call GetCallback first.");

        _currentGame.RawLogs = new List<string>(_rawLogs);
        _currentGame.Result = new GameResult
        {
            Winner = result.Winner.ToString().ToLower(),
            WinnerDifficulty = result.WinnerDifficulty.ToString(),
            TotalMoves = result.TotalMoves,
            DurationMs = result.DurationMs,
            EndedByTimeout = result.EndedByTimeout,
            HadIllegalMove = _hadIllegalMove
        };

        _games.Add(_currentGame);
        _currentGame = null;
    }

    /// <summary>
    /// Build the complete tournament snapshot
    /// </summary>
    public TournamentSnapshot BuildSnapshot(string testName)
    {
        var summary = new TournamentSummary
        {
            TotalGames = _games.Count,
            RedWins = _games.Count(g => g.Result.Winner == "red"),
            BlueWins = _games.Count(g => g.Result.Winner == "blue"),
            Draws = _games.Count(g => g.Result.Winner == "draw"),
            IllegalMoves = _games.Count(g => g.Result.HadIllegalMove),
            WinsByDifficulty = new()
        };

        foreach (var game in _games)
        {
            var key = game.Result.WinnerDifficulty;
            if (!string.IsNullOrEmpty(key) && key != "None")
            {
                summary.WinsByDifficulty.TryGetValue(key, out int count);
                summary.WinsByDifficulty[key] = count + 1;
            }
        }

        return new TournamentSnapshot
        {
            TestName = testName,
            TimestampUtc = DateTime.UtcNow,
            Games = new List<GameSnapshot>(_games),
            Summary = summary
        };
    }

    /// <summary>
    /// Save snapshot to JSON file
    /// </summary>
    public static async Task SaveSnapshotAsync(string path, TournamentSnapshot snapshot)
    {
        var directory = Path.GetDirectoryName(path);
        if (!string.IsNullOrEmpty(directory) && !Directory.Exists(directory))
        {
            Directory.CreateDirectory(directory);
        }

        var json = JsonSerializer.Serialize(snapshot, JsonOptions);
        await File.WriteAllTextAsync(path, json);
    }

    /// <summary>
    /// Save the current snapshot to a file
    /// </summary>
    public async Task SaveSnapshotAsync(string testName, string snapshotDirectory)
    {
        var snapshot = BuildSnapshot(testName);
        var sanitizedTestName = string.Join("_", testName.Split(Path.GetInvalidFileNameChars()));
        var path = Path.Combine(snapshotDirectory, $"{sanitizedTestName}.json");
        await SaveSnapshotAsync(path, snapshot);
    }

    /// <summary>
    /// Load snapshot from JSON file
    /// </summary>
    public static async Task<TournamentSnapshot> LoadSnapshotAsync(string path)
    {
        var json = await File.ReadAllTextAsync(path);
        return JsonSerializer.Deserialize<TournamentSnapshot>(json, JsonOptions)
            ?? throw new InvalidOperationException($"Failed to deserialize snapshot from {path}");
    }

    /// <summary>
    /// Get all snapshot files from a directory
    /// </summary>
    public static List<string> GetSnapshotFiles(string directory)
    {
        if (!Directory.Exists(directory))
            return new List<string>();

        return Directory.GetFiles(directory, "*.json").OrderBy(f => f).ToList();
    }

    public void Dispose()
    {
        _games.Clear();
        _rawLogs.Clear();
        GC.SuppressFinalize(this);
    }
}
