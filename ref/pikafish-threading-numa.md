# pikafish threading and numa (zread summary)

source: https://zread.ai/official-pikafish/Pikafish/17-multi-threading-and-numa (generated from src/thread.cpp, src/numa.h, src/shm.h)
fetched: 2026-10-18 via WebFetch

## summary as fetched

Three thread layers: NativeThread (pthread_create or std::thread; POSIX threads get 8 MB stacks because macOS defaults to 512 KB, too small for deep recursive search; failed creation deletes the callable to avoid leaks; type erasure via ThreadCallableBase), Thread (wraps NativeThread in an idle_loop condition-variable wait/work cycle; jobs arrive via run_custom_job which waits for !searching, installs the job, sets searching, notifies; shutdown sets exit and wakes for join; each thread's worker is a LargePagePtr<Search::Worker> on 2 MB huge pages to cut TLB misses, allocated inside a job on the thread itself so memory lands on the right NUMA node), ThreadPool (owns threads; set(), start_thinking(), get_best_thread(), nodes_searched() atomic accumulation).

Best-thread voting: each thread votes for its root move with weight proportional to (score - minScore + 14), minScore the worst score among threads; highest total wins; decisive results (mates/TB conversions) take priority with the shortest mate preferred.

NUMA: NumaConfig immutable, maps CPU to NUMA indices; discovery via system nodes, L3-cache domains, or bundling small L3 domains; Linux sysfs, Windows GetLogicalProcessorInformationEx + GetNumaProcessorNodeEx; Windows always splits at processor-group boundaries (groups cap at 64 CPUs). Thread distribution by lowest fill ratio (occupation[n] + 1) / nodes[n].size(). UCI NumaPolicy auto/system/none; auto binds only if thresholds met (custom affinity, threads > half the largest node, or >= 4x non-small nodes); a single thread is never bound under auto. Linux sched_setaffinity up to 65,536 CPUs plus sched_yield; Windows SetThreadSelectedCpuSetMasks (Win 11+) falling back to SetThreadGroupAffinity. execute_on_numa_node spawns a temporary bound thread, creates the object there, joins: the search thread inherits NUMA affinity at creation, Worker and SharedHistory memory node-local.

Memory replication: NumaReplicated<T> eager copies to every node; LazyNumaReplicated<T> node 0 up front, others on first access with double-checked locking; LazyNumaReplicatedSystemWide<T> wraps SystemWideSharedConstant<T> in shared memory, deduplicating NNUE weights across multiple Pikafish processes (discriminator from topology string plus node index). SharedHistories per NUMA node, each table sized to the next power of two of that node's thread count. Config changes re-replicate from node 0.

Lazy SMP cooperation and the shared TT are covered on separate pages; ponder behavior is not described on this page.

## extraction notes

- Best-thread voting with weight (score - minScore + 14) plus shortest-mate priority: a concrete answer to lazy-SMP's divergent-root problem. Our SMP engine currently takes main-thread root; voting is the upgrade path.
- System-wide shared NNUE weights across processes: for us, multiple engine instances in one tournament would share one embedded book artifact already (go:embed, read-only); the analog would be shared mmap TTs, which we do not do and do not need at current sizes.
- Huge-page worker allocation and node-local allocation-at-creation are Windows-applicable ideas if we ever chase tail latency; not current priorities.
