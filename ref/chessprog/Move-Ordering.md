source: https://chessprogramming.org/Move_Ordering

# Move Ordering

Home * Search * Move Ordering

Wassily Kandinsky - Four Parts 1

---

1. Wassily Kandinsky - [Four Parts](https://commons.wikimedia.org/wiki/File:Four_Parts_MET_sf1991.402.9.jpg?uselang=en), 1932, [Metropolitan Museum of Art](https://en.wikipedia.org/wiki/Metropolitan_Museum_of_Art)↩︎

For the alpha-beta algorithm to perform well, the best moves need to be searched first. This is especially true for PV-nodes and expected Cut-nodes. The goal is to become close to the minimal tree. On the other hand - at Cut-nodes - the best move is not always the cheapest refutation, see for instance enhanced transposition cutoff. Most important inside an iterative deepening framework is to try the principal variation of the previous iteration as the leftmost path for the next iteration, which might be applied by an explicit triangular PV-table or implicit by the transposition table.

# Standard techniques

Following techniques are common in finding a good first move

- PV-Move from the principal variation of the previous Iteration
- Hash Move - stored move from Transposition Table, if available
- Internal Iterative Deepening - if no hash move is available, likely only at PV-Nodes

## Captures

For captures (if any), a simple, but quite efficient heuristic is (re)capturing the last moved piece with the least valuable attacker. Otherwise following heuristics may used, concerning the order of captures:

- MVV-LVA - Most Valuable Victim - Least Valuable Aggressor
- Static Exchange Evaluation abbreviated as SEE

## Non-Captures

- Killer Heuristic 1
- History Heuristic 2
- Relative History Heuristic 3
- Dedicated Piece-Square Tables only for move ordering 4

## Less common techniques

These techniques are well known theoretically for non-captures, but not all programmers use them:

- Mate Killers
- Countermove Heuristic 5
- Guard Heuristic
- Last Best Reply
- Butterfly Heuristic 6
- Threat Move from null move refutations
- Enhanced Transposition Cutoff (ETC)
- Refutation Table

## Using Neural Networks

Move ordering (as well as Time Management) is an interesting application of Neural Networks, as introduced by Kieran Greer et al. and Levente Kocsis et al.

- Chessmaps Heuristic
- Neural MoveMap Heuristic 7 8

# Typical move ordering

After move generation with assigned move-scores, chess programs usually don't sort the whole move list, but perform a [selection sort](https://en.wikipedia.org/wiki/Selection_sort) each time a move is fetched. Exceptions are the Root and further PV-Nodes with some distance to the horizon, where one may apply additional effort to score and sort moves. For performance reasons, a lot of programs try to save the move generation of captures or non-captures at expected Cut-Nodes, but try the hash-move or killer first, if they are proved legal in this position.

A typical move ordering consists as follows:

1. PV-move of the principal variation from the previous iteration of an iterative deepening framework for the leftmost path, often implicitly done by 2.
2. Hash move from hash tables
3. Winning captures/promotions
4. Equal captures/promotions
5. Killer moves (non capture), often with mate killers first
6. Non-captures sorted by history heuristic and that like
7. Losing captures (* but see below

- ) Depending on the implementation, the board representation, whether and where SEE is used, the extension policy (recapture extensions) and other stuff - many programmers favor losing captures before other none-captures - directly behind the killers. They are kind of forced, and one possibly has to deal with all kind of tactical motives and interactions, one may not consider in move ordering. Such as pins, batteries, discovered attacks and overloaded defenders. Otherwise, obviously losing captures are likely refuted cheaply. But if a losing capture fails high for some reason, we have saved the effort to generate, and more importantly to search other non-captures at all.

# Node Type Considerations

Move ordering and scoring effort might be controlled by expected Node Types.

## PV-nodes

At PV-nodes move ordering is very important, since the best alpha-increase as early as possible makes further search cheaper, due to narrower windows in Alpha-Beta, while in PVS later but better moves require re-searches of the null window scout.

## Cut-nodes

Move ordering is crucial at expected and confirmed Cut-nodes, since it is important to fail-high as early as possible, as best with the first move, as in greater than 90% of all fail-high nodes. However, in situations where multiple moves may cut, e.g. with huge material advantage, we like it as cheap as possible, but not necessarily a huge subtree with f.i. due to check extensions.

## All-nodes

At confirmed ALL-nodes with null windows, move ordering didn't care that much. Since we don't know in advance (otherwise we wouldn't search at all), and expected All-nodes may become Cut-nodes, move ordering is an issue as well, but usually with less effort for late moves.

# Depth and Ply Considerations

Move ordering effort might be controlled by considering draft and/or plies from root. The closer the root, the farther the horizon, the more effort might be justified to score and sort moves.

## Root Node Considerations

Despite trying the best move and principal variation from previous iteration first, iterative deepening offers another ressource to order the remaining moves at the root - their subtree size which could be easily determined. As already mentioned by Ingo Althöfer in an 1992 ICCA Journal correspondence 9 inspired by Jos Uiterwijk's Countermove Heuristic article 10, based on the soundness of following rule of thumb,

| The longer it takes to refute a move, the higher is its chance to become best move in the next iteration |
|---|

the old idea is to use the search time or subtree size of the depth-n iteration to reorder the direct successors of the root before the depth-(n+1) iteration. Some programs use the evaluation to initially score the moves, to adjust them by their subtree size in subsequent iterations.

An idea to apply randomness and/or bonuses, i.e. developing bonus, or penalties to move scores at the root by an oracle approach, was proposed by Ronald de Man - without any changes in alpha-beta search or leaf evaluation, and without any problems with the transposition table 11 12.

## Ordering in Quiescence

In the quiescence search, captures are often approximated, for speed. For example, PxQ need not have a SEE performed on it, since it is clearly a winning capture, where RxB might have the SEE done, to see if it is a winning or possibly losing capture if the bishop is protected.

# See also

- Fixafan
- Move Generation
- Move List
- Node Types

Number of Leaf Nodes

- Odd-Even Effect
- Search Explosion

# Publications

## 1977 ...

- Selim Akl, Monroe Newborn (1977). The Principal Continuation and the Killer Heuristic.1977 ACM Annual Conference Proceedings
- Ozalp Babaoglu (1977). Hardware implementation of the legal move generation and relative ordering functions for the game of chess. Master's thesis, University of California, Berkeley

## 1980 ...

- Jonathan Schaeffer (1983). The History Heuristic. ICCA Journal, Vol. 6, No. 3
- Prakash Bettadapur (1986). Influence of Ordering on Capture Search. ICCA Journal, Vol. 9, No. 4
- Dap Hartmann (1988). Butterfly Boards. ICCA Journal, Vol. 11, Nos. 2/3

## 1990 ...

- Dap Hartmann, Peter Kouwenhoven (1991). Sundry Computer Chess Topics. Advances in Computer Chess 6
- Jos Uiterwijk (1992). The Countermove Heuristic. ICCA Journal, Vol. 15, No. 1
- Jos Uiterwijk (1992). Memory Efficiency in some Heuristics. ICCA Journal, Vol. 15, No. 2
- Ingo Althöfer (1992). Move Ordering by Time Ordering. Correspondence, ICCA Journal, Vol. 15, No. 2
- Eric Thé (1992). [An analysis of move ordering on the efficiency of alpha-beta search](http://digitool.library.mcgill.ca/R/?func=dbin-jump-full&object_id=56753&local_base=GEN01-MCG02). Master's thesis, McGill University, advisor Monroe Newborn » Fixafan
- Yi-Fan Ke, Tai-Ming Parng (1993). The Guard Heuristic: Legal Move Ordering with Forward Game-Tree Pruning. ICCA Journal, Vol. 16, No. 2
- Mei-En Chen, Yo-Ping Huang (1995). [Guard heuristic by dynamic fuzzy reasoning model for Chinese chess](http://ieeexplore.ieee.org/xpl/articleDetails.jsp?arnumber=527751). Proceedings of ISUMA-NAFIPS '95
- Kieran Greer, Piyush Ojha, David A. Bell (1999). A Pattern-Oriented Approach to Move Ordering: the Chessmaps Heuristic. ICCA Journal, Vol. 22, No. 1

## 2000 ...

- Kieran Greer (2000). [Computer chess move-ordering schemes using move influence](http://www.sciencedirect.com/science?_ob=ArticleURL&_udi=B6TYF-40TY77M-3&_user=10&_rdoc=1&_fmt=&_orig=search&_sort=d&view=c&_acct=C000050221&_version=1&_urlVersion=0&_userid=10&md5=7858eb0d6e100295c197661d1d454e26). [Artificial Intelligence](https://en.wikipedia.org/wiki/Artificial_Intelligence_%28journal%29), Vol. 120, No. 2
- Levente Kocsis, Jos Uiterwijk, Jaap van den Herik (2001). Move Ordering using Neural Networks. IEA/AIE 2001, [LNCS](https://en.wikipedia.org/wiki/Lecture_Notes_in_Computer_Science) 2070
- Levente Kocsis, Jos Uiterwijk, Eric Postma, Jaap van den Herik (2002). [The Neural MoveMap Heuristic in Chess](http://link.springer.com/chapter/10.1007/978-3-540-40031-8_11). CG 2002
- Yoshinori Higashiuchi, Reijer Grimbergen (2005). Enhancing Search Efficiency by Using Move Categorization Based on Game Progress in Amazons. [pdf](http://citeseerx.ist.psu.edu/viewdoc/download?doi=10.1.1.100.3211&rep=rep1&type=pdf)
- Mark Winands, Erik van der Werf, Jaap van den Herik, Jos Uiterwijk (2006). The Relative History Heuristic. CG 2004, [pdf](http://www.personeel.unimaas.nl/m-winands/documents/relhis.pdf)
- Jeff Rollason (2006). [Driving search with Plausibility analysis: Looking at the right moves](http://www.aifactory.co.uk/newsletter/2005_04_plausibility_analysis.htm). AI Factory, Winter 2006
- Jeff Rollason (2007). [Negative Plausibility](http://www.aifactory.co.uk/newsletter/2007_01_neg_plausibility.htm). AI Factory, Spring 2007 » Relative History Heuristic
- Rémi Coulom (2007). [Computing Elo Ratings of Move Patterns in the Game of Go](http://remi.coulom.free.fr/Amsterdam2007/). ICGA Journal, Vol. 30, No. 4, CGW 2007, [pdf](http://remi.coulom.free.fr/Amsterdam2007/icgaj.pdf) 13
- Timothy Furtak, Michael Buro (2009). Minimum Proof Graphs and Fastest-Cut-First Search Heuristics. IJCAI 2009, [pdf](http://ijcai.org/papers09/Papers/IJCAI09-089.pdf) 14

## 2010 ...

- David J. Wu (2011). Move Ranking and Evaluation in the Game of Arimaa. B.Sc. thesis, [Harvard College](https://en.wikipedia.org/wiki/Harvard_College), [Cambridge, Massachusetts](https://en.wikipedia.org/wiki/Cambridge,_Massachusetts), [pdf](http://arimaa.com/arimaa/papers/DavidWu/djwuthesis.pdf) » Arimaa
- David J. Wu (2015). Designing a Winning Arimaa Program. ICGA Journal, Vol. 38, No. 1

## 2020 ...

- Toni Helminen (2022). [Lazy sorting algorithm](https://github.com/SamuraiDangyo/lazy-sorting-algorithm). 15

# Forum Posts

## 1996 ...

- [Pawn Structure "Holes" and Move Ordering](https://groups.google.com/group/rec.games.chess.computer/browse_frm/thread/7d8a4124ce6f81ac#) by Daniel A. Thies, rgcc, February 12, 1996 » Pawn Structure, Holes
- [computer chess "oracle" ideas...](https://groups.google.com/group/rec.games.chess.computer/browse_frm/thread/99eec6923b0481db) by Robert Hyatt, rgcc, April 1, 1997

[Re: computer chess "oracle" ideas...](https://groups.google.com/group/rec.games.chess.computer/msg/0df39371422a600c) by Ronald de Man, rgcc, April 3, 1997

[Re: computer chess "oracle" ideas...](https://groups.google.com/group/rec.games.chess.computer/msg/ccc2546e26d92f88) by Ronald de Man, rgcc, April 7, 1997

- [Move ordering - How do I know if I have played this move already?](https://www.stmintz.com/ccc/index.php?id=48187) by Steve Maughan, CCC, April 06, 1999
- [Measure of moveorder quality](https://www.stmintz.com/ccc/index.php?id=67397) by Ralf Elvsén, CCC, September 04, 1999
- [Move Ordering at the Root](https://www.stmintz.com/ccc/index.php?id=68825) by Daniel Homan, CCC, September 15, 1999
- [Fast way to sort moves in movelist ?](https://www.stmintz.com/ccc/index.php?id=73278) by Stefan Plenkner, CCC, October 14, 1999

## 2000 ...

- [Move ordering?](https://www.stmintz.com/ccc/index.php?id=108325) by Severi Salminen, CCC, April 28, 2000
- [Better subject title: Move ordering](https://www.stmintz.com/ccc/index.php?id=113174) by Tom Kerrigan, CCC. May 31, 2000
- [Move ordering ideas](https://www.stmintz.com/ccc/index.php?id=157570) by Matt McKnight, CCC, March 08, 2001
- [Dynamic move ordering for capture/promotions?](https://www.stmintz.com/ccc/index.php?id=161474) by Scott Gasch, CCC, April 02, 2001
- [root search ordening](https://www.stmintz.com/ccc/index.php?id=167401) by Jan Willem de Kort, CCC, May 02, 2001
- [Move ordering at root of search](https://www.stmintz.com/ccc/index.php?id=168497) by Tom King, CCC, May 07, 2001
- [A SIMD idea, eg. Piece/Gain of a capture target](https://www.stmintz.com/ccc/index.php?id=343790) by Gerd Isenberg, CCC, January 21, 2004 » SSE2
- [move ordering and node count](https://www.stmintz.com/ccc/index.php?id=357188) by Martin Fierz, CCC, March 29, 2004
- [Move ordering at root](https://www.stmintz.com/ccc/index.php?id=358297) by Renze Steenhuisen, CCC, April 04, 2004
- [Fail-high on first move stat (86%)](https://www.stmintz.com/ccc/index.php?id=379624) by Michael Henderson, CCC, July 29, 2004 » Fail-High

## 2005 ...

- [Move ordering at different (Knuth) node types](http://www.open-aurec.com/wbforum/viewtopic.php?t=4384) by Tom Likens, Winboard Forum, February 21, 2006 » Node Types
- [move ordering statistic](http://www.open-aurec.com/wbforum/viewtopic.php?f=4&t=4447) by Andrew Shapira, Winboard Forum, March 03, 2006
- [SEE on non-capture moves in main search](http://www.talkchess.com/forum/viewtopic.php?t=12706) by Gary, CCC, March 28, 2007 » Static Exchange Evaluation
- [caps->noncaps vs. goodcaps->noncaps->badcaps](http://www.talkchess.com/forum/viewtopic.php?t=15198) by James Swafford, CCC, July 18, 2007
- [Move ordering: Delaying moves on the history phase](http://www.talkchess.com/forum/viewtopic.php?t=21119) by Miguel A. Ballicora, CCC, May 13, 2008

[Re: Move ordering: Delaying moves on the history phase](http://www.talkchess.com/forum/viewtopic.php?topic_view=threads&p=188924) by Lance Perkins, CCC, May 14, 2008

- [Order of implementing things](http://www.talkchess.com/forum/viewtopic.php?t=24294) by cyberfish, CCC, October 10, 2008
- [Negative Plausibility Move Ordering](http://www.talkchess.com/forum3/viewtopic.php?f=7&t=28873) by Alessandro Damiani, CCC, July 09, 2009

## 2010 ...

- [Move ordering help](http://www.talkchess.com/forum3/viewtopic.php?f=7&t=32611) by Richard Allbert, CCC, February 14, 2010 » Jabba
- [root move ordering](http://www.talkchess.com/forum/viewtopic.php?t=34655) by Edward Yu, CCC, June 02, 2010
- [Move ordering improvements](http://www.open-chess.org/viewtopic.php?f=5&t=661) by Howard E, Open Chess Programming Forum, September 26, 2010

2011

- [out of check move ordering](http://www.talkchess.com/forum/viewtopic.php?t=38387) by Don Dailey, CCC, March 12, 2011 » Check
- [The LBR move ordering heuristic](http://www.talkchess.com/forum/viewtopic.php?t=38556) by Steven Edwards, CCC, March 26, 2011 » Last Best Reply
- [Move ordering by PST](http://www.talkchess.com/forum/viewtopic.php?t=38766) by Onno Garms, CCC, April 16, 2011 » Piece-Square Tables, History Heuristic, Onno
- [Move ordering question](http://www.talkchess.com/forum/viewtopic.php?t=39338) by Stef Luijten, CCC, June 11, 2011
- [Root node search in Stockfish](http://www.talkchess.com/forum/viewtopic.php?t=39346) by Onno Garms, CCC, June 12, 2011 » Stockfish, Root
- [Improve Move Ordering for Alpha Beta](http://macechess.blogspot.de/2011/08/improve-move-ordering-for-alpha-beta.html) by Thomas Petzke, [mACE Chess](http://macechess.blogspot.de/), August 11, 2011

2012

- [Minimax/ Alpha beta pruning Move Ordering?](http://stackoverflow.com/questions/8906430/minimax-alpha-beta-pruning-move-ordering) by Felix, [Stack Overflow](https://en.wikipedia.org/wiki/Stack_Overflow), January 18, 2012
- [Move Ordering (Again :))](http://www.talkchess.com/forum3/viewtopic.php?f=7&t=42567) by Richard Allbert, CCC, February 22, 2012
- [Move ordering idea (old and new?)](http://www.talkchess.com/forum/viewtopic.php?t=44749) by Daniel Homan, CCC, August 09, 2012 » Countermove Heuristic
- [How effective is move ordering from TT?](http://www.talkchess.com/forum/viewtopic.php?t=44745) by Bill Henry, CCC, August 09, 2012 » Transposition Table
- [Relationship between move ordering and pruning](http://www.open-chess.org/viewtopic.php?f=5&t=2173) by Don Dailey, OpenChess Forum, December 17, 2012 » Pruning
- [Move Ordering?](http://www.talkchess.com/forum/viewtopic.php?t=46605) by Thomas Kolarik, CCC, December 28, 2012

2013

- [Killer and History: Increased Node Count](http://www.talkchess.com/forum/viewtopic.php?t=46886) by Cheney Nattress, CCC, January 15, 2013
- [Root move order (again)](http://www.talkchess.com/forum/viewtopic.php?t=47564) by Alberto Sanjuan, CCC, March 21, 2013
- [History pruning / move ordering question](http://www.talkchess.com/forum/viewtopic.php?t=47953) by Jerry Donald, CCC, May 10, 2013
- [Move ordering contest](http://www.talkchess.com/forum/viewtopic.php?t=48122) by Ed Schroder, CCC, May 26, 2013
- [An idea for move ordering at the root](http://www.talkchess.com/forum/viewtopic.php?t=48230) by Steven Edwards, CCC, June 09, 2013
- [MoveOrdering++](http://www.talkchess.com/forum/viewtopic.php?t=48981) by Henk van den Belt, CCC, August 16, 2013
- [How do you get a "Best first move" near the leaves](http://www.talkchess.com/forum/viewtopic.php?t=50339) by Henk van den Belt, CCC, December 05, 2013

2014

- [Idea of different history](http://www.talkchess.com/forum/viewtopic.php?t=51992) by Daniel José Queraltó, CCC, April 14, 2014 » History Heuristic
- [Effectiveness of killer moves](http://www.talkchess.com/forum/viewtopic.php?t=53317) by Alex Ferguson, CCC, August 17, 2014
- [Solving a fail low situation at the root](http://www.talkchess.com/forum/viewtopic.php?t=54241) by Alberto Sanjuan, CCC, November 03, 2014 » Aspiration Windows, Fail-Low, Root

## 2015 ...

- [Idea #8430: Optimizing move ordering, very slowly](http://www.talkchess.com/forum/viewtopic.php?t=55919) by Steven Edwards, CCC, April 06, 2015
- [Move ordering for cheapest refutation](http://www.talkchess.com/forum/viewtopic.php?t=57228) by Matthew Lai, CCC, August 09, 2015
- [Bonus for "null move SEE"](http://www.talkchess.com/forum/viewtopic.php?t=57346) by Matthew Lai, CCC, August 23, 2015
- [Q: Move ordering, checks](http://www.talkchess.com/forum/viewtopic.php?t=57479) by Harm Geert Muller, CCC, September 02, 2015
- [Ordering of Root moves and search instability !](http://www.talkchess.com/forum/viewtopic.php?t=58055) by Mahmoud Uthman, CCC, October 26, 2015 » Search Instability

2016

- [Sorting Captures](http://www.talkchess.com/forum/viewtopic.php?t=61021) by David Cimbalista, CCC, August 03, 2016
- [New killer idea](http://www.talkchess.com/forum/viewtopic.php?t=61260) by Alexandru Mosoi, CCC, August 28, 2016
- [Starting with move ordering](http://www.talkchess.com/forum/viewtopic.php?t=61262) by Luis Babboni, CCC, August 28, 2016
- [Best move statistics](http://www.talkchess.com/forum/viewtopic.php?t=61401) by Matthew Lai, CCC, September 12, 2016
- [Searching worse moves first](http://www.talkchess.com/forum/viewtopic.php?t=61420) by Matthew Lai, CCC, September 14, 2016
- [move ordering especially ordering&searching of root move](http://www.talkchess.com/forum/viewtopic.php?t=62558) by Mahmoud Uthman, CCC, December 21, 2016

2017

- [Move ordering ?](http://www.talkchess.com/forum/viewtopic.php?t=62827) by Mahmoud Uthman, CCC, January 15, 2017
- [Move ordering ?](http://www.talkchess.com/forum/viewtopic.php?t=63048) by Mahmoud Uthman, CCC, February 04, 2017
- [Sorting losing captures ?](http://www.talkchess.com/forum/viewtopic.php?t=63275) by Mahmoud Uthman, CCC, February 25, 2017
- [Move ordering statistics](http://www.talkchess.com/forum/viewtopic.php?t=63280) by Sander Maassen vd Brink, CCC, February 26, 2017
- [speed up or avoiding move sorting](http://www.talkchess.com/forum/viewtopic.php?t=63502) by Alexandru Mosoi, CCC, March 19, 2017 » Zurichess
- [Testing for Move Ordering Improvements](http://www.talkchess.com/forum/viewtopic.php?t=63555) by Cheney Nattress, CCC, March 25, 2017 » Engine Testing, Search Statistics
- [Sorting algorithms](http://www.talkchess.com/forum/viewtopic.php?t=63790) by Fermin Serrano, CCC, April 22, 2017
- [What is causing this problem?](http://www.talkchess.com/forum/viewtopic.php?t=64912) by Michael Sherwin, CCC, August 16, 2017 » RomiChess
- [Ordering Capture Moves](http://www.talkchess.com/forum/viewtopic.php?t=65084) by Jason Fernandez, CCC, September 06, 2017 » Move Ordering - Captures
- [Marginal hash move](http://www.talkchess.com/forum/viewtopic.php?t=65189) by Harm Geert Muller, CCC, September 16, 2017
- [Unordered moves phenomenon](http://www.talkchess.com/forum/viewtopic.php?t=65365) by Alvaro Cardoso, CCC, October 03, 2017
- [Move ordering](http://www.talkchess.com/forum/viewtopic.php?t=65675) by Matthew R. Brades, CCC, November 09, 2017

2018

- [Simple quiet move sorting](http://www.talkchess.com/forum/viewtopic.php?t=66312) by Andrew Grant, CCC, January 13, 2018
- [[Discussion] - Measuring move ordering](http://www.talkchess.com/forum/viewtopic.php?t=66684) by Ed Schroder, CCC, February 24, 2018

## 2020 ...

- [Idea in move ordering ...](http://www.talkchess.com/forum3/viewtopic.php?f=7&t=73088) by De Noose Daniel, CCC, February 14, 2020
- [sort every moves or pickNext](http://www.talkchess.com/forum3/viewtopic.php?f=7&t=73930) by Vivien Clauzon, CCC, May 14, 2020
- [Testing Move Order Quality](http://www.talkchess.com/forum3/viewtopic.php?f=7&t=74325) by Cheney, CCC, June 29, 2020
- [Using piece-square table score for move ordering](http://www.talkchess.com/forum3/viewtopic.php?f=7&t=74752) by Maksim Korzh, CCC, August 11, 2020 » Piece-Square Tables

2021

- [Sorting moves during move ordering](http://www.talkchess.com/forum3/viewtopic.php?f=7&t=76491) by Niels Abildskov, CCC, February 04, 2021
- [Hash move ordering vs. Hash cuts: savings in number of nodes visited](http://www.talkchess.com/forum3/viewtopic.php?f=7&t=76887) by Marcel Vanthoor, CCC, March 16, 2021 » Transposition Table
- [Best move from previous iteration first: still needed with TT?](http://www.talkchess.com/forum3/viewtopic.php?f=7&t=76888) by Marcel Vanthoor, CCC, March 16, 2021 » Hash Move, PV-Move
- [Move ordering heuristics for captures](http://www.talkchess.com/forum3/viewtopic.php?f=7&t=77152) by Niels Abildskov, CCC, April 23, 2021
- [Qsearch dynamic order besides MVV/LVA](http://www.talkchess.com/forum3/viewtopic.php?f=7&t=77380) by Aleks Peshkov, CCC, May 25, 2021 » Quiescence Search
- [PV-move ordering necessary if you have TT-move ordering?](http://www.talkchess.com/forum3/viewtopic.php?f=7&t=77593) by Marcel Vanthoor, CCC, July 01, 2021 » Hash Move, PV-Move
- [About move ordering and TT hitrate](https://www.talkchess.com/forum3/viewtopic.php?f=7&t=78474) by Giovanni Maria Manduca, CCC, October 22, 2021 » Transposition Table
- [Move ordering at the root](https://www.talkchess.com/forum3/viewtopic.php?f=7&t=78552) by Jonathan McDermid, CCC, October 30, 2021 » Root

2022

- [Move ordering using SEE](https://www.talkchess.com/forum3/viewtopic.php?f=7&t=79077) by Christian Dean, CCC, January 08, 2022 » SEE
- [Lazy sorting algorithm - Sorting on steroids](https://www.talkchess.com/forum3/viewtopic.php?f=7&t=79279) by JohnWoe, CCC, February 03, 2022
- [Having trouble understanding advanced move ordering techniques](https://www.talkchess.com/forum3/viewtopic.php?f=7&t=79342) by Pedro Duran, CCC, February 10, 2022
- [Failure of trivial approach to neural network move ordering](https://www.talkchess.com/forum3/viewtopic.php?f=7&t=79368) by Jost Triller, CCC, February 16, 2022 » Neural Networks

# External Links

- [Move Ordering in Rebel](https://web.archive.org/web/20120413083131/http://www.top-5000.nl/authors/rebel/chess840.htm#MOVE%20ORDERING) by Ed Schröder ([Wayback Machine](https://en.wikipedia.org/wiki/Wayback_Machine))
- Hiromi Uehara, [The Trio Project](http://www.cadoganhall.com/event/hiromi-the-trio-project/), feat. Anthony Jackson & Simon Phillips - Move, (2012), [YouTube](https://en.wikipedia.org/wiki/YouTube) Video

[Watch on YouTube](https://www.youtube.com/watch?v=1rxYw7Y45Eo)

# References

Up one level    Selim Akl, Monroe Newborn (1977). The Principal Continuation and the Killer Heuristic.1977 ACM Annual Conference Proceedings↩︎ Jonathan Schaeffer (1983). The History Heuristic. ICCA Journal, Vol. 6, No. 3↩︎ Mark Winands, Erik van der Werf, Jaap van den Herik, Jos Uiterwijk (2006). The Relative History Heuristic. CG 2004, [pdf](http://www.personeel.unimaas.nl/m-winands/documents/relhis.pdf)↩︎ [Move Ordering in Rebel](https://web.archive.org/web/20120413083131/http://www.top-5000.nl/authors/rebel/chess840.htm#MOVE%20ORDERING) by Ed Schröder ([Wayback Machine](https://en.wikipedia.org/wiki/Wayback_Machine)), also available as [pdf](http://members.home.nl/matador/Inside%20Rebel.pdf)↩︎ Jos Uiterwijk (1992). The Countermove Heuristic. ICCA Journal, Vol. 15, No. 1↩︎ Dap Hartmann (1988). Butterfly Boards. ICCA Journal, Vol. 11, Nos. 2/3↩︎ Levente Kocsis, Jos Uiterwijk, Jaap van den Herik (2001). Move Ordering using Neural Networks. IEA/AIE 2001, [LNCS](https://en.wikipedia.org/wiki/Lecture_Notes_in_Computer_Science) 2070↩︎ Levente Kocsis, Jos Uiterwijk, Eric Postma, Jaap van den Herik (2002). [The Neural MoveMap Heuristic in Chess](http://link.springer.com/chapter/10.1007/978-3-540-40031-8_11). CG 2002↩︎ Ingo Althöfer (1992). Move Ordering by Time Ordering. Correspondence, ICCA Journal, Vol. 15, No. 2↩︎ Jos Uiterwijk (1992). The Countermove Heuristic. ICCA Journal, Vol. 15, No. 1↩︎ [Re: random play](https://groups.google.com/d/msg/rec.games.chess.computer/AI3xadkLEIk/UUqnp9J3BaMJ) by Ronald de Man, rgcc, November 28, 1996↩︎ [Re: computer chess "oracle" ideas...](https://groups.google.com/d/msg/rec.games.chess.computer/me7GkjsEgds/iC_ZJm5UwswJ) by Ronald de Man, rgcc, April 7, 1997, see also [Re: mate threat extension/null move](https://www.stmintz.com/ccc/index.php?id=390268) by Don Beal, CCC, October 04, 2004 » Mate Threat Extensions, Null Move and WAC booster↩︎ [Bradley–Terry model from Wikipedia](https://en.wikipedia.org/wiki/Bradley%E2%80%93Terry_model)↩︎ [Re: Move ordering for cheapest refutation](http://www.talkchess.com/forum/viewtopic.php?t=57228&start=6) by Mikko Aarnos, CCC, August 10, 2015↩︎ [Lazy sorting algorithm - Sorting on steroids](https://www.talkchess.com/forum3/viewtopic.php?f=7&t=79279) by JohnWoe, CCC, February 03, 2022↩︎
