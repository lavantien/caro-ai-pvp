source: https://chessprogramming.org/Alpha-Beta

# Alpha-Beta

Home * Search * Alpha-Beta

Alpha Beta 1

---

1. [Alpha Beta - Astral Abuse](http://vangelismovements.com/alphabeta.htm) a look at the music of Vangelis Papathanassiou↩︎

The Alpha-Beta algorithm (Alpha-Beta Pruning, Alpha-Beta Heuristic 1 ) is a significant enhancement to the minimax search algorithm that eliminates the need to search large portions of the game tree applying a [branch-and-bound](https://en.wikipedia.org/wiki/Branch_and_bound) technique. Remarkably, it does this without any potential of overlooking a better move. If one already has found a quite good move and search for alternatives, one refutation is enough to avoid it. No need to look for even stronger refutations. The algorithm maintains two values, alpha and beta. They represent the minimum score that the maximizing player is assured of and the maximum score that the minimizing player is assured of respectively. Consider the following example...

# How it works

Say it is White's turn to move, and we are searching to a depth of 2 (that is, we are consider all of White's moves, and all of Black's responses to each of those moves.) First we pick one of White's possible moves - let's call this Possible Move #1. We consider this move and every possible response to this move by black. After this analysis, we determine that the result of making Possible Move #1 is an even position. Then, we move on and consider another of White's possible moves (Possible Move #2.) When we consider the first possible counter-move by black, we discover that playing this results in black winning a Rook! In this situation, we can safely ignore all of Black's other possible responses to Possible Move #2 because we already know that Possible Move #1 is better. We really don't care exactly how much worse Possible Move #2 is. Maybe another possible response wins a Queen, but it doesn't matter because we know that we can achieve at least an even game by playing Possible Move #1. The full analysis of Possible Move #1 gave us a lower bound. We know that we can achieve at least that, so anything that is clearly worse can be ignored.

The situation becomes even more complicated, however, when we go to a search depth of 3 or greater, because now both players can make choices affecting the game tree. Now we have to maintain both a lower bound and an upper bound (called Alpha and Beta.) We maintain a lower bound because if a move is too bad we don't consider it. But we also have to maintain an upper bound because if a move at depth 3 or higher leads to a continuation that is too good, the other player won't allow it, because there was a better move higher up on the game tree that he could have played to avoid this situation. One player's lower bound is the other player's upper bound.

# Savings

The savings of alpha beta can be considerable. If a standard minimax search tree has x nodes, an alpha beta tree in a well-written program can have a node count close to the square-root of x. How many nodes you can actually cut, however, depends on how well ordered your game tree is. If you always search the best possible move first, you eliminate the most of the nodes. Of course, we don't always know what the best move is, or we wouldn't have to search in the first place. Conversely, if we always searched worse moves before the better moves, we wouldn't be able to cut any part of the tree at all! For this reason, good move ordering is very important, and is the focus of a lot of the effort of writing a good chess program. As pointed out by Levin in 1961, assuming constantly b moves for each node visited and search depth n, the maximal number of leaves in alpha-beta is equivalent to minimax, b ^ n. Considering always the best move first, it is b ^ [ceil(n/2)](https://en.wikipedia.org/wiki/Floor_and_ceiling_functions) plus b ^ [floor(n/2)](https://en.wikipedia.org/wiki/Floor_and_ceiling_functions) minus one. The minimal number of leaves is shown in following table which also demonstrates the odd-even effect:

| depth n | bn | b⌈n/2⌉ + b⌊n/2⌋ - 1 |
|---|---|---|
| 0 | 1 | 1 |
| 1 | 40 | 40 |
| 2 | 1,600 | 79 |
| 3 | 64,000 | 1,639 |
| 4 | 2,560,000 | 3,199 |
| 5 | 102,400,000 | 65,569 |
| 6 | 4,096,000,000 | 127,999 |
| 7 | 163,840,000,000 | 2,623,999 |
| 8 | 6,553,600,000,000 | 5,119,999 |

number of leaves with depth n and b = 40

# History

Alpha-Beta was invented independently by several researchers and pioneers from the 50s 2, and further research until the 80s, most notable by

- John McCarthy proposed the idea of Alpha-Beta after the representation of the Chess Program by Alex Bernstein 3 at the [Dartmouth Workshop](https://en.wikipedia.org/wiki/Dartmouth_workshop) in 1956 4
- Allen Newell and Herbert Simon Approximation in 1958
- Arthur Samuel Approximation in 1959
- Daniel Edwards and Timothy Hart, Description in 1961 5
- Alexander Brudno, Description in 1963
- Samuel Fuller, John Gaschnig, James Gillogly, Analysis 1973 6
- Donald Knuth, Analysis in 1975

Knuth and Moore‘s famous Function F2, aka AlphaBeta

Knuth already introduced an iterative solution, see Iterative Search

Knuth's node types

- Gérard M. Baudet, Analysis in 1978

# Quotes

## McCarthy

Quote by John McCarthy from Human-Level AI is harder than it seemed in 1955 on the [Dartmouth workshop](https://en.wikipedia.org/wiki/Dartmouth_workshop):

`Chess programs catch some of the human chess playing abilities but rely on the limited``effective branching``of the chess move``tree``. The ideas that work for chess are inadequate for``go``.``Alpha-beta pruning``characterizes human play, but it wasn't noticed by``early chess programmers``-``Turing``,``Shannon``,``Pasta``and``Ulam``, and``Bernstein``. We humans are not very good at identifying the heuristics we ourselves use. Approximations to alpha-beta used by``Samuel``,``Newell``and``Simon``, McCarthy. Proved equivalent to``minimax``by``Hart``and``Levin``, independently by``Brudno``.``Knuth``gives details.`

## Ershov and Shura-Bura

Quote from The Early Development of Programming in the USSR by Andrey Ershov and Mikhail R. Shura-Bura 7

`At the end of the 1950's a group of Moscow mathematicians began a study of computerized chess. Sixteen years later, the studies would lead to victory in the``first world chess tournament for computer programs``held in Stockholm during the 1974``IFIP``Congress. An important component of this success was a deep study of the problems of information organization in``computer memory``and of various``search heuristics``.``G. M. Adelson-Velsky``and``E. M. Landis``invented the`[`binary search tree`](https://en.wikipedia.org/wiki/AVL_tree)`("dichotomic inquiry") and``A. L. Brudno``, independent of``J. McCarthy``, discovered the``(α,β)-heuristic``for reducing search times on a game tree.`

## Knuth

Quote by Knuth 8 : It is interesting to convert this recursive procedure to an iterative (non-recursive) form by a sequence of mechanical transformations, and to apply simple optimizations which preserve program correctness. The resulting procedure is surprisingly simple, but not as easy to prove correct as the recursive form.

# Implementation

## Fail soft

In the examples below, alpha and beta act as soft bounds of the return value if depth left is greater zero in the above code samples, this is referred to a fail-soft-framework. Fail-Soft Alpha-Beta 9 may return scores outside the bounds, that is either greater than beta or less than alpha. As such, it has to keep track of the best score, which might be below alpha. Fail-soft is generally regarded better than fail-hard (see example below) because it retains more information for the search.

### Max versus Min

A C-like pseudo code implementation of the alpha-beta algorithm with distinct indirect recursive routines for the max- and min-player, similar to the minimax routines. Making and unmaking moves is omitted, and should be done before and after the recursive calls. So called beta-cutoffs occur for the max-play, alpha-cutoffs for the min-player. Note that minimax is not used in practice due to the high amount of code duplication.

```
int alphaBetaMax( int alpha, int beta, int depthleft ) {
   if ( depthleft == 0 ) return evaluate();
   bestValue = -infinity;
   for ( all moves) {
      score = alphaBetaMin( alpha, beta, depthleft - 1 );
      if( score > bestValue )
      {
         bestValue = score;
         if( score > alpha )
            alpha = score; // alpha acts like max in MiniMax
      }
      if( score >= beta )
         return score;   // fail soft beta-cutoff
   }
   return bestValue;
}

int alphaBetaMin( int alpha, int beta, int depthleft ) {
   if ( depthleft == 0 ) return -evaluate();
   bestValue = infinity;
   for ( all moves) {
      score = alphaBetaMax( alpha, beta, depthleft - 1 );
      if( score < bestValue)
      {
         bestValue = score;
         if( score < beta )
            beta = score; // beta acts like min in MiniMax
      }
      if( score <= alpha )
         return score; // fail soft alpha-cutoff, break can also be used here
   }
   return bestValue;
}
```

With this call from the Root:

```
   score = alphaBetaMax(-oo, +oo, depth);
```

Alpha-beta search tree with two alpha-cuts at min nodes 10

### Negamax Framework

Inside a negamax framework the routine looks simpler, but is not necessarily simpler to understand. Despite negating the returned score of the direct recursion, alpha of the min-player becomes minus beta of the max-player and vice versa, and the term alpha-cutoff or alpha-pruning is somehow diminished.

```
int alphaBeta( int alpha, int beta, int depthleft ) {
   if( depthleft == 0 ) return quiesce( alpha, beta );
   bestValue = -infinity;
   for ( all moves)  {
      score = -alphaBeta( -beta, -alpha, depthleft - 1 );
      if( score > bestValue )
      {
         bestValue = score;
         if( score > alpha )
            alpha = score; // alpha acts like max in MiniMax
      }
      if( score >= beta )
         return bestValue;   //  fail soft beta-cutoff, existing the loop here is also fine
   }
   return bestValue;
}
```

Note #1: Notice the call to quiesce(). This performs a quiescence search, which makes the alpha-beta search much more stable.

Note #2: This function only returns the score for the position, not the best move. Normally, the search state is encapsulated in a searcher object, which, among other things, records the best move found at the root node. Also, most search functions collect the Principal Variation not only for display purposes, but for a good guess as the leftmost path of the next iteration inside an iterative deepening framework.

## Fail hard

In the examples below, Since alpha and beta act as hard bounds of the return value if depth left is greater zero in the above code samples, this is referred to a fail-hard-framework.

### Max versus Min

```
int alphaBetaMax( int alpha, int beta, int depthleft ) {
   if ( depthleft == 0 ) return evaluate();
   for ( all moves) {
      score = alphaBetaMin( alpha, beta, depthleft - 1 );
      if( score >= beta )
         return beta;   // fail hard beta-cutoff
      if( score > alpha )
         alpha = score; // alpha acts like max in MiniMax
   }
   return alpha;
}

int alphaBetaMin( int alpha, int beta, int depthleft ) {
   if ( depthleft == 0 ) return -evaluate();
   for ( all moves) {
      score = alphaBetaMax( alpha, beta, depthleft - 1 );
      if( score <= alpha )
         return alpha; // fail hard alpha-cutoff
      if( score < beta )
         beta = score; // beta acts like min in MiniMax
   }
   return beta;
}
```

### Negamax Framework

```
int alphaBeta( int alpha, int beta, int depthleft ) {
   if( depthleft == 0 ) return quiesce( alpha, beta );
   for ( all moves)  {
      score = -alphaBeta( -beta, -alpha, depthleft - 1 );
      if( score >= beta )
         return beta;   //  fail hard beta-cutoff
      if( score > alpha )
         alpha = score; // alpha acts like max in MiniMax
   }
   return alpha;
}
```

# Enhancements

The alpha-beta algorithm can also be improved. These enhancements come from the fact that if you restrict the window of scores that are interesting, you can achieve more cutoffs. Since move ordering is so much important, a technique applied outside of the search for this is iterative deepening boosted by a transposition table, and possibly aspiration windows. Other enhancements, applied within the search function, are further discussed.

## Obligatory

- Transposition Table
- Iterative Deepening
- Aspiration Windows

## Selectivity

- Quiescence Search
- Selectivity

## Scout and Friends

- Scout
- NegaScout
- Principal Variation Search

# Alpha-Beta goes Best-First

- Alpha-Beta Conspiracy Search
- MTD(f)
- NegaC*
- SSS* and Dual* as MT

# See also

- Alpha
- Alpha-Beta Benchmark by Stephen F. Wheeler
- Beta
- Beta-Cutoff
- Bound
- CPW-Engine_search
- Fail-Low
- Fail-High
- Gamma-Algorithm
- Iterative Search
- KC Chess
- MCαβ
- Minimax
- Negamax
- Node Types
- Odd-Even Effect
- Parallel Alpha-Beta

Parallel Alpha-Beta in Cilk

- Search Explosion
- Theorem-Proving and M & N procedure
- Window

# Selected Publications

## 1958 ..

- Allen Newell, Cliff Shaw, Herbert Simon (1958). Chess Playing Programs and the Problem of Complexity. IBM Journal of Research and Development, Vol. 4, No. 2, pp. 320-335. Reprinted (1963) in [Computers and Thought](http://mitpress.mit.edu/catalog/item/default.asp?ttype=2&tid=6685) (eds. Edward A. Feigenbaum and Julian Feldman), pp. 39-70. McGraw-Hill, New York, N.Y., [pdf](http://www.research.ibm.com/journal/rd/024/ibmrd0204I.pdf)
- Arthur Samuel (1959). [Some Studies in Machine Learning Using the Game of Checkers](http://domino.watson.ibm.com/tchjr/journalindex.nsf/600cc5649e2871db852568150060213c/39a870213169f45685256bfa00683d74%21OpenDocument). IBM Journal July 1959

## 1960 ...

- Daniel Edwards, Timothy Hart (1961). The Alpha-Beta Heuristic, AIM-030, reprint available from [DSpace](http://dspace.mit.edu/handle/1721.1/6098) at MIT. Retrieved on 2006-12-21.
- Alexander Brudno (1963). Bounds and valuations for shortening the search of estimates. Problemy Kibernetiki (10) 141–150 and Problems of Cybernetics (10) 225–241
- James R. Slagle (1963). Game Trees, M & N Minimaxing, and the M & N alpha-beta procedure. Artificial Intelligence Group Report 3, UCRL-4671, University of California
- James R. Slagle, John K. Dixon (1969). Experiments With Some Programs That Search Game Trees. Journal of the ACM, Vol. 16, No. 2: 189-207, [pdf](http://wiki.cs.pdx.edu/wurzburg2009/nfp/abmin.pdf)

## 1970 ...

- Samuel Fuller, John Gaschnig, James Gillogly (1973). An Analysis of the Alpha-Beta Pruning Algorithm. Technical Report, Carnegie Mellon University, [pdf](http://shelf2.library.cmu.edu/Tech/17700646.pdf)
- Donald Knuth, [Ronald W. Moore](http://www.informatik.uni-trier.de/~ley/pers/hd/m/Moore:Ronald_W=) (1975). An Analysis of Alpha-Beta Pruning. [Artificial Intelligence](https://en.wikipedia.org/wiki/Artificial_Intelligence_%28journal%29), Vol. 6, No. 4, pp 293–326. Reprinted in Donald Knuth (2000). [Selected Papers on Analysis of Algorithms](http://www-cs-faculty.stanford.edu/~uno/aa.html). [CSLI lecture notes series](http://web.stanford.edu/group/cslipublications/cslipublications/site/CSIN.shtml) 102, ISBN 1-57586-212-3, [pdf](http://www-public.it-sudparis.eu/~gibson/Teaching/CSC4504/ReadingMaterial/KnuthMoore75.pdf)
- Arnold K. Griffith (1976). [Empirical Exploration of the Performance of the Alpha-Beta Tree-Searching Heuristic](http://ieeexplore.ieee.org/xpl/articleDetails.jsp?arnumber=5009198). IEEE Transactions on Computers, Vol. C-25, No. 1
- Gérard M. Baudet (1978). An Analysis of the Full Alpha-Beta Pruning Algorithm. STOC 1978: 296-313
- Gérard M. Baudet (1978). On the branching factor of the alpha-beta pruning algorithm. [Artificial Intelligence](https://en.wikipedia.org/wiki/Artificial_Intelligence_%28journal%29), Vol. 10
- Patrick Winston (1978). Dealing with Adversaries. Personal Computing, Vol. 2, No. 11, pp. 30, November 1978 » Alpha-Beta
- Gary Lindstrom (1979). Alpha-Beta Pruning on Evolving Game Trees. Technical Report UUCCS 79-101, [University of Utah](https://en.wikipedia.org/wiki/University_of_Utah), [UScholar Works](http://content.lib.utah.edu/cdm/ref/collection/uspace/id/498)
- Ward Douglas Maurer (1979). [Alpha-Beta Pruning](https://archive.org/stream/byte-magazine-1979-11/1979_11_BYTE_04-11_Fun_and_Games#page/n85/mode/2up). BYTE, Vol. 4, No. 11, pp. 84-96

## 1980 ...

- David Levy (1980). [Intelligent Games](http://archive.org/stream/creativecomputing-1980-04/Creative_Computing_v06_n04_1980_Apr#page/n117/mode/2up). Creative Computing, Vol. 6, No. 4, hosted by the [Internet Archive](https://en.wikipedia.org/wiki/Internet_Archive)
- Judea Pearl (1981). Heuristic search theory: A survey of recent results. [IJCAI-81](http://www.informatik.uni-trier.de/%7Eley/db/conf/ijcai/ijcai81.html), [pdf](http://ijcai.org/Past%20Proceedings/IJCAI-81-VOL%201/PDF/100.pdf)
- Igor Roizen (1981). On the Average Number of Terminal Nodes examined by Alpha-Beta. Technical Report UCLA-ENG-CSL-8108, [University of California at Los Angeles](https://en.wikipedia.org/wiki/University_of_California,_Los_Angeles), Cognitive Systems Laboratory
- Judea Pearl (1982). [The Solution for the Branching Factor of the Alpha-Beta Pruning Algorithm and its Optimality](http://portal.acm.org/citation.cfm?id=358616&dl=ACM&coll=DL&CFID=27355608&CFTOKEN=40935826). Communications of the ACM, Vol. 25, No. 8
- Peter W. Frey (1983). The Alpha-Beta Algorithm: Incremental Updating, Well-Behaved Evaluation Functions, and Non-Speculative Forward Pruning. Computer Game-Playing (ed. Max Bramer), pp. 285-289. Ellis Horwood Limited
- John Philip Fishburn (1983). [Another optimization of alpha-beta search](http://portal.acm.org/citation.cfm?id=1056623.1056628&coll=DL&dl=GUIDE&CFID=26266656&CFTOKEN=86225814). SIGART Bulletin, Issue 84, [pdf](https://drive.google.com/file/d/0B2pvWWlf39g-cjJpZkc1cDhfbkk/view) » Fail-Soft
- [John Hughes](https://en.wikipedia.org/wiki/John_Hughes_%28computer_scientist%29) (1984). Why Functional Programming Matters. 5 An Example from Artificial Intelligence, [Chalmers Tekniska Högskola](https://en.wikipedia.org/wiki/Chalmers_University_of_Technology), [Göteborg](https://en.wikipedia.org/wiki/Gothenburg), [pdf](http://www.cse.chalmers.se/~rjmh/Papers/whyfp.pdf),
- Stephen F. Wheeler (1985). [A performance benchmark of the alpha-beta procedure on randomly ordered non-uniform depth-first game-trees generated by a chess program](https://www.researchgate.net/publication/34381496_A_performance_benchmark_of_the_alpha-beta_procedure_on_randomly_ordered_non-uniform_depth-first_game-trees_generated_by_a_chess_program). M.Sc. thesis, [East Texas State University](https://en.wikipedia.org/wiki/Texas_A%26M_University%E2%80%93Commerce)
- Toshihide Ibaraki (1986). Generalization of Alpha-Beta and SSS* Search Procedures. [Artificial Intelligence](https://en.wikipedia.org/wiki/Artificial_Intelligence_%28journal%29), Vol. 29
- Matthew Huntbach, F. Warren Burton (1988). [Alpha-beta search on virtual tree machines](http://www.sciencedirect.com/science/article/pii/0020025588900540). [Information Sciences](http://www.journals.elsevier.com/information-sciences/), Vol. 44, No. 1
- Robert Hyatt, Bruce W. Suter, Harry Nelson (1989). A Parallel Alpha-Beta Tree Searching Algorithm. [Parallel Computing](https://www.journals.elsevier.com/parallel-computing), Vol. 10, No. 3

## 1990 ...

- Ingo Althöfer, Bernhard Balkenhol (1991). [[https://www.sciencedirect.com/science/article/abs/pii/000437029190042I#](https://www.sciencedirect.com/science/article/abs/pii/000437029190042I#)! A Game Tree with Distinct Leaf Values which is Easy for the Alpha-Beta Algorithm]. [Artificial Intelligence](https://en.wikipedia.org/wiki/Artificial_Intelligence_%28journal%29) Vol. 52, No. 2
- Alois Heinz, Christoph Hense (1993). [Bootstrap learning of α-β-evaluation functions](http://citeseerx.ist.psu.edu/viewdoc/summary?doi=10.1.1.56.872). [ICCI 1993](http://dblp.uni-trier.de/db/conf/icci/icci1993.html#HeinzH93), [pdf](http://citeseerx.ist.psu.edu/viewdoc/download?doi=10.1.1.56.872&rep=rep1&type=pdf)
- Van-Dat Cung (1993). Parallelizations of Game-Tree Search. [PARCO 1993](http://dblp.uni-trier.de/db/conf/parco/parco1993.html#Cung93), [pdf](http://citeseerx.ist.psu.edu/viewdoc/download?doi=10.1.1.48.6959&rep=rep1&type=pdf) hosted by [CiteSeerX](https://en.wikipedia.org/wiki/CiteSeer)
- Alois Heinz (1994). [Efficient Neural Net α-β-Evaluators](http://citeseerx.ist.psu.edu/viewdoc/summary?doi=10.1.1.55.3994). [pdf](http://citeseerx.ist.psu.edu/viewdoc/download?doi=10.1.1.55.3994&rep=rep1&type=pdf)
- Yanjun Zhang (1995). [On the Optimality of Randomized Alpha-Beta Search](https://epubs.siam.org/doi/abs/10.1137/S009753979223037X). [SIAM Journal on Computing](https://en.wikipedia.org/wiki/SIAM_Journal_on_Computing), Vol. 24, No. 1
- Ernst A. Heinz (1999). [Scalable Search in Computer Chess](http://people.csail.mit.edu/heinz/node1.html#scale-cchess). [Morgan Kaufmann](https://en.wikipedia.org/wiki/Morgan_Kaufmann), ISBN 3-528-05732-7

## 2000 ...

- Matthew L. Ginsberg, Alan Jaffray (2002). Alpha-Beta Pruning Under Partial Orders. in Richard J. Nowakowski (ed.) [More Games of No Chance](http://library.msri.org/books/Book42/). [Cambridge University Press](https://en.wikipedia.org/wiki/Cambridge_University_Press), [pdf](http://library.msri.org/books/Book42/files/ginsberg.pdf)
- Todd W. Neller (2002). [Information-Based Alpha-Beta Search and the Homicidal Chauffeur](http://cupola.gettysburg.edu/csfac/11/). [HSCC 2002](http://dblp.uni-trier.de/db/conf/hybrid/hscc2002.html#Neller02), in [Claire J. Tomlin](http://people.eecs.berkeley.edu/~tomlin/), [Mark R. Greenstreet](https://www.cs.ubc.ca/~mrg/) (eds.) (2002). [Hybrid Systems: Computation and Control](http://link.springer.com/book/10.1007/3-540-45873-5). [Lecture Notes in Computer Science](https://en.wikipedia.org/wiki/Lecture_Notes_in_Computer_Science) 2289, [Springer](https://en.wikipedia.org/wiki/Springer_Science%2BBusiness_Media) 11
- Jacek Mańdziuk, Daniel Osman (2004). Alpha-Beta Search Enhancements with a Real-Value Game-State Evaluation Function. ICGA Journal, Vol. 27, No. 1, [pdf](http://www.mini.pw.edu.pl/~mandziuk/PRACE/ICGA.pdf)
- Hendrik Baier (2006). Der Alpha-Beta-Algorithmus und Erweiterungen bei Vier Gewinnt. Bachelor's thesis (German), TU Darmstadt, advisor Johannes Fürnkranz, [pdf](http://www.ke.tu-darmstadt.de/lehre/arbeiten/bachelor/2006/Baier_Hendrik.pdf)

## 2010 ...

- Damjan Strnad, Nikola Guid (2011). [Parallel Alpha-Beta Algorithm on the GPU](http://cit.fer.hr/index.php/CIT/article/view/2029). [CIT. Journal of Computing and Information Technology](http://cit.fer.hr/index.php/CIT), Vol. 19, No. 4 » GPU, Parallel Search, Reversi
- Abdallah Saffidine, Hilmar Finnsson, Michael Buro (2012). Alpha-Beta Pruning for Games with Simultaneous Moves. AAAI 2012
- Daniel S. Abdi (2013). Analysis of pruned minimax trees. [pdf](https://dl.dropboxusercontent.com/u/55295461/analysis/pruning.pdf) » Late Move Reductions, Null Move Pruning
- Jr-Chang Chen, I-Chen Wu, Wen-Jie Tseng, Bo-Han Lin, Chia-Hui Chang (2015). [Job-Level Alpha-Beta Search](https://ir.nctu.edu.tw/handle/11536/124541). IEEE Transactions on Computational Intelligence and AI in Games, Vol. 7, No. 1
- Bojun Huang (2015). [Pruning Game Tree by Rollouts](https://www.semanticscholar.org/paper/Pruning-Game-Tree-by-Rollouts-Huang/a38b358745067f71a9c780db117ae2471e693d63). AAAI » MCTS, MT-SSS*, Rollout Paradigm 12
- Naoyuki Sato, Kokolo Ikeda (2016). [Three types of forward pruning techniques to apply the alpha beta algorithm to turn-based strategy games](https://ieeexplore.ieee.org/document/7860427). [CIG 2016](https://dblp.uni-trier.de/db/conf/cig/cig2016.html)
- Hendrik Baier (2017). [A Rollout-Based Search Algorithm Unifying MCTS and Alpha-Beta](https://link.springer.com/chapter/10.1007/978-3-319-57969-6_5). [Computer Games](https://link.springer.com/book/10.1007%2F978-3-319-57969-6) » MCαβ, Monte-Carlo Tree Search
- Shogo Takeuchi (2019). Advice is Useful for Game AI: Experiments with Alpha-Beta Search Players in Shogi. Advances in Computer Games 16

# Forum Posts

## 1993 ...

- [computer chess](https://groups.google.com/d/msg/rec.games.chess/XQWb-ZjSsy0/IQO0MduTUjQJ) by Paul W Celmer, rgc, September 10, 1993

[Re: Computer Chess (LONG)](https://groups.google.com/d/msg/rec.games.chess/XQWb-ZjSsy0/CjVUkx-hSQIJ) by Andy Walker, rgc, September 16, 1993

[Computer Chess and alpha-beta pruning](https://groups.google.com/d/msg/rec.games.chess/XQWb-ZjSsy0/JLZH-MwDbQoJ) by Rickard Westman, rgc, September 21, 1993

[Re: Computer Chess and alpha-beta pruning](https://groups.google.com/d/msg/rec.games.chess/XQWb-ZjSsy0/gsXMq42a-FAJ) by Johannes Fürnkranz, rgc, September 22, 1993 » Iterative Deepening

[alpha-beta pruning != brute force](https://groups.google.com/d/msg/rec.games.chess/XQWb-ZjSsy0/MiYEhpjTT08J) by Feng-hsiung Hsu, rgc, September 22, 1993 » Brute-Force

[Re: Computer Chess and alpha-beta pruning](https://groups.google.com/d/msg/rec.games.chess/XQWb-ZjSsy0/YqxBGHAlO7AJ) by Robert Hyatt, rgc, September 25, 1993

- [Alpha-beta inconsistencies](https://groups.google.com/group/rec.games.chess/browse_frm/thread/b5f847cde3d26fd6) by Chua Kong Sian, rgc, February 19, 1994

## 1995 ...

- [Alpha-Beta explained?](https://groups.google.com/d/msg/rec.games.chess.computer/TSAzRyajwsg/G6Ts2VFXhJcJ) by Brian, rgcc, October 15, 1996
- [New improvement to alpha/beta + TT?](https://groups.google.com/group/rec.games.chess.computer/browse_frm/thread/a895e1a5524f8158) by Heiner Marxen, rgcc, January 13, 1997 » Fail-Soft
- [Re: Argument against Alpha-Beta searching](https://groups.google.com/group/rec.games.chess.computer/browse_frm/thread/f81d85c5fa058958) by Robert Hyatt, rgcc, March 19, 1997
- [computer chess "oracle" ideas...](https://groups.google.com/group/rec.games.chess.computer/browse_frm/thread/99eec6923b0481db) by Robert Hyatt, rgcc, April 1, 1997 » Oracle

[Re: computer chess "oracle" ideas...](https://groups.google.com/group/rec.games.chess.computer/msg/0df39371422a600c) by Ronald de Man, rgcc, April 3, 1997

[Re: computer chess "oracle" ideas...](https://groups.google.com/group/rec.games.chess.computer/msg/ccc2546e26d92f88) by Ronald de Man, rgcc, April 7, 1997

- [Basic alpha-beta question](https://www.stmintz.com/ccc/index.php?id=13725) by John Scalo, CCC, January 06, 1998
- [alpha-beta is silly?](https://www.stmintz.com/ccc/index.php?id=19760) by Werner Inmann, CCC, June 02, 1998

[Re: alpha-beta is silly?](https://www.stmintz.com/ccc/index.php?id=19922) by Don Dailey, CCC, June 03, 1998

- [Re: Basic questions about alpha beta](https://www.stmintz.com/ccc/index.php?id=28262) by Bruce Moreland, CCC, September 28, 1998

## 2000 ...

- [Another Alpha-Beta algorithm question](https://www.stmintz.com/ccc/index.php?id=98141) by Jeff Anderson, CCC, February 18, 2000
- [A Question on simple Alpha-Beta versus PVS/Negascout](https://www.stmintz.com/ccc/index.php?id=102792) by Andrei Fortuna, CCC, March 21, 2000 » Principal Variation Search, NegaScout
- [outline for alpha beta](https://www.stmintz.com/ccc/index.php?id=110353) by John Coffey, CCC, May 12, 2000
- [Alpha-Beta explanation on Heinz's book?](https://www.stmintz.com/ccc/index.php?id=131537) by Severi Salminen, CCC, October 05, 2000 13
- [Who invented the Alpha-Beta-algorithm?](https://www.stmintz.com/ccc/index.php?id=162573) by Rafael B. Andrist, CCC, April 09, 2001
- [The Alpha-Beta search!](https://www.stmintz.com/ccc/index.php?id=183650) by Sune Fischer, CCC, August 14, 2001
- [An Idiot's Guide to Minimax, Alpha/Beta, etc...](https://www.stmintz.com/ccc/index.php?id=281522) by Mike Carter, CCC, February 03, 2003
- [Fail soft alpha-beta](https://www.stmintz.com/ccc/index.php?id=314585) by Russell Reagan, CCC, September 08, 2003 » Fail-Soft
- [Complexity Analyses of Alpha-Beta](https://www.stmintz.com/ccc/index.php?id=319935) by Renze Steenhuisen, CCC, October 07, 2003
- [Mixing alpha-beta with PN search](https://www.stmintz.com/ccc/index.php?id=343084) by Tord Romstad, CCC, January 18, 2004 » Proof-Number Search
- [Question for Hyatt about Alpha/Beta](https://www.stmintz.com/ccc/index.php?id=347303) by Bob Durrett, CCC, February 05, 2004

## 2005 ...

- [Iterative alpha-beta search?](https://www.stmintz.com/ccc/index.php?id=478627) by Andrew Wagner, CCC, January 11, 2006 » Iterative Search
- [Trivial alfa-beta question](https://www.stmintz.com/ccc/index.php?id=487561) by Jouni Uski, CCC, February 18, 2006

## 2010 ...

- [Dumb question about alpha-beta](http://www.talkchess.com/forum/viewtopic.php?t=51491) by Daylen Yang, CCC, March 04, 2014

## 2015 ...

- [Search algorithm in it's simplest forum](http://www.talkchess.com/forum/viewtopic.php?t=55474) by Mahmoud Uthman, CCC, February 25, 2015
- [Negative alpha/beta windows: Are they useful?](http://www.talkchess.com/forum/viewtopic.php?t=55577) by Thomas Dybdahl Ahle, CCC, March 06, 2015
- [Stuck on Alphabeta](http://www.open-chess.org/viewtopic.php?f=5&t=2931) by kuket15, OpenChess Forum, December 07, 2015
- [Alpha-Beta woes, textbook-like resources, etc.](http://www.talkchess.com/forum/viewtopic.php?t=58923) by Meni Rosenfeld, CCC, January 14, 2016
- [Search](http://www.talkchess.com/forum/viewtopic.php?t=60581) by Laurie Tunnicliffe, CCC, June 24, 2016
- [Alpha-Beta as a rollouts algorithm](http://www.talkchess.com/forum/viewtopic.php?t=66414) by Daniel Shawul, CCC, January 25, 2018 » MCαβ, Monte-Carlo Tree Search, Scorpio

## 2020 ...

- [AB search with NN on GPU...](http://www.talkchess.com/forum3/viewtopic.php?f=7&t=74771) by Srdja Matovic, CCC, August 13, 2020 » GPU, NN 14
- [Mathematical proof that AB with B-cutoff does not miss a variation](http://www.talkchess.com/forum3/viewtopic.php?f=7&t=77208) by Yves De Billoëz, CCC, April 30, 2021
- [Alpha-beta search for drawing endgames](http://www.talkchess.com/forum3/viewtopic.php?f=7&t=77497) by Emanuel Torres, CCC, June 16, 2021 » Draw Evaluation, Graph History Interaction, Repetitions

# External Links

- [Alpha-beta Pruning from Wikipedia](https://en.wikipedia.org/wiki/Alpha-beta_pruning)
- [Branch-and-bound from Wikipedia](https://en.wikipedia.org/wiki/Branch_and_bound)
- [Integer programming from Wikipedia](https://en.wikipedia.org/wiki/Integer_programming) 15
- [The alpha-beta heuristic](http://www.chilton-computing.org.uk/acl/literature/books/gamesplaying/p004.htm#index01) from Alex Bell (1972). [Games Playing with Computers](http://www.chilton-computing.org.uk/acl/literature/books/gamesplaying/overview.htm). [Allen & Unwin](https://en.wikipedia.org/wiki/Allen_%26_Unwin), ISBN-13: 978-0080212227
- [Alpha-Beta Search](http://web.archive.org/web/20070811182954/www.seanet.com/%7Ebrucemo/topics/alphabeta.htm) from Bruce Moreland's [Programming Topics](http://web.archive.org/web/20070811182741/www.seanet.com/%7Ebrucemo/topics/topics.htm)
- [Lecture notes for April 22, 1997 Alpha-Beta Search](http://www.ics.uci.edu/%7Eeppstein/180a/970422.html) by David Eppstein
- [G13GAM -- Game Theory - alpha-beta pruning](http://web.archive.org/web/20070113132331/http://www.maths.nottingham.ac.uk/personal/anw/G13GT1/alphabet.html) by Andy Walker ([Wayback Machine](https://en.wikipedia.org/wiki/Wayback_Machine))
- [Alpha-Beta](https://web.archive.org/web/20120331060714/http://www.top-5000.nl/authors/rebel/chess840.htm#SEARCH) from [How Rebel Plays Chess](https://web.archive.org/web/20120331060714/http://www.top-5000.nl/authors/rebel/chess840.htm) by Ed Schröder ([Wayback Machine](https://en.wikipedia.org/wiki/Wayback_Machine))
- [Alpha-Beta search](https://web.archive.org/web/20060107101829/http://chess.verhelst.org/1997/03/10/search/#alpha-beta) from Paul Verhelst's Computer Chess Sites ([Wayback Machine](https://en.wikipedia.org/wiki/Wayback_Machine))
- [Alpha-Beta Slide Show in 18 steps](https://emunix.emich.edu/~mevett/AI/AlphaBeta_movie/sld001.htm) by [Mikael Bodén](https://scmb.uq.edu.au/profile/321/mikael-boden)
- [An Introduction to Game Tree Algorithms](http://hamedahmadi.com/gametree/) by Hamed Ahmadi
- [Alpha Beta](https://vangelismovements.com/alphabeta.htm) - [Astral Abuse](https://en.wikipedia.org/wiki/Vangelis_discography), 1971, [YouTube](https://en.wikipedia.org/wiki/YouTube) Video

Alpha Beta are [Vilma Lado](http://www.vangelismovements.com/vilmalado.htm), Vangelis Papathanassiou, [Argyris Koulouris](https://tr.wikipedia.org/wiki/Argyris_Koulouris) and [Giorgio Gomelski](https://en.wikipedia.org/wiki/Giorgio_Gomelsky)

[Watch on YouTube](https://www.youtube.com/watch?v=qhmwTc_MNjM)

# References

Up one level    Arthur Samuel (1967). Some Studies in Machine Learning. Using the Game of Checkers. II-Recent Progress. [pdf](https://researcher.ibm.com/researcher/files/us-beygel/samuel-checkers.pdf), IBM Journal - November 1967, on the name Alpha-Beta Heuristic pp. 602: So named by Prof. John McCarthy. This procedure was extensively investigated by Prof. John McCarthy and his students at M.I.T. but it has been inadequately described in the literature. It is, of course, not a heuristic at all, being a simple algorithmic procedure and actually only a special case of the more general "[branch and bound](https://en.wikipedia.org/wiki/Branch_and_bound)" technique which was been rediscovered many times and which is currently being exploited in [integer programming](https://en.wikipedia.org/wiki/Integer_programming) research.↩︎ Jaap van den Herik (2001). Science, Competition and Business. ICGA Journal, Vol. 24, No. 4, [pdf](http://arno.uvt.nl/show.cgi?fid=107331)↩︎ [The Dartmouth Workshop--as planned and as it happened](http://www-formal.stanford.edu/jmc/slides/dartmouth/dartmouth/node1.html)↩︎ [A Proposal for the Dartmouth Summer Research Project on Artificial Intelligence](http://www-formal.stanford.edu/jmc/history/dartmouth/dartmouth.html) by John McCarthy, Marvin Minsky, Nathaniel Rochester, Claude Shannon, August 31, 1955↩︎ Daniel Edwards and Timothy Hart (1961). The Alpha-Beta Heuristic, AIM-030, reprint available from [DSpace](http://dspace.mit.edu/handle/1721.1/6098) at MIT. Retrieved on 2006-12-21.↩︎ Samuel Fuller, John Gaschnig, James Gillogly (1973). An Analysis of the Alpha-Beta Pruning Algorithm. Technical Report, Carnegie Mellon University↩︎ Andrey Ershov, Mikhail R. Shura-Bura (1980). [The Early Development of Programming in the USSR](http://ershov.iis.nsk.su/archive/eaindex.asp?lang=2&gid=910). in [Nicholas C. Metropolis](https://en.wikipedia.org/wiki/Nicholas_C._Metropolis) (ed.) [A History of Computing in the Twentieth Century](http://dl.acm.org/citation.cfm?id=578384). [Academic Press](https://en.wikipedia.org/wiki/Academic_Press), [preprint pp. 44](http://ershov.iis.nsk.su/archive/eaimage.asp?did=28792&fileid=173671)↩︎ Donald Knuth, [Ronald W. Moore](http://www.informatik.uni-trier.de/~ley/pers/hd/m/Moore:Ronald_W=) (1975). [An analysis of alpha-beta pruning](http://www.scribd.com/doc/28194932/An-Analysis-of-Alpha-Beta-Pruning). [Artificial Intelligence](https://en.wikipedia.org/wiki/Artificial_Intelligence_%28journal%29), Vol. 6, No. 4, pp 293–326. Reprinted in Donald Knuth (2000). [Selected Papers on Analysis of Algorithms](http://www-cs-faculty.stanford.edu/~uno/aa.html). [CSLI lecture notes series](http://web.stanford.edu/group/cslipublications/cslipublications/site/CSIN.shtml) 102, ISBN 1-57586-212-3↩︎ John Philip Fishburn (1983). [Another optimization of alpha-beta search](http://portal.acm.org/citation.cfm?id=1056623.1056628&coll=DL&dl=GUIDE&CFID=26266656&CFTOKEN=86225814). SIGART Bulletin, Issue 84↩︎ McGill University, Winter 1997 Class Notes, [Topic #11: Game trees. Alpha-beta search](http://cgm.cs.mcgill.ca/~hagha/topic11/topic11.html), Diagram by Pui Yee Chan↩︎ [Homicidal chauffeur problem - Wikipedia](https://en.wikipedia.org/wiki/Homicidal_chauffeur_problem)↩︎ [Re: Announcing lczero](http://www.talkchess.com/forum/viewtopic.php?t=66280&start=67) by Daniel Shawul, CCC, January 21, 2018 » Leela Chess Zero↩︎ Ernst A. Heinz (1999). [Scalable Search in Computer Chess](http://people.csail.mit.edu/heinz/node1.html#scale-cchess). [Morgan Kaufmann](https://en.wikipedia.org/wiki/Morgan_Kaufmann), ISBN 3-528-05732-7↩︎ [kernel launch latency - CUDA / CUDA Programming and Performance - NVIDIA Developer Forums](https://forums.developer.nvidia.com/t/kernel-launch-latency/62455) by LukeCuda, June 18, 2018↩︎ [William Cook](http://www2.isye.gatech.edu/~wcook/) (2009). Fifty-Plus Years of Combinatorial Integer Programming. [pdf](http://www2.isye.gatech.edu/~wcook/papers/ip50.pdf)↩︎
