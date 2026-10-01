source: https://chessprogramming.org/Search

# Search

Home * Search

Bernd Besser, Auf der Suche 1

---

1. [Schach Bilder Welten - Bernd Besser - Galerie](http://schachbilderwelten.tenners.de/html/galerie.html)↩︎

Because finding or guessing a good move in a chess position is hard to achieve statically, chess programs rely on some type of Search in order to play reasonably. Searching involves looking ahead at different move sequences and evaluating the positions after making the moves. Formally, searching a two-player [zero-sum](https://en.wikipedia.org/wiki/Zero-sum_%28game_theory%29) board game with [perfect information](https://en.wikipedia.org/wiki/Perfect_information) implies [traversing](https://en.wikipedia.org/wiki/Tree_traversal) and min-maxing a [tree-like data-structure](https://en.wikipedia.org/wiki/Tree_%28data_structure%29) by various [search algorithms](https://en.wikipedia.org/wiki/Search_algorithm).

# Shannon's Types

Claude Shannon categorized searches into two types 1 :

- Type A - a brute-force search looking at every variation to a given depth
- Type B - a selective search looking at "important" branches only

Inspired by the experiments of Adriaan de Groot 2 , Shannon and early programmers favored Type B strategy. Type B searches use some type of static heuristics in order to only look at branches that look important - with some risk to oversee some serious tactics not covered by the plausible move selector. Type B was most popular until the 1970's, when Type A programs had enough processing power and more efficient brute force algorithms to become stronger. Today most programs are closer to Type A, but have some characteristics of a Type B as mentioned in selectivity.

# The Search Tree

The search tree as subset of the search space is a [directed graph](https://en.wikipedia.org/wiki/Graph_%28mathematics%29#Directed_graph) of nodes, the alternating white and black to move chess positions - and edges connecting two nodes, representing the moves of either side. The root of the search-tree is the position we like to evaluate to find the best move. Because of transpositions due to different move sequences, nodes inside the tree may occur from various ancestors - even within a different number of moves. The search tree contains various cycles, since both sides may repeat a former position with the minimum of two reversible moves each, or four plies. Cycles are usually eliminated and not searched twice, which results in searching a [directed acyclic graph](https://en.wikipedia.org/wiki/Directed_acyclic_graph) DAG.

- Search Space
- Search Tree
- Root
- Node
- Conspiracy Numbers
- Move List
- Principal Variation
- Graph History Interaction
- Path-Dependency
- Repetitions
- Transposition
- Score
- Improving

# Search Algorithms

Most chess-programs use a variation of the alpha-beta algorithm to search the tree in a depth-first manner to attain an order of magnitude performance improvement over a pure minimax algorithm. Although move ordering doesnt affect the performance of a pure mini-max search (as all branches and nodes are searched) — it becomes crucial for the performance of alpha beta search and enhancements such as PVS, NegaScout and MTD(f). Hans Berliner's chess-program HiTech and Ulf Lorenz's program P.ConNerS used best-first approaches quite successfully.

## Depth-First Search

Depth-First search starts at the root and explores as far as possible along each branch before backtracking. Memory requirements are moderate, since only one path from the root to one leaf is kept in memory. The giga bytes of RAM in recent computers is utilized by a transposition table. Minimax and Negamax are mentioned for educational reasons as the prototypes for the more advanced algorithms. They otherwise have no practical relevance in software, except traversing a minimax tree inside a perft framework for testing the move generator. Depth-first algorithms are generally embedded inside an iterative deepening framework for time control and move ordering issues.

- Minimax
- Negamax
- Alpha-Beta

## Alpha-Beta Enhancements

### Obligatory

- Transposition Table
- Iterative Deepening
- Aspiration Windows

### Selectivity

- Quiescence Search
- Selectivity
- Mate Search

### Scout and Friends

- Scout
- NegaScout
- Principal Variation Search

### Alpha-Beta goes Best-First

- NegaC*
- MTD(f)
- Alpha-Beta Conspiracy Search

## Best-First Search

Best-First approaches build a search-tree by visiting the most promising nodes first. They usually have huge memory requirements, since they keep an exponentially growing search space in memory.

- B* as used by Hans Berliner's chess-program HiTech
- Best-First Minimax Search
- Conspiracy Number Search
- MCαβ
- Monte-Carlo Tree Search
- Proof-Number Search
- SSS* and Dual*
- UCT

# Opponent Model

- Opponent Model Search

# Parallel Search

- Parallel Search
- Parallel Controlled Conspiracy Number Search as used by Ulf Lorenz's program P.ConNerS

# Using Time

- Pondering
- Time Management

# Related Issues

- Depth
- Horizon Effect
- Iterative Search
- Move Ordering
- Search Explosion
- Search Instability
- Search Pathology
- Search Statistics
- Search with Random Leaf Values
- Theorem-Proving and M & N procedure

# See also

- Backtracking
- History of Computer Chess
- Knowledge

Search versus Evaluation

# Publications

## 1960 ...

- Daniel Edwards, Timothy Hart (1961). The Alpha-Beta Heuristic, AIM-030, reprint available from [DSpace](http://dspace.mit.edu/handle/1721.1/6098) at MIT
- Alexander Brudno (1963). Bounds and valuations for shortening the search of estimates. Problemy Kibernetiki (10) 141–150 and Problems of Cybernetics (10) 225–241
- James R. Slagle (1963). Game Trees, M & N Minimaxing, and the M & N alpha-beta procedure. Artificial Intelligence Group Report 3, UCRL-4671, University of California
- Jim Doran, Donald Michie (1966). [Experiments with the Graph Traverser Program](https://royalsocietypublishing.org/doi/10.1098/rspa.1966.0205). [Proceedings of the Royal Society](https://en.wikipedia.org/wiki/Proceedings_of_the_Royal_Society), Series A, Vol. 294, No. 1437, [pdf](https://stacks.stanford.edu/file/druid:yf330xx7624/yf330xx7624.pdf)
- James R. Slagle, Philip Bursky (1968). [Experiments With a Multipurpose, Theorem-Proving Heuristic Program](http://portal.acm.org/citation.cfm?id=321444). Journal of the ACM, Vol. 15, No. 1
- James R. Slagle, John K. Dixon (1969). [Experiments With Some Programs That Search Game Trees](http://portal.acm.org/citation.cfm?id=321510.321511). Journal of the ACM, Vol. 16, No. 2, [pdf](http://wiki.cs.pdx.edu/cs542-spring2011/nfp/abmin.pdf), [pdf](http://wiki.cs.pdx.edu/wurzburg2009/nfp/abmin.pdf)

## 1970 ...

- James R. Slagle, John K. Dixon (1970). [Experiments with the M & N Tree-Searching Program](http://portal.acm.org/citation.cfm?id=362052.362054). Communications of the ACM, Vol. 13, No. 3, pp. 147-154.
- James R. Slagle, Richard C. T. Lee (1971). [Application of game tree searching techniques to sequential pattern recognition](http://portal.acm.org/citation.cfm?id=362515.362562). Communications of the ACM, Vol. 14, No. 2
- Larry Harris (1973). The bandwidth heuristic search. [3. IJCAI 1973](http://dblp.uni-trier.de/db/conf/ijcai/ijcai73.html), [pdf](http://www.ijcai.org/Past%20Proceedings/IJCAI-73/PDF/004.pdf)
- Gerhard Wolf (1973). [Implementation of a dynamic tree searching algorithm in a chess programme](http://dl.acm.org/citation.cfm?id=805704). [Proceedings of the ACM annual conference](http://dl.acm.org/citation.cfm?id=800192&picked=prox)
- Larry Harris (1974). Heuristic Search under Conditions of Error. [Artificial Intelligence](https://en.wikipedia.org/wiki/Artificial_Intelligence_(journal)), Vol. 5, No. 3, also published (1977) under the title: The heuristic search: An alternative to the alpha-beta minimax procedure. Chess Skill in Man and Machine (ed. Peter W. Frey)
- Larry Harris (1975) The Heuristic Search And The Game Of Chess - A Study Of Quiescence, Sacrifices, And Plan Oriented Play. [IJCAI 1975](http://dblp.uni-trier.de/db/conf/ijcai/ijcai75.html), reprinted in Computer Chess Compendium
- Toshihide Ibaraki (1978). [Depth-m search in branch-and-bound algorithms](https://link.springer.com/article/10.1007/BF00991818). [International Journal of Parallel Programming](https://link.springer.com/journal/10766), Vol. 7, No. 4
- Georgy Adelson-Velsky, Vladimir Arlazarov, Mikhail Donskoy (1979). Algorithms of adaptive search. [Machine Intelligence 9](http://www.doc.ic.ac.uk/~shm/MI/mi9.html) (eds. Jean Hayes Michie, Donald Michie and L.I. Mikulich), Ellis Horwood, Chichester.
- George Stockman (1979). A Minimax Algorithm Better than Alpha-Beta? [Artificial Intelligence](https://en.wikipedia.org/wiki/Artificial_Intelligence_(journal)), Vol. 12, No. 2
- John Gaschnig (1979). [Performance Measurement and Analysis of Certain Search Algorithms](https://dl.acm.org/citation.cfm?id=909244). Ph.D. thesis, Carnegie Mellon University, [pdf](http://reports-archive.adm.cs.cmu.edu/anon/scan/CMU-CS-79-124.pdf)

## 1980 ...

- Judea Pearl (1981). Heuristic search theory: A survey of recent results. [IJCAI-81](http://www.informatik.uni-trier.de/%7Eley/db/conf/ijcai/ijcai81.html), [pdf](http://ijcai.org/Past%20Proceedings/IJCAI-81-VOL%201/PDF/100.pdf)
- Judea Pearl (1982). [The Solution for the Branching Factor of the Alpha-Beta Pruning Algorithm and its Optimality](http://portal.acm.org/citation.cfm?id=358616&dl=ACM&coll=DL&CFID=27355608&CFTOKEN=40935826). Communications of the ACM, Vol. 25, No. 8
- Murray Campbell, Tony Marsland (1983). A Comparison of Minimax Tree Search Algorithms. [Artificial Intelligence](https://en.wikipedia.org/wiki/Artificial_Intelligence_%28journal%29), Vol. 20, No. 4, [pdf](http://webdocs.cs.ualberta.ca/~tony/OldPapers/TR82-3.pdf)
- Andrew L. Reibman, Bruce W. Ballard (1983). [Non-Minimax Search Strategies for Use against Fallible Opponents](http://www.aaai.org/Library/AAAI/1983/aaai83-084.php). Proceedings of AAAI 83
- Nanda Srimani (1985). A New Algorithm (PS*) for Searching Game Trees. Master's thesis, University of Alberta
- Toshihide Ibaraki (1986). Generalization of Alpha-Beta and SSS* Search Procedures. [Artificial Intelligence](https://en.wikipedia.org/wiki/Artificial_Intelligence_%28journal%29), Vol. 29
- Tony Marsland, Nanda Srimani (1986). Phased State Search. [Fall Joint Computer Conference](http://www.informatik.uni-trier.de/~ley/db/conf/fjcc/fjcc86.html#MarslandS86), [pdf](http://webdocs.cs.ualberta.ca/~tony/OldPapers/fjcc.1986.pdf)
- Hermann Kaindl, Helmut Horacek, Marcus Wagner (1986). Selective Search versus Brute Force. ICCA Journal, Vol. 9, No. 3
- Ronald L. Rivest (1987). Game Tree Searching by Min/Max Approximation. [Artificial Intelligence](https://en.wikipedia.org/wiki/Artificial_Intelligence_(journal)), Vol. 34, No. 1, [pdf 1995](http://people.csail.mit.edu/rivest/Rivest-GameTreeSearchingByMinMaxApproximation.pdf)
- Bruce Abramson (1989). Control Strategies for Two-Player Games. ACM Computing Surveys, Vol. 21, No. 2, [pdf](http://www.theinformationist.com/pdf/constrat.pdf/)
- Stuart Russell, Eric Wefald (1989). [On optimal game-tree search using rational metareasoning](http://portal.acm.org/citation.cfm?id=1623807). In Proceedings of the Eleventh International Joint Conference on Artificial Intelligence, Detroit, MI: Morgan Kaufmann, [pdf](http://citeseerx.ist.psu.edu/viewdoc/download?doi=10.1.1.79.9229&rep=rep1&type=pdf)
- Liwu Li (1989). [Probabilistic Analysis of Search](https://doi.org/10.7939/R3VX06F26). Ph.D. thesis, University of Alberta, advisor Tony Marsland
- Wolfgang Nagl (1989). Best-Move Proving: A Fast Game Tree Searching Program. Heuristic Programming in AI 1

## 1990 ...

- Toshihide Ibaraki, Naoki Katoh (1990). [Searching Minimax Game Trees Under Memory Space Constraint](https://link.springer.com/article/10.1007/BF01531075). [Annals of Mathematics and Artificial Intelligence](https://link.springer.com/journal/10472), Vol. 1, Nos. 1-4
- Victor Allis, Maarten van der Meulen, Jaap van den Herik (1991). Proof-Number Search. Technical Reports in Computer Science, CS 91-01. Department of Computer Science, University of Limburg
- Tony Marsland (1992). Computer Chess and Search. Encyclopedia of Artificial Intelligence (2nd ed.) (ed. S.C. Shapiro) John Wiley & Sons, Inc. [pdf](http://webdocs.cs.ualberta.ca/~tony/RecentPapers/encyc.mac-1991.pdf) 3 4
- Eric B. Baum (1992). On Optimal Game Tree Propagation for Imperfect Players. AAAI-92, [pdf](http://www.aaai.org/Papers/AAAI/1992/AAAI92-078.pdf)
- Claude G. Diderich (1993). [A Bibliography on Minimax Trees](http://portal.acm.org/citation.cfm?id=165007). ACM SIGACT News, Vol. 24, No. 4
- Deniz Yuret (1994). [The Principle of Pressure in Chess](https://scholar.google.com/citations?view_op=view_citation&hl=en&user=EJurXJ4AAAAJ&cstart=40&citation_for_view=EJurXJ4AAAAJ:TQgYirikUcIC). TAINN 1994
- Hans Berliner, Chris McConnell (1995). B* Probability Based Search. Carnegie Mellon University Computer Science research report, Pittsburgh, PA, [postscript](http://www.cs.cmu.edu/afs/cs.cmu.edu/user/ccm/www/papers/BStar.ps)
- Claude G. Diderich, Marc Gengler (1995). [A Survey on Minimax Trees and Associated Algorithms](http://link.springer.com/chapter/10.1007/978-1-4613-3557-3_2). [Minimax and Its Applications](http://link.springer.com/book/10.1007/978-1-4613-3557-3). [Kluwer Academic Publishers](https://en.wikipedia.org/wiki/Springer_Science%2BBusiness_Media)
- Eric B. Baum, Warren D. Smith (1995). Best Play for Imperfect Players and Game Tree Search. Part 1 - Theory
- Eric B. Baum, Warren D. Smith (1995). Best Play for Imperfect Players and Game Tree Search. with pseudocode appendix by Charles Garrett, [ps](http://scorevoting.net/WarrenSmithPages/homepage/bpip1.ps)
- Warren D. Smith, Eric B. Baum, Charles Garrett, Rico Tudor (1995). Best Play for Imperfect Players and Game Tree Search. Part 2 - Experiments, [ps](http://scorevoting.net/WarrenSmithPages/homepage/bpip2.ps)
- Andrew N. Walker (1996). Hybrid Heuristic Search. ICCA Journal, Vol. 19, No. 1
- Ingo Althöfer (1997). On the k-best Mode in Computer Chess: Measuring the Similarity of Move Proposals. ICCA Journal, Vol. 20, No. 3
- Eric B. Baum, Warren D. Smith (1997). A Bayesian Approach to Relevance in Game Playing. [Artificial Intelligence](https://en.wikipedia.org/wiki/Artificial_Intelligence_%28journal%29), Vol. 97, [CiteSeerX](http://citeseer.ist.psu.edu/viewdoc/summary?doi=10.1.1.26.7961)
- Donald E. Knuth (1998). [The Art Of Computer Programming](http://www-cs-faculty.stanford.edu/%7Eknuth/taocp.html) Vol 3. Sorting and Searching, Second Edition, Addison-Wesley
- Wim Pijls, Arie de Bruin (1998). [Game Tree Algorithms and Solution Trees](http://link.springer.com/chapter/10.1007/3-540-48957-6_12). CG 1998

## 2000 ...

- Paul E. Utgoff, Richard P. Cochran (2000). [A Least-Certainty Heuristic for Selective Search](http://link.springer.com/chapter/10.1007/3-540-45579-5_1). CG 2000, [pdf](http://people.cs.umass.edu/~utgoff/papers/springer-lcf.pdf) » LCF
- Thomas Thomsen (2000). [Lambda-Search in Game Trees - with Application to Go](http://link.springer.com/chapter/10.1007/3-540-45579-5_2). CG 2000 also published in ICGA Journal, Vol. 23, No. 4, winning the 2001 ICGA Journal Award, [preprint as pdf](http://www.t-t.dk/publications/lambda_icga.pdf) » Lambda-Search
- Todd W. Neller (2000). Simulation-Based Search for Hybrid System Control and Analysis. Ph.D. thesis, Stanford University, advisor Richard Fikes, [pdf](http://cs.gettysburg.edu/~tneller/papers/neller-dissertation.pdf)
- Martin Müller (2001, 2002). Proof-Set Search. Technical Report TR 01-09, University of Alberta, CG 2002, [CiteSeerX](http://citeseerx.ist.psu.edu/viewdoc/summary?doi=10.1.1.20.9972) 5
- Thomas Lincke (2002). Exploring the Computational Limits of Large Exhaustive Search Problems. Ph.D thesis, ETH Zurich, [pdf](http://e-collection.library.ethz.ch/eserv/eth:25905/eth-25905-02.pdf) » Awari, Repetitions 6
- Steven Walczak (2003). [Knowledge-Based Search in Competitive Domains](http://portal.acm.org/citation.cfm?id=776752.776792&coll=DL&dl=GUIDE&CFID=34101495&CFTOKEN=18614940). IEEE Transactions on Knowledge and Data Engineering, Vol. 15, No. 3
- Arie de Bruin, Wim Pijls (2003). [Trends in game tree search](https://repub.eur.nl/pub/459). [Erasmus University, Rotterdam](https://en.wikipedia.org/wiki/Erasmus_University_Rotterdam)
- David Rasmussen (2004). Parallel Chess Searching and Bitboards. Masters thesis, [ps](http://www2.imm.dtu.dk/pubdb/views/edoc_download.php/3267/ps/imm3267.ps)
- Yan Radovilsky, Solomon Eyal Shimony (2004). Generalized Model for Rational Game Tree Search. [SMC 2004](https://dblp.uni-trier.de/db/conf/smc/smc2004-2.html), [pdf](https://www.cs.bgu.ac.il/~yanr/Publications/smc04.pdf) 7
- Markian Hlynka, Jonathan Schaeffer (2004). Pre-Searching. ICGA Journal, Vol. 27, No. 4
- Markian Hlynka, Jonathan Schaeffer (2005). [Automatic Generation of Search Engines](http://link.springer.com/chapter/10.1007/11922155_3). Advances in Computer Games 11
- Ulf Lorenz (2006). A new Implementation of Error Analysis in Game Trees. ICGA Journal, Vol. 29, No. 2
- Dmitry Batenkov (2006). Modern developments of Shannon’s Chess. [pdf](http://www.wisdom.weizmann.ac.il/~dmitryb/writing/chess_report.pdf)
- Pim Nijssen (2009). Using Intelligent Search Techniques to Play the Game Khet. Master's Thesis, Maastricht University, [pdf](http://www.personeel.unimaas.nl/pim.nijssen/pub/msc.pdf) 8
- Claude G. Diderich, Marc Gengler (2009). [Minimax Game Tree Searching](http://link.springer.com/referenceworkentry/10.1007%2F978-0-387-74759-0_370). [Encyclopedia of Optimization](http://link.springer.com/book/10.1007/978-0-387-74759-0), [Springer](https://en.wikipedia.org/wiki/Springer_Science%2BBusiness_Media)
- David Silver (2009). Reinforcement Learning and Simulation-Based Search. Ph.D. thesis, University of Alberta, [pdf](http://www0.cs.ucl.ac.uk/staff/D.Silver/web/Applications_files/thesis.pdf)

## 2010 ...

- Junichi Hashimoto (2011). A Study on Game-Independent Heuristics in Game-Tree Search. Ph.D. thesis, JAIST
- Hung-Jui Chang, Meng-Tsung Tsai, Tsan-sheng Hsu (2011). Game Tree Search with Adaptive Resolution. Advances in Computer Games 13, [pdf](https://www.conftool.net/acg13/index.php/Chang-Game_Tree_Search_with_Adaptive_Resolution-145.pdf?page=downloadPaper&filename=Chang-Game_Tree_Search_with_Adaptive_Resolution-145.pdf&form_id=145&form_version=final)
- Alexandru Godescu (2011). [Information and search in computer chess](http://arxiv.org/abs/1112.2149). 9
- Pim Nijssen, Mark Winands (2012). An Overview of Search Techniques in Multi-Player Games. ECAI CGW 2012
- Abdallah Saffidine, Tristan Cazenave (2012). A General Multi-Agent Modal Logic K Framework for Game Tree Search. ECAI CGW 2012
- Akihiro Kishimoto, Mark Winands, Martin Müller, Jahn-Takeshi Saito (2012). Game-Tree Search using Proof Numbers: The First Twenty Years. ICGA Journal, Vol. 35, No. 3
- Kuo-Yuan Kao, I-Chen Wu, Yi-Chang Shan, Shi-Jim Yen (2012). Selection Search for Mean and Temperature of Multi-branch Combinatorial Games. ICGA Journal, Vol. 35, No. 3
- David Silver, Richard Sutton, Martin Mueller (2013). Temporal-Difference Search in Computer Go. Proceedings of the [ICAPS-13 Workshop on Planning and Learning](http://icaps13.icaps-conference.org/technical-program/workshop-program/planning-and-learning/), [pdf](http://webdocs.cs.ualberta.ca/~sutton/papers/SSM-ICAPS-13.pdf)
- Jeff Rollason (2014). [Interest Search - Another way to do Minimax](http://www.aifactory.co.uk/newsletter/2014_01_interest_minimax.htm). AI Factory, Summer 2014
- Tom Holden (2014). Notes on an alternative approach to move choice in games such as Chess. [pdf](http://www.tholden.org/wp-content/uploads/2014/11/Notes-on-an-alternative-approach-to-move-choice-in-games-such-as-Chess.pdf) 10
- Christopher D. Rosin (2014). Game playing. [WIREs Cognitive Science](https://en.wikipedia.org/wiki/Wiley_Interdisciplinary_Reviews:_Cognitive_Science), Vol. 5, [pdf preprint](http://www.chrisrosin.com/Rosin-Game-Playing-submitted-ver.pdf)

## 2015 ...

- Jakub Pawlewicz, Ryan Hayward (2015). Feature Strength and Parallelization of Sibling Conspiracy Number Search. Advances in Computer Games 14
- Mohd Nor Akmal Khalid, Umi Kalsom Yusof, Hiroyuki Iida, Taichi Ishitobi (2015). [Critical Position Identiﬁcation in Games and Its Application to Speculative Play](https://www.researchgate.net/publication/281152992_Critical_Position_Identification_in_Games_and_Its_Application_to_Speculative_Play). [ICAART 2015](http://www.scitepress.org/DigitalLibrary/ProceedingsDetails.aspx?ID=+mGlly8Sp00=&t=1)
- Mohd Nor Akmal Khalid, E. Mei Ang, Umi Kalsom Yusof, Hiroyuki Iida, Taichi Ishitobi (2015). [Identifying Critical Positions Based on Conspiracy Numbers](http://link.springer.com/chapter/10.1007%2F978-3-319-27947-3_6). [Agents and Artificial Intelligence](http://link.springer.com/book/10.1007/978-3-319-27947-3), [ICAART 2015 - Revised Selected Papers](http://dblp.uni-trier.de/db/conf/icaart/icaart2015s.html#KhalidAYII15)
- Ting-Han Wei, Chao-Chin Liang, I-Chen Wu, Lung-Pin Chen (2015). Software Development Framework for Job-Level Algorithms. ICGA Journal, Vol. 38, No. 3
- Tobias Joppen, Miriam Moneke, Nils Schroder, Christian Wirth, Johannes Fürnkranz (2017). [Informed Hybrid Game Tree Search for General Video Game Playing](http://ieeexplore.ieee.org/document/7970136/). IEEE Transactions on Computational Intelligence and AI in Games, Vol. PP, No. 99
- Michael Hartisch, Ulf Lorenz (2019). A Novel Application for Game Tree Search - Exploiting Pruning Mechanisms for Quantified Integer Programs. Advances in Computer Games 16

## 2020 ...

- Quentin Cohen-Solal, Tristan Cazenave (2020). Minimax Strikes Back. [arXiv:2012.10700](https://arxiv.org/abs/2012.10700) » Reinforcement Learning
- Quentin Cohen-Solal (2021). Completeness of Unbounded Best-First Game Algorithms. [arXiv:2109.09468](https://arxiv.org/abs/2109.09468)

# Forum Posts

## 1999

- [Some crazy ideas](https://www.stmintz.com/ccc/index.php?id=47379) by Gareth McCaughan, CCC, March 29, 1999

## 2000 ...

- [About search algorithms and heuristics](https://www.stmintz.com/ccc/index.php?id=112586) by José Carlos, CCC, May 26, 2000
- [Search algorithms and effeciency](https://www.stmintz.com/ccc/index.php?id=172496) by Kim Roper Jensen, CCC, May 30, 2001
- [Search algorithms in chess programs](https://www.stmintz.com/ccc/index.php?id=201686) by Russell Reagan, CCC, December 12, 2001
- [Search algorithms](https://www.stmintz.com/ccc/index.php?id=325977) by Renze Steenhuisen, CCC, November 06, 2003
- [Game tree search algorithms](https://www.stmintz.com/ccc/index.php?id=343686) by Russell Reagan, CCC, January 20, 2004

## 2005 ...

- [Search questions](http://www.open-aurec.com/wbforum/viewtopic.php?f=4&t=6665) by Sven Schüle, Winboard Forum, July 17, 2007 » Fail-Soft, Principal Variation Search
- [Even more search questions](http://www.open-aurec.com/wbforum/viewtopic.php?f=4&t=6666) by Sven Schüle, Winboard Forum, July 17, 2007 » Root, Iterative Deepening
- [Search or Evaluation?](http://www.hiarcs.net/forums/viewtopic.php?t=402) by Ed Schröder, Hiarcs Forum, October 05, 2007 » Search versus Evaluation, Evaluation

[Re: Search or Evaluation?](http://www.hiarcs.net/forums/viewtopic.php?p=2944) by Mark Uniacke, Hiarcs Forum, October 14, 2007

- [Efficient algorithm for k-best mode?](http://www.talkchess.com/forum/viewtopic.php?t=17921) by Gijsbert Wiesenekker, CCC, November 17, 2007

## 2010 ...

- [Scaling at 2x nodes (or doubling time control).](http://www.talkchess.com/forum/viewtopic.php?t=48733) by Kai Laskos, CCC, July 23, 2013 » Doubling TC, Diminishing Returns, Playing Strength, Houdini
- [Is search irrelevant when computing ahead of very big trees?](http://www.talkchess.com/forum/viewtopic.php?t=48743) by Fermin Serrano, CCC, July 24, 2013 » Knowledge
- [Improve the search or the evaluation?](http://www.talkchess.com/forum/viewtopic.php?t=49190) by Jens Bæk Nielsen, CCC, August 31, 2013 » Evaluation, Search versus Evaluation
- [Slow Searchers?](http://www.talkchess.com/forum/viewtopic.php?t=49906) by Michael Neish, CCC, November 02, 2013
- [A new algorithm accounting for the uncertainty in eval funcs](http://www.talkchess.com/forum/viewtopic.php?t=54324) by Tom Holden, CCC, November 12, 2014

## 2015 ...

- [Search algorithm in it's simplest forum](http://www.talkchess.com/forum/viewtopic.php?t=55474) by Mahmoud Uthman, CCC, February 25, 2015 » Alpha-Beta, Quiescence Search
- [Time assignment to children](http://www.talkchess.com/forum/viewtopic.php?t=57092) by Matthew Lai, CCC, July 26, 2015
- [Some musings about search](http://www.talkchess.com/forum/viewtopic.php?t=57270) by Ed Schroder, CCC, August 14, 2015 » Automated Tuning
- [Search](http://www.talkchess.com/forum/viewtopic.php?t=60581) by Laurie Tunnicliffe, CCC, June 24, 2016
- [Searching using slow eval with tactical verification](http://www.talkchess.com/forum/viewtopic.php?t=61348) by Matthew Lai, CCC, September 06, 2016
- [Root search](http://www.talkchess.com/forum/viewtopic.php?t=61358) by Laurie Tunnicliffe, CCC, September 08, 2016 » Root
- [Doubling of time control](http://www.talkchess.com/forum/viewtopic.php?t=61784) by Andreas Strangmüller, CCC, October 21, 2016 » Doubling TC, Diminishing Returns, Playing Strength, Komodo
- [search efficiency](http://www.talkchess.com/forum/viewtopic.php?t=64390) by Marco Pampaloni, CCC, June 23, 2017
- [comparing between search or evaluation](http://www.talkchess.com/forum/viewtopic.php?t=65403) by Uri Blass, CCC, October 09, 2017 » Evaluation

## 2020 ...

- [Tactical search](http://www.talkchess.com/forum3/viewtopic.php?f=7&t=74170) by Alvaro Cardoso, CCC, June 13, 2020 » Tactics
- [Listening for GUI input when searching](http://www.talkchess.com/forum3/viewtopic.php?f=7&t=77189) by Niels Abildskov, CCC, April 27, 2021 » GUI, Thread, UCI
- [On reaching maximum ply](http://www.talkchess.com/forum3/viewtopic.php?f=7&t=77202) by Martin Bryant, CCC, April 29, 2021 » Maximum Search Depth, Ply
- [Search](https://www.talkchess.com/forum3/viewtopic.php?f=7&t=78505) by Vincent Diepeveen, CCC, October 26, 2021
- [Strategies to unit testing the search](https://www.talkchess.com/forum3/viewtopic.php?f=7&t=79276) by Olexiy Svitashev, CCC, February 03, 2022 » Engine Testing

# External Links

- [Search algorithm from Wikipedia](https://en.wikipedia.org/wiki/Search_algorithm)
- [Combinatorial search from Wikipedia](https://en.wikipedia.org/wiki/Combinatorial_search)
- [Depth-first search from Wikipedia](https://en.wikipedia.org/wiki/Depth-first_search)
- [Boost Graph Library: Depth-First Search](http://www.boost.org/doc/libs/1_42_0/libs/graph/doc/depth_first_search.html)
- [Best-first search from Wikipedia](https://en.wikipedia.org/wiki/Best-first_search)

[A* search algorithm from Wikipedia](https://en.wikipedia.org/wiki/A*_search_algorithm)

- [Breadth-first search from Wikipedia](https://en.wikipedia.org/wiki/Breadth-first_search)
- [Dijkstra's algorithm from Wikipedia](https://en.wikipedia.org/wiki/Dijkstra%27s_algorithm)
- [Lambda-search Java-code (version 2.0)](http://www.t-t.dk/go/cg2000/code20.html) by Thomas Thomsen
- [Chess Programming Part IV: Basic Search](http://www.gamedev.net/page/resources/_/technical/artificial-intelligence/chess-programming-part-iv-basic-search-r1171) by François-Dominic Laramée, [gamedev.net](https://en.wikipedia.org/wiki/GameDev.net), Ausgust 2000
- [Chess Programming Part V: Advanced Search](http://www.gamedev.net/page/resources/_/technical/artificial-intelligence/chess-programming-part-v-advanced-search-r1197) by François-Dominic Laramée, [gamedev.net](https://en.wikipedia.org/wiki/GameDev.net), September 2000
- [Engine - Hispanic Chess Engines | The search function](https://sites.google.com/site/hispanicchessengines/programs--interface---engines/engine) by Pedro Castro
- [An Introduction to Game Tree Algorithms](http://hamedahmadi.com/gametree/) by Hamed Ahmadi
- Miroslav Vitouš - [Infinite Search](https://de.wikipedia.org/wiki/Infinite_Search) (1969), [YouTube](https://en.wikipedia.org/wiki/YouTube) Video

Jack DeJohnette, John McLaughlin, Herbie Hancock, Joe Henderson

[Watch on YouTube](https://www.youtube.com/watch?v=-OdIEbFwQEs)

# References

Up one Level    Claude Shannon (1949). [Programming a Computer for Playing Chess](http://www.pi.infn.it/%7Ecarosi/chess/shannon.txt). [pdf](http://archive.computerhistory.org/projects/chess/related_materials/text/2-0%20and%202-1.Programming_a_computer_for_playing_chess.shannon/2-0%20and%202-1.Programming_a_computer_for_playing_chess.shannon.062303002.pdf)↩︎ Adriaan de Groot (1946). Het denken van den Schaker, een experimenteel-psychologische studie. Ph.D. thesis, [University of Amsterdam](https://en.wikipedia.org/wiki/University_of_Amsterdam); N.V. Noord-Hollandse Uitgevers Maatschappij, [Amsterdam](https://en.wikipedia.org/wiki/Amsterdam). Translated with the help of George Baylor, with additions (in 1965) as Thought and Choice in Chess. Mouton Publishers, The Hague. ISBN 90-279-7914-6. ([amazon](https://www.amazon.com/gp/reader/9027979146/ref=sib_dp_pt#reader-link))↩︎ [Excellent Computer-Chess Overview Paper Found!](https://groups.google.com/group/rec.games.chess.computer/browse_frm/thread/7df61a100528f201) by Ernst A. Heinz, rgcc, March 6, 1997↩︎ [Great article for people who wants to write a chess engine](https://www.stmintz.com/ccc/index.php?id=221364) by Miguel A. Ballicora, CCC, April 03, 2002↩︎ [Re: A new(?) technique to recognize draws](https://www.stmintz.com/ccc/index.php?id=233322) by Dan Andersson, June 01, 2002↩︎ [Re: Aquarium IDEA, repetitions, and minimax over cycles](http://www.open-chess.org/viewtopic.php?f=5&t=2093#p17469) by syzygy, OpenChess Forum, September 22, 2012↩︎ [Re: Interesting ideas](http://www.talkchess.com/forum/viewtopic.php?t=57560&start=14) by Karlo Bala Jr., CCC, September 09, 2015↩︎ [Khet (game) from Wikipedia](https://en.wikipedia.org/wiki/Khet_%28game%29)↩︎ [Information and search in computer chess (Godescu)](http://www.open-chess.org/viewtopic.php?f=5&t=1736) by BB+, OpenChess Forum, December 12, 2011↩︎ [A new algorithm accounting for the uncertainty in eval funcs](http://www.talkchess.com/forum/viewtopic.php?t=54324) by Tom Holden, CCC, November 12, 2014↩︎
