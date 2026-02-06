using System.Diagnostics;
using System.Text;

namespace Caro.UCIMockClient;

/// <summary>
/// Manages a UCI engine process and communicates via stdin/stdout.
/// </summary>
public sealed class UCIMockClient : IDisposable
{
    private Process? _process;
    private StreamReader _stdout;
    private StreamWriter _stdin;
    private StreamReader? _stderr;
    private readonly string _exePath;
    private readonly CancellationTokenSource _cts = new();
    private readonly List<string> _moveHistory = new();
    private bool _isInitialized;
    private int _skillLevel = 3;
    private bool _openingBookEnabled = true;
    private int _bookDepthLimit = 24;

    /// <summary>
    /// Event raised when the engine sends info messages.
    /// </summary>
    public event Action<string>? OnInfo;

    /// <summary>
    /// The skill level (1-6) configured for this engine.
    /// </summary>
    public int SkillLevel => _skillLevel;

    /// <summary>
    /// Whether the opening book is enabled.
    /// </summary>
    public bool OpeningBookEnabled => _openingBookEnabled;

    /// <summary>
    /// The opening book depth limit.
    /// </summary>
    public int BookDepthLimit => _bookDepthLimit;

    /// <summary>
    /// Create a new UCI client for the specified engine executable.
    /// </summary>
    /// <param name="exePath">Path to the UCI engine executable</param>
    public UCIMockClient(string exePath)
    {
        _exePath = exePath ?? throw new ArgumentNullException(nameof(exePath));
        
        // Create dummy streams for initialization (will be replaced when engine starts)
        _stdout = new StreamReader(Stream.Null);
        _stdin = new StreamWriter(Stream.Null);
    }

    /// <summary>
    /// Start the UCI engine process.
    /// </summary>
    public void StartEngine()
    {
        // Determine if we're using a project file or exe
        bool useDotnetRun = _exePath.EndsWith(".csproj", StringComparison.OrdinalIgnoreCase);
        
        // Get the directory of the executable for DLL resolution
        var exeDir = Path.GetDirectoryName(_exePath);
        var workingDir = useDotnetRun ? exeDir : exeDir;
        
        var startInfo = new ProcessStartInfo
        {
            FileName = useDotnetRun ? "dotnet" : _exePath,
            Arguments = useDotnetRun ? $"run --project \"{_exePath}\" --" : "",
            RedirectStandardInput = true,
            RedirectStandardOutput = true,
            RedirectStandardError = false,  // Don't redirect stderr - fixes Windows DLL loading issue
            UseShellExecute = false,
            CreateNoWindow = true,
            WorkingDirectory = workingDir
        };

        // For .exe, add PATH environment variable with the exe directory and runtimes directory
        if (!useDotnetRun)
        {
            var pathEnv = Environment.GetEnvironmentVariable("PATH");
            if (!string.IsNullOrEmpty(exeDir) && !string.IsNullOrEmpty(pathEnv))
            {
                var newPath = exeDir;
                
                // Add runtimes directory to PATH if it exists
                var runtimesDir = Path.Combine(exeDir, "runtimes", "win-x64", "native");
                if (Directory.Exists(runtimesDir))
                {
                    newPath = runtimesDir + Path.PathSeparator + newPath;
                }
                
                startInfo.Environment["PATH"] = newPath + Path.PathSeparator + pathEnv;
            }
        }

        _process = Process.Start(startInfo);
        if (_process == null)
            throw new InvalidOperationException($"Failed to start engine process: {_exePath}");

        _stdin = _process.StandardInput;
        _stdout = _process.StandardOutput;
        _stderr = null;  // Not redirecting stderr anymore to fix Windows DLL loading issue
    }

    /// <summary>
    /// Initialize the UCI engine with protocol handshake.
    /// </summary>
    public async Task InitializeEngineAsync()
    {
        if (_process == null || _process.HasExited)
        {
            var errorMsg = ReadAllErrorOutput();
            throw new InvalidOperationException($"Engine process is not running. Error output: {errorMsg}");
        }

        SendCommand("uci");
        await WaitForResponseAsync("uciok", TimeSpan.FromSeconds(5));

        // Set skill level
        SendCommand($"setoption name Skill Level value {_skillLevel}");

        // Set opening book options
        SendCommand($"setoption name Use Opening Book value {_openingBookEnabled.ToString().ToLowerInvariant()}");
        SendCommand($"setoption name Book Depth Limit value {_bookDepthLimit}");

        SendCommand("ucinewgame");
        
        _isInitialized = true;
    }

    /// <summary>
    /// Set the skill level for this engine (1-6).
    /// </summary>
    public void SetSkillLevel(int level)
    {
        if (level < 1 || level > 6)
            throw new ArgumentOutOfRangeException(nameof(level), "Skill level must be between 1 and 6");

        _skillLevel = level;
        if (_isInitialized)
        {
            SendCommand($"setoption name Skill Level value {_skillLevel}");
        }
    }

    /// <summary>
    /// Configure the opening book.
    /// </summary>
    public void SetOpeningBook(bool enabled, int depthLimit = 24)
    {
        _openingBookEnabled = enabled;
        _bookDepthLimit = depthLimit;
        
        if (_isInitialized)
        {
            SendCommand($"setoption name Use Opening Book value {_openingBookEnabled.ToString().ToLowerInvariant()}");
            SendCommand($"setoption name Book Depth Limit value {_bookDepthLimit}");
        }
    }

    /// <summary>
    /// Get the current position in UCI format.
    /// </summary>
    public string GetPosition()
    {
        if (_moveHistory.Count == 0)
            return "position startpos";

        return "position startpos moves " + string.Join(" ", _moveHistory);
    }

    /// <summary>
    /// Set the position using a list of UCI moves.
    /// This overrides the internal move history for synchronization.
    /// </summary>
    public void SetPosition(IList<string> moves)
    {
        _moveHistory.Clear();
        foreach (var move in moves)
        {
            _moveHistory.Add(move);
        }
    }

    /// <summary>
    /// Add a move to the history (used by GameManager to sync both engines).
    /// </summary>
    public void AddMove(string uciMove)
    {
        _moveHistory.Add(uciMove);
    }

    /// <summary>
    /// Get a move from the engine with time control parameters.
    /// </summary>
    /// <param name="wtime">White (Red) time remaining in milliseconds</param>
    /// <param name="btime">Black (Blue) time remaining in milliseconds</param>
    /// <param name="winc">White (Red) increment in milliseconds</param>
    /// <param name="binc">Black (Blue) increment in milliseconds</param>
    /// <param name="addToHistory">Whether to add the move to this engine's history (default: true)</param>
    /// <returns>UCI move string (e.g., "j10")</returns>
    public async Task<string> GetMoveAsync(long wtime, long btime, long winc, long binc, bool addToHistory = true)
    {
        if (_process == null)
            throw new InvalidOperationException("Engine process is null");

        if (_process.HasExited)
        {
            var errorMsg = ReadAllErrorOutput();
            var exitCode = _process.ExitCode;
            throw new InvalidOperationException($"Engine process exited with code {exitCode}. Error output: {errorMsg}");
        }

        // Send position
        SendCommand(GetPosition());

        // Send go command with time controls
        SendCommand($"go wtime {wtime} btime {btime} winc {winc} binc {binc}");

        // Wait for bestmove response
        var bestMove = await WaitForBestMoveAsync(TimeSpan.FromMinutes(5));
        
        if (string.IsNullOrEmpty(bestMove))
            throw new InvalidOperationException("Engine did not return a move");

        // Add to move history if requested
        if (addToHistory)
            _moveHistory.Add(bestMove);

        return bestMove;
    }

    /// <summary>
    /// Start a search without blocking.
    /// </summary>
    public void StartSearch(long wtime, long btime, long winc, long binc)
    {
        if (_process == null || _process.HasExited)
            throw new InvalidOperationException("Engine process is not running");

        SendCommand(GetPosition());
        SendCommand($"go wtime {wtime} btime {btime} winc {winc} binc {binc}");
    }

    /// <summary>
    /// Wait for and return the best move from a running search.
    /// </summary>
    public async Task<string> WaitForBestMoveAsync(TimeSpan timeout)
    {
        var startTime = DateTime.UtcNow;
        var infoLines = new List<string>();

        using var timer = new PeriodicTimer(TimeSpan.FromMilliseconds(100));
        
        while (await timer.WaitForNextTickAsync(_cts.Token))
        {
            if (DateTime.UtcNow - startTime > timeout)
                throw new TimeoutException("Engine did not respond within timeout period");

            if (_process == null)
                throw new InvalidOperationException("Engine process became null");

            if (_process.HasExited)
            {
                var errorMsg = ReadAllErrorOutput();
                var exitCode = _process.ExitCode;
                throw new InvalidOperationException($"Engine process exited with code {exitCode}. Error output: {errorMsg}");
            }

            // Read all available lines
            while (true)
            {
                var line = _stdout.ReadLine();
                if (line == null)
                    break;

                if (string.IsNullOrWhiteSpace(line))
                    continue;

                if (line.StartsWith("info ", StringComparison.OrdinalIgnoreCase))
                {
                    infoLines.Add(line);
                    OnInfo?.Invoke(line);
                }
                else if (line.StartsWith("bestmove ", StringComparison.OrdinalIgnoreCase))
                {
                    var parts = line.Split(' ', StringSplitOptions.RemoveEmptyEntries);
                    if (parts.Length >= 2)
                        return parts[1];
                }
            }
        }

        throw new TimeoutException("Engine did not return bestmove within timeout");
    }

    /// <summary>
    /// Send a command to the engine.
    /// </summary>
    private void SendCommand(string command)
    {
        if (_stdin == null)
            throw new InvalidOperationException("Engine stdin is not available");

        _stdin.WriteLine(command);
        _stdin.Flush();
    }

    /// <summary>
    /// Wait for a specific response from the engine.
    /// </summary>
    private async Task WaitForResponseAsync(string expectedResponse, TimeSpan timeout)
    {
        var startTime = DateTime.UtcNow;
        
        while (DateTime.UtcNow - startTime < timeout)
        {
            if (_process == null)
                throw new InvalidOperationException("Engine process became null");

            if (_process.HasExited)
            {
                var errorMsg = ReadAllErrorOutput();
                var exitCode = _process.ExitCode;
                throw new InvalidOperationException($"Engine process exited with code {exitCode}. Error output: {errorMsg}");
            }

            var line = await _stdout.ReadLineAsync(_cts.Token);
            if (line == null)
                continue;

            if (line.Trim().Equals(expectedResponse, StringComparison.OrdinalIgnoreCase))
                return;

            if (line.StartsWith("info ", StringComparison.OrdinalIgnoreCase))
            {
                OnInfo?.Invoke(line);
            }
        }

        throw new TimeoutException($"Did not receive expected response '{expectedResponse}' within timeout");
    }

    /// <summary>
    /// Reset for a new game.
    /// </summary>
    public void NewGame()
    {
        _moveHistory.Clear();
        SendCommand("ucinewgame");
    }

    /// <summary>
    /// Stop the engine and cleanup resources.
    /// </summary>
    public void StopEngine()
    {
        try
        {
            SendCommand("quit");
        }
        catch
        {
            // Ignore errors during shutdown
        }

        _cts.Cancel();

        try
        {
            _process?.WaitForExit(1000);
        }
        catch
        {
            // Ignore
        }

        try
        {
            _process?.Kill();
        }
        catch
        {
            // Ignore
        }

        _process?.Dispose();
    }

    /// <summary>
    /// Check if the engine process is still running.
    /// </summary>
    public bool IsRunning => _process != null && !_process.HasExited;


    /// <summary>
    /// Read all available error output from the engine.
    /// </summary>
    private string ReadAllErrorOutput()
    {
        if (_stderr == null || _process == null || _process.HasExited)
            return "";

        try
        {
            // Read any available error output
            var errorOutput = new StringBuilder();
            while (_stderr.Peek() >= 0)
            {
                errorOutput.AppendLine(_stderr.ReadLine());
            }
            return errorOutput.ToString();
        }
        catch
        {
            return "";
        }
    }

    public void Dispose()
    {
        StopEngine();  // StopEngine uses _cts.Cancel(), so call before disposing
        
        try
        {
            _cts.Dispose();
        }
        catch { }
        
        try
        {
            _stdin?.Dispose();
        }
        catch { }
        
        try
        {
            _stdout?.Dispose();
        }
        catch { }
    }
}
