using Xunit;

// Wall-clock budget tests sit next to CPU-heavy search tests; parallel class
// execution (and coverage instrumentation) steals cycles and turns the soft
// limit assertions into coin flips. The Go suite ran serially in one process;
// mirror that here.
[assembly: CollectionBehavior(DisableTestParallelization = true)]
