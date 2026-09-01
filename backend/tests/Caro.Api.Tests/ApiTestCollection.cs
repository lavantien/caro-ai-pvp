using Xunit;

// The ponder kill-switch test flips a process-wide static; with parallel
// test classes that races every other ponder test. The Go package suite ran
// serially, so this mirrors the original execution model.
[assembly: CollectionBehavior(DisableTestParallelization = true)]
