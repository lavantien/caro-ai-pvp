using Caro.Uci;

ConsoleLineWriter writer = new();
using UciHandler handler = new(writer);
UciHandler.RunUciLoop(handler, Console.In);

internal sealed class ConsoleLineWriter : ILineWriter
{
    private readonly object _gate = new();

    public void WriteLine(string line)
    {
        lock (_gate)
        {
            Console.Out.WriteLine(line);
        }
    }
}
