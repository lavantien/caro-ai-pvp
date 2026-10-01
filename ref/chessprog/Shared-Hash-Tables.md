source: https://chessprogramming.org/Shared_Hash_Table

# Shared Hash Table

Home * Programming * Data * Hash Table * Shared Hash Table

Dining philosophers problem 1

---

1. [Dining philosophers problem from Wikipedia](https://en.wikipedia.org/wiki/Dining_philosophers_problem)↩︎

Shared Hash Table,
 a Hash table or Transposition table which is accessed by various processes or threads simultaneously, running on [multiple processors](https://en.wikipedia.org/wiki/Multiprocessing) or [processor cores](https://en.wikipedia.org/wiki/Multi-core_processor). Shared hash tables are most often implemented as dynamically allocated memory treated as global array. Due to [memory protection](https://en.wikipedia.org/wiki/Memory_protection) between processes, they require an [Application programming interface](https://en.wikipedia.org/wiki/Application_programming_interface) provided by the [operating system](https://en.wikipedia.org/wiki/Operating_system) to allocate [shared memory](https://en.wikipedia.org/wiki/Shared_memory). Threads may share global memory from the process they are belonging to.

# Parallel Search

Almost all parallel search algorithms on SMP- or NUMA systems profit from probing hash entries written by other instances of the search, in its most simple form by instances of a sequential search algorithm which simultaneously search the same root position. The gains come from the effect of nondeterminism. Each processor will finish the various subtrees in varying amounts of time, and as the search continues, these effects grow making the search trees diverge. The [speedup](https://en.wikipedia.org/wiki/Speedup) is then based on how many nodes the main processor is able to skip from transposition table entries. It had the reputation of little speedup on a mere 2 processors, and to scale quite badly after this. However, the NPS scaling is nearly perfect.

## ABDADA

see Main page: ABDADA

ABDADA, Alpha-Bêta Distribué avec Droit d'Anesse (Distributed Alpha-Beta Search with Eldest Son Right) is a loosely synchronized, distributed search algorithm by Jean-Christophe Weill 1 . It is based on the Shared Hash Table, and adds the number of processors searching this node inside the hash-table entry for better utilization - considering the Young Brothers Wait Concept.

## Lazy SMP

see Main page: Lazy SMP

Recent improvements by Daniel Homan 2, Martin Sedlak 3 and others on Lazy SMP indicate that the algorithm scales quite well up to 8 cores and beyond 4.

# Concurrent Access

Due to its size, i.e. 16 or more bytes, writing and reading hash entries are none [atomic](https://en.wikipedia.org/wiki/Linearizability) and require multiple write- and read-cycles. It may and will happen that [concurrent](https://en.wikipedia.org/wiki/Concurrency_%28computer_science%29) writes and reads at the same table address and almost same time results in corrupt data retrieved that causes significant problems to the search. [Interrupts](https://en.wikipedia.org/wiki/Interrupt) may occur between accesses, and there are further nondeterministic issues involved 5 which may cause one thread to read two or more atomic data items, which were written by different threads, searching different positions with the same hash-index due to type-2 errors.

## Locks

One common solution to avoid such errors is [synchronization](https://en.wikipedia.org/wiki/Synchronization_%28computer_science%29) using [atomic locks](https://en.wikipedia.org/wiki/Lock_%28computer_science%29), and to implement a [critical section](https://en.wikipedia.org/wiki/Critical_section) or [mutual exclusion](https://en.wikipedia.org/wiki/Mutual_exclusion).

### CilkChess

As an example, CilkChess used Cilk's support for atomicity 6. It uses one lock per hash entry:

```
typedef struct
{
   Cilk_lockvar lock;
   U64 key;
   U64 data;
} ttentry;

ttentry hashtable[TABLESIZE];

void ttinit ( ) {
   for (int i = 0; i < TABLESIZE; ++i)
      Cilk_lock_init( hashtable[i].lock);
}

void update_entry ( ttentry *e, U64 key, U64 data ) {
   Cilk_lock (e->lock); /* begin critical section */
   e->key = key;
   e->data = data;
   ...
   Cilk_unlock (e->lock); /* end critical section */
}
```

### Granularity

An important property of a lock is its [granularity](https://en.wikipedia.org/wiki/Lock_%28computer_science%29#Granularity), which is a measure of the amount of data the lock is protecting. In general, choosing a coarse granularity (a small number of locks, each protecting a large segment of data) results in less lock overhead when a single process is accessing the protected data, but worse performance when multiple processes are running concurrently. This is because of increased lock contention. The more coarse the lock, the higher the likelihood that the lock will stop an unrelated process from proceeding, i.e. in the extreme case, one lock for the whole table. Conversely, using a fine granularity (a larger number of locks, each protecting a fairly small amount of data), like in the CilkChess sample above, increases the overhead of the locks themselves but reduces lock contention. For a huge transposition table with millions of fairly small entries locks incur a significant performance penalty on many architectures.

## Lock-less

### Xor

Robert Hyatt and Tim Mann proposed a lock-less transposition table implementation 7 for 128 bit entries with two atomic quad words, one qword for storing the key or signature, the 64-bit Zobrist- or BCH-key of the position, and one qword for the other information stored, move, score, draft and that like (data). Rather than to store two disjoint items, the key is stored xored with data, while data is stored additionally as usual. According to Robert Hyatt, the original idea came from Harry Nelson somewhere in 1990-1992 8.

```
index = key % TABLESIZE;
hashtable[index].key  = key ^ data;
hashtable[index].data = data;
```

Since the retrieving position requires the same key for a probing hit, the stored key xored by the retrieved key must match the stored data.

```
index = key % TABLESIZE;
if (( hashtable[index].key ^ hashtable[index].data) == key )
{
   /* entry matches key */
}
```

If key and data were written simultaneously by different search instances with different keys, the error will usually yield in a mismatch of the comparison, except the rare but inherent Key collisions or type-1 errors 9. As pointed out by Harm Geert Muller 10, the XOR technique might be applied for any size.

### Checksum

For a lock-less shared Hash table with (much) larger entry sizes such as the Pawn Hash Table, one may store an additional [checksum](https://en.wikipedia.org/wiki/Checksum) of the data, to likely detect errors after retrieving, and to safe the consistence of an entry.

### SSE2

x86 and x86-64 SSE2 128-bit read/write instructions might in practice [atomic](https://en.wikipedia.org/wiki/Linearizability), but they are not guaranteed even if properly aligned 11 12. If the processor implements a 16-byte store instruction internally as 2 8-byte stores in the store pipeline, it's perfectly possible for another processor to "steal" the cache line in between the two stores 13. However, Intel states any locked instruction (either the XCHG instruction or another read-modify-write instruction with a [LOCK prefix](https://en.wikipedia.org/wiki/Fetch-and-add#x86_implementation)) appears to execute as an indivisible and uninterruptible sequence of load(s) followed by store(s) regardless of alignment 14 15.

# Allocation

Multiple threads inside one process can share its [global variables](https://en.wikipedia.org/wiki/Global_variable) or heap. Processes require special [API](https://en.wikipedia.org/wiki/Application_programming_interface) calls to create shared memory and to pass a handle to other processes around for [interprocess communication](https://en.wikipedia.org/wiki/Inter-process_communication). [POSIX](https://en.wikipedia.org/wiki/POSIX) provides a standardized API for using shared memory 16 . Linux kernel builds since 2.6 offer /dev/shm as shared memory in the form of a [RAM disk](https://en.wikipedia.org/wiki/RAM_disk).

# See also

- ABDADA
- AVX
- Cilk
- Lazy SMP
- Linux
- Memory
- Parallel Search
- SSE2
- Transposition Table
- Unix
- Windows
- XOP

# Publications

17

## 1980 ...

- Clyde Kruskal, Larry Rudolph, Marc Snir (1988). [Efficient Synchronization on Multiprocessors with Shared Memory](https://dl.acm.org/citation.cfm?id=48024). ACM TOPLAS, Vol. 10, No. 4
- Henri Bal (1989). [The shared data-object model as a paradigm for programming distributed systems](http://dare.ubvu.vu.nl/handle/1871/12760?mode=full&submit_simple=Show+full+item+record). Ph.D. thesis, [Vrije Universiteit](https://en.wikipedia.org/wiki/Vrije_Universiteit)

## 1990 ...

- [Maurice Herlihy](http://www.cs.brown.edu/~mph/) (1991). Wait-free synchronization. [ACM Transactions on Programming Languages and Systems](https://en.wikipedia.org/wiki/ACM_Transactions_on_Programming_Languages_and_Systems) Vol. 13 No. 1, [pdf](http://www.cs.brown.edu/~mph/Herlihy91/p124-herlihy.pdf)
- Vincent David (1993). [Algorithmique parallèle sur les arbres de décision et raisonnement en temps contraint. Etude et application au Minimax](http://cat.inist.fr/?aModele=afficheN&cpsidt=161774) = Parallel algorithm for heuristic tree searching and real-time reasoning. Study and application to the Minimax, Ph.D. Thesis, [École nationale supérieure de l'aéronautique et de l'espace](https://en.wikipedia.org/wiki/%C3%89cole_nationale_sup%C3%A9rieure_de_l%27a%C3%A9ronautique_et_de_l%27espace), [Toulouse](https://en.wikipedia.org/wiki/Toulouse), [France](https://en.wikipedia.org/wiki/France)

Abstract: The method of parallelization is based on a suppression of control between the search processes, in favor of a speculative parallelism and full sharing of information achieved through a physically distributed but virtually shared memory. The contribution of our approach for real-time distributed systems and fault-tolerant is evaluated through experimental results.

- [Maged M. Michael](http://www.research.ibm.com/people/m/michael/), [Michael L. Scott](http://www.cs.rochester.edu/%7Escott/) (1995). Implementation of Atomic Primitives on Distributed Shared Memory Multiprocessors. [HPCA'95](http://www-2.cs.cmu.edu/%7Escandal/conf/pro-HPCA-950122.txt), [pdf](http://www.research.ibm.com/people/m/michael/hpca-1995.pdf)
- Paul Lu (1997). Aurora: Scoped Behaviour for Per-Context Optimized Distributed Data Sharing. 11th International Parallel Processing Symposium (IPPS) 18
- John Romein, Aske Plaat, Henri Bal, Jonathan Schaeffer (1999). Transposition Table Driven Work Scheduling in Distributed Search. AAAI-99, [pdf](https://www.aaai.org/Papers/AAAI/1999/AAAI99-103.pdf) 19 20

## 2000 ...

- Paul Lu (2000). [Scoped Behaviour for Optimized Distributed Data Sharing](http://webdocs.cs.ualberta.ca/~paullu/PhDThesis/thesis.html). Ph.D. thesis, University of Toronto
- Valavan Manohararajah (2001) Parallel Alpha-Beta Search on Shared Memory Multiprocessors. Masters Thesis, [pdf](http://www.valavan.net/mthesis.pdf)
- John Romein, Henri Bal, Jonathan Schaeffer, Aske Plaat (2002). A Performance Analysis of Transposition-Table-Driven Scheduling in Distributed Search. IEEE Transactions on Parallel and Distributed Systems, Vol. 13, No. 5, pp. 447–459. [pdf](http://www.cs.vu.nl/~bal/Papers/tds.pdf) 21
- [Maged M. Michael](http://www.research.ibm.com/people/m/michael/) (2002). High Performance Dynamic Lock-Free Hash Tables and List-Based Sets. [IBM Thomas J. Watson Research Center](https://en.wikipedia.org/wiki/Thomas_J._Watson_Research_Center), [pdf](http://www.research.ibm.com/people/m/michael/spaa-2002.pdf)
- Robert Hyatt, Tim Mann (2002). [A lock-less transposition table implementation for parallel search chess engines](http://www.craftychess.com/hyatt/hashing.html). ICGA Journal, Vol. 25, No. 1
- [Jiří Barnat](http://www.fi.muni.cz/%7Exbarnat/), [Petr Ročkai](http://www.behindkde.org/node/625) (2007). Shared Hash Tables in Parallel Model Checking. Faculty of Informatics, [Masaryk University](https://en.wikipedia.org/wiki/Masaryk_University), [Brno](https://en.wikipedia.org/wiki/Brno), [Czech Republic](https://en.wikipedia.org/wiki/Czech_Republic), PDMC 2007, Preliminary Version as [pdf](http://anna.fi.muni.cz/papers/src/public/168699cd5787307ee3c8c1a509327e6f.pdf)
- [Vivek Sarkar](http://www.cs.rice.edu/%7Evs3/home/Vivek_Sarkar.html) (2008). Shared-Memory Parallel Programming with OpenMP. [Rice University](https://en.wikipedia.org/wiki/Rice_University), [slides as pdf](http://www.cs.rice.edu/%7Evs3/comp422/lecture-notes/comp422-lec7-s08-v1.pdf)
- [Jouni Leppäjärvi](http://offcode.fi/) (2008). A pragmatic, historically oriented survey on the universality of synchronization primitives. [pdf](http://www.oamk.fi/~joleppaj/personal/jleppaja_gradu_080511.pdf)

## 2010 ...

- [Alfons Laarman](http://www.vf.utwente.nl/%7Elaarman/), [Jaco van de Pol](http://wwwhome.ewi.utwente.nl/%7Evdpol/), [Michael Weber](http://wwwhome.cs.utwente.nl/%7Emichaelw/) (2010). [Boosting Multi-Core Reachability Performance with Shared Hash Tables](http://doc.utwente.nl/73119/). Formal Methods and Tools, [University of Twente](https://en.wikipedia.org/wiki/University_of_Twente), [The Netherlands](https://en.wikipedia.org/wiki/Netherlands), [pdf](http://fmcad10.iaik.tugraz.at/Papers/papers/12Session11/033Laarman.pdf)
- [John Mellor-Crummey](http://www.cs.rice.edu/%7Ejohnmc/) (2011). Shared-memory Parallel Programming with Cilk. [Rice University](https://en.wikipedia.org/wiki/Rice_University), [slides as pdf](http://www.clear.rice.edu/comp422/lecture-notes/comp422-2011-Lecture4-Cilk.pdf) » Cilk
- [Anthony Williams](http://stackoverflow.com/users/5597/anthony-williams) (2012). [C++ Concurrency in Action: Practical Multithreading](http://www.cplusplusconcurrencyinaction.com/). 22
- [Tobias Maier](https://dblp.uni-trier.de/pers/hd/m/Maier:Tobias), Peter Sanders, [Roman Dementiev](https://dblp.uni-trier.de/pers/hd/d/Dementiev:Roman) (2016). Concurrent Hash Tables: Fast and General?(!). [arXiv:1601.04017](https://arxiv.org/abs/1601.04017)

# Forum Posts

## 1997 ...

- [Parallel searching](https://groups.google.com/d/msg/rec.games.chess.computer/Wl7A-v-gWYQ/QLuvAp0l4_gJ) by Andrew Tridgell, rgcc, March 22, 1997 » KnightCap
- [CilkChess question for Don](https://www.stmintz.com/ccc/index.php?id=41708) by Robert Hyatt, CCC, January 31, 1999 » CilkChess

## 2000 ...

- [Re: Atomic write of 64 bits](https://groups.google.com/group/comp.lang.asm.x86/browse_frm/thread/ab55c5d57a3a1fd1) by Frans Morsch, [comp.lang.asm.x86](https://groups.google.com/group/comp.lang.asm.x86/topics), September 25, 2000

## 2005 ...

- [multithreading questions](http://www.talkchess.com/forum/viewtopic.php?t=15662) by Martin Fierz, CCC, August 08, 2007
- [If making an SMP engine, do NOT use processes](http://www.talkchess.com/forum/viewtopic.php?t=19446) by Zach Wegner, CCC, February 07, 2008
- [threads vs processes](http://www.talkchess.com/forum/viewtopic.php?t=22398) by Robert Hyatt, CCC, July 16, 2008
- [threads vs processes again](http://www.talkchess.com/forum/viewtopic.php?t=22799) by Robert Hyatt, CCC, August 05, 2008
- [SMP hashing problem](http://www.talkchess.com/forum/viewtopic.php?t=26208) by Robert Hyatt, CCC, January 24, 2009
- [Interlock clusters](http://www.talkchess.com/forum/viewtopic.php?t=26223) by Steven Edwards, CCC, January 25, 2009

## 2010 ...

- [Crafty Transpostion Table Question](http://www.talkchess.com/forum3/viewtopic.php?f=7&t=34606) by Eric Stock, CCC, May 30, 2010 » Crafty, Lockless Hashing
- [lockless hashing](http://www.talkchess.com/forum/viewtopic.php?t=37976) by Daniel Shawul, CCC, February 07, 2011
- [On parallelization](http://www.talkchess.com/forum/viewtopic.php?t=38411) by Onno Garms, CCC, March 13, 2011
- [cache alignment of tt](http://www.talkchess.com/forum/viewtopic.php?t=42833) by Daniel Shawul, CCC, March 11, 2012
- [Speaking of the hash table](http://www.talkchess.com/forum/viewtopic.php?t=46346) by Ed Schroder, CCC, December 09, 2012
- [Lazy SMP](http://www.talkchess.com/forum/viewtopic.php?t=46597) by Julien Marcel, CCC, December 27, 2012 » Lazy SMP
- [Lazy SMP, part 2](http://www.talkchess.com/forum/viewtopic.php?t=46858) by Daniel Homan, CCC, January 12, 2013
- [Multi-threaded memory access](http://www.open-chess.org/viewtopic.php?f=5&t=2262) by ThinkingALot, OpenChess Forum, February 10, 2013 » Memory, Thread
- [Lazy SMP, part 3](http://www.talkchess.com/forum/viewtopic.php?t=47455) by Daniel Homan, CCC, March 09, 2013
- [Shared hash table smp result](http://www.talkchess.com/forum/viewtopic.php?t=47568) by Daniel Shawul, CCC, March 21, 2013
- [Transposition driven scheduling](http://www.talkchess.com/forum/viewtopic.php?t=47700) by Daniel Shawul, CCC, April 04, 2013 23
- [Lazy SMP and Work Sharing](http://www.talkchess.com/forum/viewtopic.php?t=48536) by Daniel Homan, CCC, July 03, 2013 » Lazy SMP in EXchess
- [Lockless hash: Thank you Bob Hyatt!](http://www.talkchess.com/forum/viewtopic.php?t=49099) by Julien Marcel, CCC, August 25, 2013
- [How could a compiler break the lockless hashing method?](http://www.talkchess.com/forum/viewtopic.php?t=50388) by Rein Halbersma, CCC, December 08, 2013
- [Parallel Search with Transposition Table](http://www.talkchess.com/forum/viewtopic.php?t=51755) by Daylen Yang, CCC, March 27, 2014 » Parallel Search
- [Two hash functions for distributed transposition table](http://www.talkchess.com/forum/viewtopic.php?t=54666) by Daniel Shawul, CCC, December 16, 2014

## 2015 ...

- [Lazy SMP in Cheng](http://www.talkchess.com/forum/viewtopic.php?t=55188) by Martin Sedlak, CCC, February 02, 2015 » Cheng
- [Trying to improve lazy smp](http://www.talkchess.com/forum/viewtopic.php?t=55970) by Daniel José Queraltó, CCC, April 11, 2015
- [lazy smp questions](http://www.talkchess.com/forum/viewtopic.php?t=57572) by Lucas Braesch, CCC, September 09, 2015 » Lazy SMP
- [atomic TT](http://www.talkchess.com/forum/viewtopic.php?t=57634) by Lucas Braesch, CCC, September 13, 2015
- [lazy smp questions](http://www.talkchess.com/forum/viewtopic.php?t=58645) by Marco Belli, CCC, December 21, 2015 » Lazy SMP

2016

- [NUMA 101](http://www.talkchess.com/forum/viewtopic.php?t=58830) by Robert Hyatt, CCC, January 07, 2016 » NUMA
- [Lazy SMP - how it works](http://www.talkchess.com/forum/viewtopic.php?t=59389) by Kalyankumar Ramaseshan, CCC, February 29, 2016 » Lazy SMP
- [lockless hashing](http://www.talkchess.com/forum/viewtopic.php?t=60122) by Lucas Braesch, CCC, May 10, 2016
- [What do you do with NUMA?](http://www.talkchess.com/forum/viewtopic.php?t=61472) by Matthew Lai, CCC, September 19, 2016 » NUMA

2017 ...

- [Question about parallel search and race conditions](http://www.talkchess.com/forum/viewtopic.php?t=65134) by Michael Sherwin, CCC, September 11, 2017 » Parallel Search
- [Prefetch and Threading](http://www.talkchess.com/forum3/viewtopic.php?f=7&t=70586) by Dennis Sceviour, CCC, April 25, 2019 » Thread, Transposition Table
- [RMO - Randomized Move Order - yet another Lazy SMP derivate](http://www.talkchess.com/forum3/viewtopic.php?f=7&t=72684) by Srdja Matovic, CCC, December 30, 2019

## 2020 ...

- [hash collisions](http://www.talkchess.com/forum3/viewtopic.php?f=7&t=72932) by Jon Dart, CCC, January 28, 2020 » Key Collisions
- [Transposition table and multithreaded search](http://www.talkchess.com/forum3/viewtopic.php?f=7&t=76483) by Niels Abildskov, CCC, February 03, 2021

# External Links

## Shared Memory

Shared Memory:

- [Shared memory from Wikipedia](https://en.wikipedia.org/wiki/Shared_memory)
- [Memory model from Wikipedia](https://en.wikipedia.org/wiki/Memory_model_%28computing%29)
- [Information on the C++11 Memory Model](http://scottmeyers.blogspot.co.uk/2012/04/information-on-c11-memory-model.html) by [Scott Meyers](https://en.wikipedia.org/wiki/Scott_Meyers), April 24, 2012
- [Volatile variable from Wikipedia](https://en.wikipedia.org/wiki/Volatile_variable)
- [Memory ordering from Wikipedia](https://en.wikipedia.org/wiki/Memory_ordering)
- [Memory Ordering in Modern Microprocessors, Part I](http://www.linuxjournal.com/article/8211) by [Paul E. McKenney](https://plus.google.com/113202287320302059445/about), [Linux Journal](https://en.wikipedia.org/wiki/Linux_Journal), June 30, 2005
- [Memory barrier from Wikipedia](https://en.wikipedia.org/wiki/Memory_barrier)
- [Parallel Random Access Machine from Wikipedia](https://en.wikipedia.org/wiki/Parallel_Random_Access_Machine)
- [The Shared Memory Library (SharedMemoryLib) FAQ](http://www.inf.pucrs.br/%7Epinho/shared_memory_library.htm) by [Márcio Serolli Pinho](http://www.inf.pucrs.br/%7Epinho/)
- [Transactional memory from Wikipedia](https://en.wikipedia.org/wiki/Transactional_memory)
- [Software transactional memory from Wikipedia](https://en.wikipedia.org/wiki/Software_transactional_memory)
- [Cache coherence from Wikipedia](https://en.wikipedia.org/wiki/Cache_coherence)

[False sharing from Wikipedia](https://en.wikipedia.org/wiki/False_sharing)

- [Distributed shared memory from Wikipedia](https://en.wikipedia.org/wiki/Distributed_shared_memory)
- [Memory-mapped file from Wikipedia](https://en.wikipedia.org/wiki/Memory-mapped_file)
- [Memory disambiguation from Wikipedia](https://en.wikipedia.org/wiki/Memory_disambiguation)
- [Memory dependence prediction from Wikipedia](https://en.wikipedia.org/wiki/Memory_dependence_prediction)
- [OpenMP from Wikipedia](https://en.wikipedia.org/wiki/OpenMP)
- [POSIX from Wikipedia](https://en.wikipedia.org/wiki/POSIX)
- [shm_open](http://pubs.opengroup.org/onlinepubs/007908799/xsh/shm_open.html), [The Single UNIX Specification version 2](https://en.wikipedia.org/wiki/Single_UNIX_Specification#1997:_Single_UNIX_Specification_version_2), Copyright © 1997 [The Open Group](https://en.wikipedia.org/wiki/The_Open_Group)
- [shmget(2): allocates shared memory segment - Linux man page](http://linux.die.net/man/2/shmget) » Linux
- [mm(3): Shared Memory Allocation - Linux man page](http://www.pkill.info/linux/man/3-mm/)
- [CreateSharedMemory](http://msdn.microsoft.com/en-us/library/aa374778.aspx), [MSDN](https://en.wikipedia.org/wiki/Microsoft_Developer_Network) » Windows
- [Chapter 9. Boost.Interprocess - Boost 1.36.0](http://www.boost.org/doc/libs/1_36_0/doc/html/interprocess.html) by Ion Gaztañaga
- [Threads and memory model for C++](http://hboehm.info/c++mm/) by [Hans J. Boehm](http://www.hpl.hp.com/personal/Hans_Boehm/)
- [IPC:Shared Memory](http://www.cs.cf.ac.uk/Dave/C/node27.html) by [Dave Marshall](http://www.cs.cf.ac.uk/Dave/), 1999
- [Symmetric Multi-Processing (SMP) from Wikipedia](https://en.wikipedia.org/wiki/Symmetric_multiprocessing) » SMP
- [Asymmetric multiprocessing from Wikipedia](https://en.wikipedia.org/wiki/Asymmetric_multiprocessing)
- [Uniform Memory Access from Wikipedia](https://en.wikipedia.org/wiki/Uniform_Memory_Access)
- [Non-Uniform Memory Access (NUMA) from Wikipedia](https://en.wikipedia.org/wiki/Non-Uniform_Memory_Access) » NUMA
- [Optimizing Applications for NUMA | Intel® Developer Zone](http://software.intel.com/en-us/articles/optimizing-applications-for-numa)
- [Performance Guidelines for AMD Athlon™ 64 and AMD Opteron™ ccNUMA Multiprocessor Systems](https://doc.xdevs.com/doc/AMD/_Performance/Performance%20Guidelines%20for%20AMD%20Athlon%2064%20and%20AMD%20Opteron%20ccNUMA%20Multiprocessor%20Systems.%20rev.3.00%5D.%5B2006-06%5D.pdf) (pdf)

## Cache

Cache:

- [Cache from Wikipedia](https://en.wikipedia.org/wiki/Cache)
- [Cache (computing) from Wikipedia](https://en.wikipedia.org/wiki/Cache_(computing))
- [Functional Principles of Cache Memory](http://alasir.com/articles/cache_principles/) by [Paul V. Bolotoff](http://alasir.com/articles/), April 2007
- [CPU cache from Wikipedia](https://en.wikipedia.org/wiki/CPU_cache)
- [Cache-only memory architecture (COMA) from Wikipedia](https://en.wikipedia.org/wiki/Cache-only_memory_architecture)
- [Cache coherence from Wikipedia](https://en.wikipedia.org/wiki/Cache_coherence)

[MSI protocol from Wikipedia](https://en.wikipedia.org/wiki/MSI_protocol)

[MESI protocol from Wikipedia](https://en.wikipedia.org/wiki/MESI_protocol)

[MOESI protocol from Wikipedia](https://en.wikipedia.org/wiki/MOESI_protocol)

- [False sharing from Wikipedia](https://en.wikipedia.org/wiki/False_sharing)
- [Cache coloring from Wikipedia](https://en.wikipedia.org/wiki/Cache_coloring)
- [Cache hierarchy from Wikipedia](https://en.wikipedia.org/wiki/Cache_hierarchy)
- [Cache-oblivious algorithm from Wikipedia](https://en.wikipedia.org/wiki/Cache-oblivious_algorithm)
- [Cache pollution from Wikipedia](https://en.wikipedia.org/wiki/Cache_pollution)
- [Cache prefetching from Wikipedia](https://en.wikipedia.org/wiki/Cache_prefetching)
- [Prefetching from Wikipedia](https://en.wikipedia.org/wiki/Prefetching)

[assembly - The prefetch instruction - Stack Overflow](http://stackoverflow.com/questions/3122915/the-prefetch-instruction)

[Data Prefetch Support - GNU Project - Free Software Foundation (FSF)](http://gcc.gnu.org/projects/prefetch.html)

[Software prefetching considered harmful](http://lwn.net/Articles/444344/) by [Linus Torvalds](https://en.wikipedia.org/wiki/Linus_Torvalds), [LWN.net](https://en.wikipedia.org/wiki/LWN.net), May 19, 2011

- [Cache replacement policies from Wikipedia](https://en.wikipedia.org/wiki/Cache_replacement_policies)
- [Page cache from Wikipedia](https://en.wikipedia.org/wiki/Page_cache)
- [Acumem SlowSpotter from Wikipedia](https://en.wikipedia.org/wiki/Acumem_SlowSpotter)
- [Analyzing Efficiency of Shared and Dedicated L2 Cache in Modern Dual-Core Processors](http://ixbtlabs.com/articles2/cpu/rmmt-l2-cache.html) from [iXBT Labs - Computer Hardware In Detail](http://ixbtlabs.com/)
- [Scratchpad memory from Wikipedia](https://en.wikipedia.org/wiki/Scratchpad_memory)

## Concurrency and Synchronization

Synchronization:

- Edsger W. Dijkstra [Archive](http://www.cs.utexas.edu/users/EWD/):

[Cooperating sequential processes (EWD 123)](http://www.cs.utexas.edu/users/EWD/transcriptions/EWD01xx/EWD123.html)

[A challenge to memory designers? (EWD 497)](http://www.cs.utexas.edu/users/EWD/transcriptions/EWD04xx/EWD497.html)

- [I remember Edsger Dijkstra (1930 – 2002) « A Programmers Place](http://vanemden.wordpress.com/2008/05/06/i-remember-edsger-dijkstra-1930-2002/) by Maarten van Emden
- [Concurrency from Wikipedia](https://en.wikipedia.org/wiki/Concurrency_%28computer_science%29)
- [Category:Concurrency from Wikipedia](https://en.wikipedia.org/wiki/Category:Concurrency)
- [Concurrency control from Wikipedia](https://en.wikipedia.org/wiki/Concurrency_control)
- [Optimistic concurrency control from Wikipedia](https://en.wikipedia.org/wiki/Optimistic_concurrency_control)
- [Synchronization from Wikipedia](https://en.wikipedia.org/wiki/Synchronization_%28computer_science%29)
- [Inter-process communication from Wikipedia](https://en.wikipedia.org/wiki/Inter-process_communication)
- [Non-blocking algorithm from Wikipedia](https://en.wikipedia.org/wiki/Non-blocking_algorithm)
- [Linearizability from Wikipedia](https://en.wikipedia.org/wiki/Linearizability)
- [Monitor (synchronization)](https://en.wikipedia.org/wiki/Monitor_%28synchronization%29)
- [Lock from Wikipedia](https://en.wikipedia.org/wiki/Lock_%28computer_science%29)
- [Busy waiting from Wikipedia](https://en.wikipedia.org/wiki/Busy_waiting)
- [Seqlock from Wikipedia](https://en.wikipedia.org/wiki/Seqlock)
- [Spinlock from Wikipedia](https://en.wikipedia.org/wiki/Spinlock)
- [Double-checked locking from Wikipedia](https://en.wikipedia.org/wiki/Double-checked_locking)
- [Compare-and-swap from Wikipedia](https://en.wikipedia.org/wiki/Compare-and-swap)
- [Test-and-set from Wikipedia](https://en.wikipedia.org/wiki/Test-and-set)
- [Test and Test-and-set from Wikipedia](https://en.wikipedia.org/wiki/Test_and_Test-and-set)
- [Fetch-and-add from Wikipedia](https://en.wikipedia.org/wiki/Fetch-and-add)
- [Barrier from Wikipedia](https://en.wikipedia.org/wiki/Barrier_%28computer_science%29)
- [Memory barrier from Wikipedia](https://en.wikipedia.org/wiki/Memory_barrier)
- [Critical section from Wikipedia](https://en.wikipedia.org/wiki/Critical_section)
- [Mutual exclusion from Wikipedia](https://en.wikipedia.org/wiki/Mutual_exclusion)
- [Semaphore from Wikipedia](https://en.wikipedia.org/wiki/Semaphore_%28programming%29)
- [Transactional Synchronization Extensions from Wikipedia](https://en.wikipedia.org/wiki/Transactional_Synchronization_Extensions) ([Haswell](https://en.wikipedia.org/wiki/Haswell_%28microarchitecture%29))
- [Readers-writers problem from Wikipedia](https://en.wikipedia.org/wiki/Readers-writers_problem)
- [Readers-writer lock from Wikipedia](https://en.wikipedia.org/wiki/Readers-writer_lock)
- [Read-copy-update from Wikipedia](https://en.wikipedia.org/wiki/Read-copy-update)
- [Producer-consumer problem from Wikipedia](https://en.wikipedia.org/wiki/Producers-consumers_problem)
- [Dining philosophers problem from Wikipedia](https://en.wikipedia.org/wiki/Dining_philosophers_problem)
- [Cigarette smokers problem from Wikipedia](https://en.wikipedia.org/wiki/Cigarette_smokers_problem)
- [Sleeping barber problem from Wikipedia](https://en.wikipedia.org/wiki/Sleeping_barber_problem)
- [Resource starvation from Wikipedia](https://en.wikipedia.org/wiki/Resource_starvation)
- [Deadlock from Wikipedia](https://en.wikipedia.org/wiki/Deadlock)
- [Anatomy of Linux synchronization methods](http://www.ibm.com/developerworks/linux/library/l-linux-synchronization.html) by [M. Tim Jones](http://www.mtjones.com/), [IBM developerWorks](http://www.ibm.com/developerworks/), October 31, 2007
- [The Little Book of Semaphores](http://greenteapress.com/semaphores/) by [Allen B. Downey](http://allendowney.com/)

## Distributed memory

- [Distributed memory from Wikipedia](https://en.wikipedia.org/wiki/Distributed_memory)
- [Distributed hash table from Wikipedia](https://en.wikipedia.org/wiki/Distributed_hash_table)

[Koorde](https://en.wikipedia.org/wiki/Koorde) based on [Chord](https://en.wikipedia.org/wiki/Chord_%28peer-to-peer%29) and De Bruijn Sequence

- [Transposition-driven scheduling - Wikipedia](https://en.wikipedia.org/wiki/Transposition-driven_scheduling)
- [The Aurora Distributed Shared Data System](http://webdocs.cs.ualberta.ca/~paullu/Aurora/aurora.html) by Paul Lu

## Misc

- [Cilk from Wikipedia](https://en.wikipedia.org/wiki/Cilk) » Cilk
- [The Cilk Project](http://supertech.csail.mit.edu/cilk/) from MIT
- [Fetch-and-add from Wikipedia](https://en.wikipedia.org/wiki/Fetch-and-add)
- [Intel Cilk Plus from Wikipedia](https://en.wikipedia.org/wiki/Intel_Cilk_Plus)
- [XMTC from Wikipedia](https://en.wikipedia.org/wiki/XMTC)
- Ian Carr with Nucleus - [Solar Plexus 1971](https://en.wikipedia.org/wiki/Nucleus_(band)#Discography), feat. Karl Jenkins, Jeff Clyne and John Marshall, [YouTube](https://en.wikipedia.org/wiki/YouTube) Video

[Watch on YouTube](https://www.youtube.com/watch?v=gHwuo1eGouw)

# References

Up one Level    Jean-Christophe Weill (1996). The ABDADA Distributed Minimax Search Agorithm. Proceedings of the 1996 ACM Computer Science Conference, pp. 131-138. ACM, New York, N.Y, reprinted ICCA Journal, Vol. 19, No. 1, [zipped postscript](http://www.recherche.enac.fr/%7Eweill/publications/acm.ps.gz)↩︎ [Lazy SMP, part 2](http://www.talkchess.com/forum/viewtopic.php?t=46858) by Daniel Homan, CCC, January 12, 2013↩︎ [Lazy SMP in Cheng](http://www.talkchess.com/forum/viewtopic.php?t=55188) by Martin Sedlak, CCC, February 02, 2015↩︎ [Re: A new chess engine : m8 (comming not so soon)](http://www.talkchess.com/forum/viewtopic.php?t=55170&start=11) by Peter Österlund, CCC, February 01, 2015↩︎ [Memory disambiguation from Wikipedia](https://en.wikipedia.org/wiki/Memory_disambiguation)↩︎ Don Dailey, Charles E. Leiserson (2001). Using Cilk to Write Multiprocessor Chess Programs. Advances in Computer Games 9, [pdf](http://supertech.csail.mit.edu/papers/icca99.pdf), 5 Other parallel programming issues, Cilk support for atomicity, pp. 17-18↩︎ Robert Hyatt, Tim Mann (2002). [A lock-less transposition table implementation for parallel search chess engines](http://www.craftychess.com/hyatt/hashing.html). ICGA Journal, Vol. 25, No. 1↩︎ [Re: Lockless hash: Thank you Bob Hyatt!](http://www.talkchess.com/forum/viewtopic.php?topic_view=threads&p=531603&t=49099) by Robert Hyatt, CCC, August 26, 2013↩︎ Robert Hyatt, Anthony Cozzie (2005). [The Effect of Hash Signature Collisions in a Chess Program](http://www.craftychess.com/hyatt/collisions.html). ICGA Journal, Vol. 28, No. 3↩︎ [Re: lockless hashing](http://www.talkchess.com/forum/viewtopic.php?topic_view=threads&p=393348&t=37976) by H.G.Muller, CCC, February 07, 2011↩︎ [Re: Effectively atomic read of 16 bytes on x86_64 without cmpxchg16b?](https://groups.google.com/d/msg/lock-free/hXtlgYrJj7M/j5mTHsaWYo0J) by [Anthony Williams](http://stackoverflow.com/users/5597/anthony-williams), [groups.google.lock-free](https://groups.google.com/forum/#!forum/lock-free), February 08, 2012↩︎ [Re: Speaking of the hash table](http://www.talkchess.com/forum/viewtopic.php?t=46346&start=44) by Ronald de Man, CCC, December 10, 2012↩︎ [SSE instructions: single memory access](http://stackoverflow.com/questions/7646018/sse-instructions-single-memory-access) by Atom, [Stack overflow](https://de.wikipedia.org/wiki/Stack_Overflow_%28Website%29), October 04, 2011↩︎ [Intel® 64 and IA-32 Architectures Developer's Manual: Vol. 3A](http://www.intel.com/content/www/us/en/architecture-and-technology/64-ia-32-architectures-software-developer-vol-3a-part-1-manual.html), section 8.2.3.1.↩︎ [Fetch-and-add from Wikipedia](https://en.wikipedia.org/wiki/Fetch-and-add)↩︎ [shm_open](http://pubs.opengroup.org/onlinepubs/007908799/xsh/shm_open.html), [The Single UNIX Specification version 2](https://en.wikipedia.org/wiki/Single_UNIX_Specification#1997:_Single_UNIX_Specification_version_2), Copyright © 1997 [The Open Group](https://en.wikipedia.org/wiki/The_Open_Group)↩︎ [Maged Michael - Selected Publications](http://www.research.ibm.com/people/m/michael/pubs.htm)↩︎ [The Aurora Distributed Shared Data System](http://webdocs.cs.ualberta.ca/~paullu/Aurora/aurora.html)↩︎ [Re: scorpio can run on 8192 cores](http://www.talkchess.com/forum/viewtopic.php?t=57343&start=5) by Daniel Shawul, CCC, August 29, 2015↩︎ [Transposition-driven scheduling - Wikipedia](https://en.wikipedia.org/wiki/Transposition-driven_scheduling)↩︎ [Transposition driven scheduling](http://www.talkchess.com/forum/viewtopic.php?t=47700) by Daniel Shawul, CCC, April 04, 2013↩︎ [Information on the C++11 Memory Model](http://scottmeyers.blogspot.co.uk/2012/04/information-on-c11-memory-model.html) by [Scott Meyers](https://en.wikipedia.org/wiki/Scott_Meyers), April 24, 2012↩︎ John Romein, Henri Bal, Jonathan Schaeffer, Aske Plaat (2002). A Performance Analysis of Transposition-Table-Driven Scheduling in Distributed Search. IEEE Transactions on Parallel and Distributed Systems, Vol. 13, No. 5, pp. 447–459. [pdf](http://www.cs.vu.nl/~bal/Papers/tds.pdf)↩︎
