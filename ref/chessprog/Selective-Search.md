source: https://chessprogramming.org/Selectivity

# Selectivity

Home * Search * Selectivity

Selectivity gives a whiff of Shannon's Type B search to Shannon's Type A or brute-force. The goal is to search "interesting" and forced branches which are or are likely to become part of the principal variation deeper than nominal, but to reduce uninteresting branches.

# Extensions

- Botvinnik-Markoff Extension
- Capture Extensions
- Check Extensions
- Mate Threat Extensions
- One Reply Extensions
- Passed Pawn Extensions
- PV Extensions
- Recapture Extensions
- SEX Algorithm
- Singular Extensions

# Pruning

- AEL-Pruning
- Delta Pruning
- Enhanced Forward Pruning
- Futility Pruning
- History Leaf Pruning
- Mate Distance Pruning
- Move Count Based Pruning (Late Move Pruning)
- Multi-Cut
- Null Move Pruning
- Parity Pruning
- ProbCut
- Reverse Futility Pruning
- Uncertainty Cut-Offs

# Reductions

- Fail-High Reductions - FHR
- Late Move Reductions - LMR
- Null Move Reductions
- RankCut
- Razoring

# See also

- Bobby's Strategic Quiescence Search
- Parallelism and Selectivity in Game Tree Search | Video, Talk by Tord Romstad
- Quiescence Search
- Selective Search (Magazine)
- Selective Search versus Brute Force - Conference at WCCC 1986

# Publications

## 1970 ...

- Russell M. Church, Kenneth W. Church (1977). Plans, Goals, and Search Strategies for the Selection of a Move in Chess. Chess Skill in Man and Machine

## 1980 ...

- Hermann Kaindl (1983). Searching to Variable Depth in Computer Chess. Proceedings of [IJCAI 83](http://www.informatik.uni-trier.de/~ley/db/conf/ijcai/ijcai83.html), pp. 760-762. Karlsruhe. [pdf](http://ijcai.org/Past%20Proceedings/IJCAI-83-VOL-2/PDF/039.pdf)
- Don Beal (1986). Selective Search without Tears. ICCA Journal, Vol. 9, No. 2
- Hermann Kaindl, Helmut Horacek, Marcus Wagner (1986). Selective Search versus Brute Force. ICCA Journal, Vol. 9, No. 3
- David Levy, David Broughton, Mark Taylor (1989). The SEX Algorithm in Computer Chess. ICCA Journal, Vol. 12, No. 1

## 1990 ...

- Chun Ye (1992). Experiments in Selective Search Extensions. MSc. thesis, Department of Computing Science, University of Alberta, [pdf](https://era.library.ualberta.ca/public/datastream/get/uuid:e4fbf48d-7603-490f-85cc-5497bbecf5a8/DS1)
- Chun Ye, Tony Marsland (1992). Selective Extensions in Game-Tree Search. Heuristic Programming in AI 3
- David McAllester, Deniz Yuret (1993). Alpha-Beta Conspiracy Search. [ps (draft)](http://ttic.uchicago.edu/~dmcallester/abc.ps) » Alpha-Beta Conspiracy Search
- Javier Ros Padilla (1994). Estimating Asymmetry and Selectivity in Chess Programs. ICCA Journal, Vol. 17, No. 1 1
- Deniz Yuret (1994). [The Principle of Pressure in Chess](https://scholar.google.com/citations?view_op=view_citation&hl=en&user=EJurXJ4AAAAJ&cstart=40&citation_for_view=EJurXJ4AAAAJ:TQgYirikUcIC). TAINN 1994
- Lev Finkelstein, Shaul Markovitch (1998). [Learning to Play Chess Selectively by Acquiring Move Patterns.](http://www.cs.technion.ac.il/%7Eshaulm/papers/abstracts/Finkelstein-1998-LPC.html) ICCA Journal, Vol. 21, No. 2, [pdf](http://www.cs.technion.ac.il/%7Eshaulm/papers/pdf/Finkelstein-Markovitch-icca1998.pdf)
- Rainer Feldmann, Burkhard Monien (1998). [Selective Game Tree Search on a Cray T3E](http://www2.cs.uni-paderborn.de/fachbereich/AG/monien/PUBLICATIONS/ABSTRACTS/FM_T3E.html). [ps](http://www2.cs.uni-paderborn.de/fachbereich/AG/monien/PUBLICATIONS/POSTSCRIPTS/FM_T3E.ps.Z)

## 2000 ...

- Paul E. Utgoff, Richard P. Cochran (2000). [A Least-Certainty Heuristic for Selective Search](http://link.springer.com/chapter/10.1007/3-540-45579-5_1). CG 2000, [pdf](http://people.cs.umass.edu/~utgoff/papers/springer-lcf.pdf) » LCF
- Yngvi Björnsson, Tony Marsland (2000). Selective Depth-First Search Methods. in Jaap van den Herik, Hiroyuki Iida (eds.) (2000). Games in AI Research. Universiteit Maastricht, [pdf preprint](http://www.cs.ualberta.ca/%7Etony/RecentPapers/nec97w.pdf)
- Ulf Lorenz, Burkhard Monien (2002). [The Secret of Selective Game Tree Search, When Using Random-Error Evaluations](http://www.springerlink.com/content/f6b4wb6l63dpd0jv/). Proceedings of 19th Annual Symposium on Theoretical Aspects of Computer Science (STACS)
- David McAllester, Deniz Yuret (2002). Alpha-Beta Conspiracy Search. ICGA Journal, Vol. 25, No. 1 » Alpha-Beta Conspiracy Search
- Brian Sheppard (2004). [Efficient Control of Selective Simulations](https://link.springer.com/chapter/10.1007/11674399_1). CG 2004
- Brian Sheppard (2004). Efficient Control of Selective Simulations. ICGA Journal, Vol. 27, No. 2 2
- Pálmi Skowronski (2009). Gradual Focus: A Method for Automated Feature Discovery in Selective Search. M.Sc. thesis

## 2010 ...

- Omid David, Moshe Koppel, Nathan S. Netanyahu (2010). Optimizing Selective Search in Chess. ICML - Workshop on Machine Learning and Games
- Maarten Schadd (2011). Selective Search in Games of Different Complexity. Ph.D. Thesis. Department of Knowledge Engineering, Maastricht University

# Forum Posts

## 1998 ...

- [A new selective heuristic?](https://www.stmintz.com/ccc/index.php?id=21017) by Frank Schneider, CCC, June 21, 1998 » Reductions

## 2000 ...

- [Selective Searching](https://groups.google.com/group/rec.games.chess.computer/browse_frm/thread/7b66bdec7f729fa7) by Bob Durrett, rgcc, November 19, 2000
- [pruning vs extensions vs qsearch - are these all effectively the same?](https://www.stmintz.com/ccc/index.php?id=267678) by Scott Farrell, CCC, November 27, 2002
- [Evaluation-based Reductions and/or Extensions](https://www.stmintz.com/ccc/index.php?id=338851) by Tom Likens, CCC, December 28, 2003
- [extensions + reductions + pruning = confusion](https://www.stmintz.com/ccc/index.php?id=356488) by Johan de Koning, CCC, March 24, 2004 (was [Shredder 8 secret: search depth?](https://www.stmintz.com/ccc/index.php?id=356109))
- [Extension - Reductions and threats](http://www.open-aurec.com/wbforum/viewtopic.php?f=4&t=4820) by mjlef, Winboard Forum, May 17, 2006
- [Extensions/Reductions](http://www.talkchess.com/forum/viewtopic.php?t=29905) by Luca Hemmerich, CCC, September 28, 2009

## 2010 ...

- [Counting depth as a function of number of legal moves](http://www.talkchess.com/forum3/viewtopic.php?f=7&t=42677) by Pio Korinth, CCC, February 28, 2012 » Depth
- [Nullmove vs classic selective search](http://www.talkchess.com/forum/viewtopic.php?t=44686) by Ed Schröder, CCC, August 04, 2012 3
- [Houdini 3 reducing the depth feature](http://www.talkchess.com/forum/viewtopic.php?t=45624) by Maurizio Maglio, CCC, October 17, 2012 » Houdini
- [selective depth definition](http://www.talkchess.com/forum/viewtopic.php?t=51264) by Uri Blass, CCC, February 13, 2014 » Stockfish
- [Is modern chess software lossless or lossy?](http://www.talkchess.com/forum/viewtopic.php?t=66298) by Meni Rosenfeld, CCC, January 10, 2018 » Playing Strength
- [Names of selectivity algorithms](http://www.talkchess.com/forum3/viewtopic.php?f=7&t=68082) by Vivien Clauzon, CCC, July 26, 2018
- [delaying tactics: prune or extend?](http://www.talkchess.com/forum3/viewtopic.php?f=7&t=70165) by Harm Geert Muller, CCC, March 10, 2019 » Tactics

## 2020 ...

- [Tactical search](http://www.talkchess.com/forum3/viewtopic.php?f=7&t=74170) by Alvaro Cardoso, CCC, June 13, 2020 » Tactics

# External Links

- [Living Being Quintet](http://www.vincent-peirani.com/projets) - On the heights, [Altitude Jazz Festival 2015](http://www.altitudejazz.com/programme-concerts-festival-jazz-hautes-alpes.html), [YouTube](https://en.wikipedia.org/wiki/YouTube) Video

Vincent Peirani, Émile Parisien, Tony Paeleman, [Julien Herné](http://www.julienherne.com/), [Yoann Serra](http://www.simonoviez.com/english/YoannSerraus.htm)

[Watch on YouTube](https://www.youtube.com/watch?v=xoaSRUrL8lE)

# References

Up one level    [ICCA Journal, Vol. 17, No. 1](https://groups.google.com/d/msg/rec.games.chess/5_dMbe0_juo/bXQQVYVEpykJ) by Jos Uiterwijk, rgcc, May 02, 1994↩︎ slightly revised version of the CG 2004 paper↩︎ Selective Search Techniques in REBEL (introduction) from Programmer Corner by Ed Schröder↩︎
