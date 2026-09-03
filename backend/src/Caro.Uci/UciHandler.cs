using System.Globalization;
using Caro.Domain;
using Caro.Engine;

namespace Caro.Uci;

/// <summary>
/// Receives one line per UCI command; implementations must emit each reply
/// line whole (newline included).
/// </summary>
public interface ILineWriter
{
    void WriteLine(string line);
}

public sealed class UciHandler(ILineWriter writer, CaroConfig? config = null) : IDisposable
{
    private static CaroConfig ConfigOf(CaroConfig? c) => c ?? CaroConfig.Default;

    private readonly CaroConfig _config = ConfigOf(config);
    private readonly object _gate = new();
    private MinimaxAI _ai = new(ConfigOf(config).Uci.Threads.Default, ConfigOf(config).Uci.HashMB.Default, ConfigOf(config).TimeManagement);
    private Board _board = Board.NewBoard();
    private Player _player = Player.Red;
    private CancellationTokenSource? _searchCts;
    private Task? _searchTask;
    private int _threads = ConfigOf(config).Uci.Threads.Default;
    private int _hashMB = ConfigOf(config).Uci.HashMB.Default;
    private int _skillLevel = ConfigOf(config).Uci.SkillLevel.Default;

    public Board CurrentBoard()
    {
        lock (_gate)
        {
            return _board;
        }
    }

    public int SkillLevel()
    {
        lock (_gate)
        {
            return _skillLevel;
        }
    }

    public int CurrentThreads()
    {
        lock (_gate)
        {
            return _threads;
        }
    }

    public void HandleCommand(string cmd)
    {
        string[] fields = cmd.Split(' ', StringSplitOptions.RemoveEmptyEntries);
        if (fields.Length == 0)
        {
            return;
        }

        switch (fields[0])
        {
            case "uci":
                Respond("id name Caro AI");
                Respond("id author Caro AI Project");
                Respond($"option name Threads type spin default {CurrentThreads()} min {_config.Uci.Threads.Min} max {_config.Uci.Threads.Max}");
                Respond($"option name Hash type spin default {_config.Uci.HashMB.Default} min {_config.Uci.HashMB.Min} max {_config.Uci.HashMB.Max}");
                Respond($"option name Skill Level type spin default {_config.Uci.SkillLevel.Default} min {_config.Uci.SkillLevel.Min} max {_config.Uci.SkillLevel.Max}");
                Respond("uciok");
                break;

            case "isready":
                Respond("readyok");
                break;

            case "ucinewgame":
                StopSearchAndWait();
                _board = Board.NewBoard();
                _player = Player.Red;
                RebuildAI();
                break;

            case "position":
                StopSearchAndWait();
                HandlePosition(fields[1..]);
                break;

            case "go":
                HandleGo(fields[1..]);
                break;

            case "stop":
                StopSearch();
                break;

            case "quit":
                StopSearchAndWait();
                _ai.Dispose();
                break;

            case "setoption":
                HandleSetOption(fields[1..]);
                break;
        }
    }

    private void HandlePosition(string[] args)
    {
        if (args.Length == 0)
        {
            return;
        }

        if (args[0] != "startpos")
        {
            Respond("info string error unsupported position argument");
            return;
        }
        _board = Board.NewBoard();
        _player = Player.Red;
        if (args.Length > 2 && args[1] == "moves")
        {
            List<Position> moves = new(args.Length - 2);
            foreach (string moveStr in args[2..])
            {
                if (!Notation.TryParseMove(moveStr, out int x, out int y))
                {
                    // Reject the whole command: partially applying it would
                    // desync the engine from the caller's board.
                    Respond($"info string error invalid move \"{moveStr}\"; position not changed");
                    return;
                }
                moves.Add(new Position(x, y));
            }
            foreach (Position m in moves)
            {
                // Placing on an occupied cell throws; a replayed game with a
                // duplicate move must error instead of taking down the engine.
                try
                {
                    _board = _board.PlaceStone(m.X, m.Y, _player);
                }
                catch (CaroException e)
                {
                    Respond($"info string error move {Notation.MoveToString(m.X, m.Y)}: {e.Message}; position not changed");
                    _board = Board.NewBoard();
                    _player = Player.Red;
                    return;
                }
                _player = _player.Opponent();
            }
        }
    }

    private void HandleGo(string[] args)
    {
        StopSearchAndWait();

        Board board;
        Player player;
        SearchOptions opts;
        lock (_gate)
        {
            board = _board;
            player = _player;
            opts = ParseGoOptions(args, player, SkillSearchOptions());
        }

        CancellationTokenSource cts = new();
        CancellationTokenSource linked = CancellationTokenSource.CreateLinkedTokenSource(cts.Token);
        CancellationToken token = linked.Token;

        lock (_gate)
        {
            _searchCts = cts;
            _searchTask = Task.Factory.StartNew(() =>
            {
                try
                {
                    (int x, int y, SearchStats stats) = _ai.GetBestMove(board, player, opts, token);
                    Respond(string.Create(CultureInfo.InvariantCulture,
                        $"info depth {stats.DepthAchieved} nodes {stats.NodesSearched} nps {stats.NodesPerSecond:F0} score cp {stats.SearchScore} tt-hitrate {stats.TableHitRate:F2} threads {stats.ThreadCount}"));
                    Respond($"bestmove {Notation.MoveToString(x, y)}");
                }
                finally
                {
                    linked.Dispose();
                }
            }, CancellationToken.None, TaskCreationOptions.LongRunning, TaskScheduler.Default);
        }
    }

    private void HandleSetOption(string[] args)
    {
        // Expected shape: "name <Name...> value <Value>"; the name may span
        // several tokens (e.g. "Skill Level").
        int nameStart = -1;
        int valueIdx = -1;
        for (int i = 0; i < args.Length; i++)
        {
            if (args[i] == "name" && i + 1 < args.Length)
            {
                nameStart = i + 1;
            }
            if (args[i] == "value")
            {
                valueIdx = i;
                break;
            }
        }
        if (nameStart < 0 || valueIdx < 0 || valueIdx <= nameStart || valueIdx + 1 >= args.Length)
        {
            return;
        }
        string name = string.Join(' ', args[nameStart..valueIdx]);
        string value = args[valueIdx + 1];

        if (!int.TryParse(value, out int n))
        {
            return;
        }

        lock (_gate)
        {
            switch (name)
            {
                case "Threads":
                    if (n >= _config.Uci.Threads.Min && n <= _config.Uci.Threads.Max)
                    {
                        _threads = n;
                        RebuildAI();
                    }
                    break;
                case "Hash":
                    if (n >= _config.Uci.HashMB.Min && n <= _config.Uci.HashMB.Max)
                    {
                        _hashMB = n;
                        RebuildAI();
                    }
                    break;
                case "Skill Level":
                    if (n >= _config.Uci.SkillLevel.Min && n <= _config.Uci.SkillLevel.Max)
                    {
                        _skillLevel = n;
                    }
                    break;
            }
        }
    }

    // RebuildAI recreates the engine with the configured threads/hash.
    // Callers must hold the gate and must have stopped any running search.
    private void RebuildAI()
    {
        _ai.Dispose();
        _ai = new MinimaxAI(_threads, _hashMB, _config.TimeManagement);
    }

    // SkillSearchOptions maps the configured skill level onto the engine's
    // strength profile, capped by the configured thread count.
    internal SearchOptions SkillSearchOptions()
    {
        int skill = _skillLevel;
        int threads = _threads;

        if (skill < _config.Uci.SkillLevel.Min || skill > _config.Uci.SkillLevel.Max)
        {
            skill = _config.Uci.SkillLevel.Default;
        }
        DifficultyProfile profile = Difficulty.GetDifficultyProfile(skill, _config);
        int engineThreads = Math.Min(threads, profile.Threads);
        return new SearchOptions
        {
            ThreadCount = engineThreads,
            ParallelEnabled = engineThreads > 1,
            TimeFraction = profile.TimeFraction,
            UseVCF = profile.UseVCF,
            VCFMaxDepth = profile.VCFDepth,
            MaxDepth = profile.MaxDepth,
        };
    }

    // ParseGoOptions overlays go-command arguments onto base options for the
    // given side. movetime maps to a fixed budget; wtime/winc or btime/binc
    // follow the side to move; depth caps the search.
    internal static SearchOptions ParseGoOptions(string[] args, Player player, SearchOptions @base)
    {
        for (int i = 0; i + 1 < args.Length; i++)
        {
            if (!long.TryParse(args[i + 1], out long val))
            {
                continue;
            }
            switch (args[i])
            {
                case "movetime":
                    @base.TimeRemainingMs = val;
                    @base.IncrementMs = 0;
                    break;
                case "depth":
                    if (val > 0)
                    {
                        @base.MaxDepth = (int)val;
                    }
                    break;
                case "wtime":
                    if (player == Player.Red)
                    {
                        @base.TimeRemainingMs = val;
                    }
                    break;
                case "btime":
                    if (player == Player.Blue)
                    {
                        @base.TimeRemainingMs = val;
                    }
                    break;
                case "winc":
                    if (player == Player.Red)
                    {
                        @base.IncrementMs = val;
                    }
                    break;
                case "binc":
                    if (player == Player.Blue)
                    {
                        @base.IncrementMs = val;
                    }
                    break;
            }
        }
        return @base;
    }

    private void StopSearch()
    {
        CancellationTokenSource? cts;
        lock (_gate)
        {
            cts = _searchCts;
        }
        cts?.Cancel();
    }

    // StopSearchAndWait cancels any running search and waits for its
    // bestmove to be emitted, so state changes never race an active search.
    private void StopSearchAndWait()
    {
        StopSearch();
        Task? task;
        lock (_gate)
        {
            task = _searchTask;
        }
        task?.Wait();
        lock (_gate)
        {
            _searchCts = null;
            _searchTask = null;
        }
    }

    private void Respond(string msg) => writer.WriteLine(msg);

    /// <summary>Stops any running search and releases the handler's engine.</summary>
    public void Close()
    {
        StopSearchAndWait();
        lock (_gate)
        {
            _ai.Dispose();
        }
    }

    public void Dispose() => Close();

    public static void RunUciLoop(UciHandler handler, TextReader reader)
    {
        string? line;
        while ((line = reader.ReadLine()) != null)
        {
            line = line.Trim();
            if (line.Length == 0)
            {
                continue;
            }
            handler.HandleCommand(line);
            if (line == "quit")
            {
                handler.StopSearchAndWait();
                return;
            }
        }
    }
}
