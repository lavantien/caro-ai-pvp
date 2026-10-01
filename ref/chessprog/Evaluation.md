source: https://chessprogramming.org/Evaluation

# Evaluation

Home * Evaluation

Wassily Kandinsky - Graceful Ascent, 1934 1

---

1. [File:Kandinsky - Graceful Ascent, 1934.jpg - Wikimedia Commons](https://commons.wikimedia.org/wiki/File:Kandinsky_-_Graceful_Ascent,_1934.jpg)↩︎

Evaluation,
 a [heuristic function](https://en.wikipedia.org/wiki/Heuristic_(computer_science)) to determine the relative value of a position, i.e. the chances of winning. If we could see to the end of the game in every line, the evaluation would only have values of -1 (loss), 0 (draw), and 1 (win), and the chess engine should search depth 1 only to get the best move. In practice, however, we do not know the exact value of a position, so we must make an approximation with the main purpose is to comparing positions, and the chess engine now must search deeply and find the highest score position within a given period.

Recently, there are two main ways to build an evaluation: traditional hand-crafted evaluation (HCE) and multi-layer neural networks. This page focuses on the traditional way of considering explicit features of a chess position.

Beginning chess players learn to do this starting with the value of the pieces themselves. Computer evaluation functions also use the value of the material balance as the most significant aspect and then add other considerations.

# Where to Start

The first thing to consider when writing an evaluation function is how to score a move in Minimax or the more common NegaMax framework. While Minimax usually associates the white side with the max-player and black with the min-player and always evaluates from the white point of view, NegaMax requires a symmetric evaluation in relation to the side to move. We can see that one must not score the move per se – but the result of the move (i.e. a positional evaluation of the board as a result of the move). Such a symmetric evaluation function was first formulated by Claude Shannon in 1949 1 :

```
f(p) = 200(K-K')
       + 9(Q-Q')
       + 5(R-R')
       + 3(B-B' + N-N')
       + 1(P-P')
       - 0.5(D-D' + S-S' + I-I')
       + 0.1(M-M') + ...

KQRBNP = number of kings, queens, rooks, bishops, knights and pawns
D,S,I = doubled, blocked and isolated pawns
M = Mobility (the number of legal moves)
```

Here, we can see that the score is returned as a result of subtracting the current side's score from the equivalent evaluation of the opponent's board scores (indicated by the prime letters K' Q' and R'.. ).

## Side to move relative

In order for NegaMax to work, it is important to return the score relative to the side being evaluated. For example, consider a simple evaluation, which considers only material and mobility:

```
materialScore = kingWt  * (wK-bK)
              + queenWt * (wQ-bQ)
              + rookWt  * (wR-bR)
              + knightWt* (wN-bN)
              + bishopWt* (wB-bB)
              + pawnWt  * (wP-bP)

mobilityScore = mobilityWt * (wMobility-bMobility)
```

return the score relative to the side to move (who2Move = +1 for white, -1 for black):

```
Eval  = (materialScore + mobilityScore) * who2Move
```

## Linear vs. Nonlinear

Most evaluations terms are a [linear combination](https://en.wikipedia.org/wiki/Linear_combination) of independent features and associated weights in the form of  A function f is [linear](https://en.wikipedia.org/wiki/Linear) if the function is [additive](https://en.wikipedia.org/wiki/Additive_function):  and second if the function is [homogeneous](https://en.wikipedia.org/wiki/Homogeneous_function) of degree 1:

It depends on the definition and [independence](https://en.wikipedia.org/wiki/Linear_independence) of features and the acceptance of the [axiom of choice](https://en.wikipedia.org/wiki/Axiom_of_choice) (Ernst Zermelo 1904), whether additive real number functions are linear or not 2. Features are either related to single pieces (material), their location (piece-square tables), or more sophisticated, considering interactions of multiple pawns and pieces, based on certain patterns or chunks. Often several phases to first process simple features and after building appropriate data structures, in consecutive phases more complex features based on patterns and chunks are used.

Based on that, to distinguish [first-order](https://en.wikipedia.org/wiki/First-order), [second-order](https://en.wikipedia.org/wiki/Second-order), etc. terms, makes more sense than using the arbitrary terms linear vs. nonlinear evaluation 3. With respect to tuning, one has to take care that features are independent, which is not always that simple. Hidden dependencies may otherwise make the evaluation function hard to maintain with undesirable nonlinear effects.

## General Aspects

- Evaluation Philosophy
- Pawn Advantage, Win Percentage, and Elo
- Value Range

# Basic Evaluation Features

- Material
- Piece-Square Tables
- Pawn Structure
- Evaluation of Pieces
- Evaluation Patterns
- Mobility
- Center Control
- Connectivity
- Trapped Pieces
- King Safety
- Space
- Tempo

# Considering Game Phase

- Game Phases

Opening

Middlegame

Endgame

- Evaluation Discontinuity
- Tapered Eval (a score is interpolated between opening and endgame based on game stage/pieces)

# Miscellaneous

- Analog Evaluation
- Asymmetric Evaluation
- Automated Tuning
- Evaluation Function
- Evaluation Function Draft
- Evaluation Hash Table
- Evaluation Overlap by Mark Watkins
- Evaluation Patterns
- Lazy Evaluation
- Quantifying Evaluation Features by Mark Watkins
- Simplified Evaluation Function
- PeSTO's Evaluation Function

# See also

- CPW-Engine_eval - an example of a medium strength evaluation function
- Entropy in Papa
- Evaluation in Kaissa (PC)
- Evaluation in Rookie 2.0
- Knowledge

Search versus Evaluation

- Learning
- NNUE
- Oracle
- Point Value
- Search with Random Leaf Values
- Stockfish Evaluation Guide

# Publications

## 1949

- Claude Shannon (1949). [Programming a Computer for Playing Chess](http://www.pi.infn.it/%7Ecarosi/chess/shannon.txt). [pdf](http://archive.computerhistory.org/projects/chess/related_materials/text/2-0%20and%202-1.Programming_a_computer_for_playing_chess.shannon/2-0%20and%202-1.Programming_a_computer_for_playing_chess.shannon.062303002.pdf) from The Computer History Museum

## 1950 ...

- Eliot Slater (1950). Statistics for the Chess Computer and the Factor of Mobility, Proceedings of the Symposium on Information Theory, London. Reprinted 1988 in Computer Chess Compendium, pp. 113-117. Including the transcript of a discussion with Alan Turing and Jack Good
- Alan Turing (1953). Chess. part of the collection Digital Computers Applied to Games, in [Bertram Vivian Bowden](https://en.wikipedia.org/wiki/B._V._Bowden,_Baron_Bowden) (editor), [Faster Than Thought](http://www.computinghistory.org.uk/cgi-bin/sitewise.pl?act=det&p=10719), a symposium on digital computing machines, reprinted 1988 in Computer Chess Compendium, reprinted 2004 in The Essential Turing, [google books](https://books.google.com/books?id=RSkxnKlv1D4C&lpg=PP882&ots=VOWmiIm_lD&dq=Turochamp%2C%20chess&pg=PP881#v=onepage&q&f=true)

## 1960 ...

- [Israel Albert Horowitz](https://en.wikipedia.org/wiki/Israel_Albert_Horowitz), [Geoffrey Mott-Smith](https://en.wikipedia.org/wiki/Mott-Smith_Trophy) (1960,1970,2012). Point Count Chess. [Samuel Reshevsky](https://en.wikipedia.org/wiki/Samuel_Reshevsky) (Introduction), Sam Sloan (2012 Introduction), [Amazon](https://www.amazon.com/Point-Count-Chess-Accurate-Winning/dp/4871874699/ref=sr_1_2?s=books&ie=UTF8&qid=1366734801&sr=1-2) 4
- Jack Good (1968). A Five-Year Plan for Automatic Chess. Machine Intelligence II pp. 110-115

## 1970 ...

- Ron Atkin (1972). Multi-Dimensional Structure in the Game of Chess. In International Journal of Man-Machine Studies, Vol. 4
- Ron Atkin, Ian H. Witten (1975). [A Multi-Dimensional Approach to Positional Chess](http://www.bibsonomy.org/bibtex/2b91106ea980eb48aa505f6b54c130707/dblp). International Journal of Man-Machine Studies, Vol. 7, No. 6
- Gerard Zieliński (1976). [Simple Evaluation Function](http://www.emeraldinsight.com/doi/abs/10.1108/eb005425). [Kybernetes](http://www.emeraldinsight.com/loi/k), Vol. 5, No. 3
- Ron Atkin (1977). Positional Play in Chess by Computer. Advances in Computer Chess 1
- David Slate, Larry Atkin (1977). CHESS 4.5 - The Northwestern University Chess Program. Chess Skill in Man and Machine (ed. Peter W. Frey), pp. 82-118. Springer-Verlag, New York, N.Y. 2nd ed. 1983. ISBN 0-387-90815-3. Reprinted (1988) in Computer Chess Compendium
- Hans Berliner (1979). [On the Construction of Evaluation Functions for Large Domains](http://www.bkgm.com/articles/Berliner/EvaluationFunctionsLargeDomains/). [IJCAI 1979](http://www.informatik.uni-trier.de/%7Eley/db/conf/ijcai/index.html) Tokyo, Vol. 1, pp. 53-55.

## 1980 ...

- Helmut Horacek (1984). Some Conceptual Defects of Evaluation Functions. [ECAI-84](http://dl.acm.org/citation.cfm?id=537320), [Pisa](https://en.wikipedia.org/wiki/Pisa), [Elsevier](https://en.wikipedia.org/wiki/Elsevier)
- Peter W. Frey (1985). An Empirical Technique for Developing Evaluation Functions. ICCA Journal, Vol. 8, No. 1
- Tony Marsland (1985). Evaluation-Function Factors. ICCA Journal, Vol. 8, No. 2, [pdf](http://webdocs.cs.ualberta.ca/~tony/OldPapers/evaluation.pdf)
- Jens Christensen, Richard Korf (1986). A Unified Theory of Heuristic Evaluation functions and Its Applications to Learning. Proceedings of the [AAAI-86](http://www.aaai.org/Conferences/AAAI/aaai86.php), pp. 148-152, [pdf](http://www.aaai.org/Papers/AAAI/1986/AAAI86-023.pdf)
- Dap Hartmann (1987). How to Extract Relevant Knowledge from Grandmaster Games. Part 1: Grandmasters have Insights - the Problem is what to Incorporate into Practical Problems. ICCA Journal, Vol. 10, No. 1
- Dap Hartmann (1987). How to Extract Relevant Knowledge from Grandmaster Games. Part 2: the Notion of Mobility, and the Work of De Groot and Slater. ICCA Journal, Vol. 10, No. 2
- Bruce Abramson, Richard Korf (1987). A Model of Two-Player Evaluation Functions. [AAAI-87](http://www.aaai.org/Conferences/AAAI/aaai87.php). [pdf](http://www.aaai.org/Papers/AAAI/1987/AAAI87-016.pdf)
- Kai-Fu Lee, Sanjoy Mahajan (1988). [A Pattern Classification Approach to Evaluation Function Learning](http://www.sciencedirect.com/science/article/pii/0004370288900768). [Artificial Intelligence](https://en.wikipedia.org/wiki/Artificial_Intelligence_%28journal%29), Vol. 36, No. 1
- Dap Hartmann (1989). Notions of Evaluation Functions Tested against Grandmaster Games. Advances in Computer Chess 5
- Maarten van der Meulen (1989). Weight Assessment in Evaluation Functions. Advances in Computer Chess 5
- Bruce Abramson (1989). On Learning and Testing Evaluation Functions. Proceedings of the Sixth Israeli Conference on Artificial Intelligence, 1989, 7-16.
- Danny Kopec, Ed Northam, David Podber, Yehya Fouda (1989). The Role of Connectivity in Chess. Workshop on New Directions in Game-Tree Search, [pdf](http://www.sci.brooklyn.cuny.edu/%7Ekopec/Publications/Publications/O_24_C.pdf)

## 1990 ...

- Bruce Abramson (1990). On Learning and Testing Evaluation Functions. Journal of Experimental and Theoretical Artificial Intelligence 2: 241-251.
- Ron Kalnim (1990). A Positional Assembly Model. ICCA Journal, Vol. 13, No. 3
- Paul E. Utgoff, [Jeffery A. Clouse](http://dblp.uni-trier.de/pers/hd/c/Clouse:Jeffery_A=) (1991). [Two Kinds of Training Information for Evaluation Function Learning](http://scholarworks.umass.edu/cs_faculty_pubs/193/). [University of Massachusetts, Amherst](https://en.wikipedia.org/wiki/University_of_Massachusetts_Amherst), Proceedings of the AAAI 1991
- Ingo Althöfer (1991). An Additive Evaluation Function in Chess. ICCA Journal, Vol. 14, No. 3
- Ingo Althöfer (1993). On Telescoping Linear Evaluation Functions. ICCA Journal, Vol. 16, No. 2 5
- Alois Heinz, Christoph Hense (1993). [Bootstrap learning of α-β-evaluation functions](http://citeseerx.ist.psu.edu/viewdoc/summary?doi=10.1.1.56.872). [ICCI 1993](http://dblp.uni-trier.de/db/conf/icci/icci1993.html#HeinzH93), [pdf](http://citeseerx.ist.psu.edu/viewdoc/download?doi=10.1.1.56.872&rep=rep1&type=pdf)
- Alois Heinz (1994). [Efficient Neural Net α-β-Evaluators](http://citeseerx.ist.psu.edu/viewdoc/summary?doi=10.1.1.55.3994). [pdf](http://citeseerx.ist.psu.edu/viewdoc/download?doi=10.1.1.55.3994&rep=rep1&type=pdf) 6
- Peter Mysliwietz (1994). Konstruktion und Optimierung von Bewertungsfunktionen beim Schach. Ph.D. Thesis (German)
- Don Beal, Martin C. Smith (1994). Random Evaluations in Chess. ICCA Journal, Vol. 17, No. 1
- Yaakov HaCohen-Kerner (1994). [Case-Based Evaluation in Computer Chess](http://www.springerlink.com/content/f5n27h25q4l920q8/). [EWCBR 1994](http://www.informatik.uni-trier.de/~ley/db/conf/ewcbr/ewcbr1994.html#Kerner94)
- Michael Buro (1995). [Statistical Feature Combination for the Evaluation of Game Positions](http://www.jair.org/papers/paper179.html). [JAIR](https://en.wikipedia.org/wiki/Journal_of_Artificial_Intelligence_Research), Vol. 3
- Peter Mysliwietz (1997). A Metric for Evaluation Functions. Advances in Computer Chess 8
- Michael Buro (1998). [From Simple Features to Sophisticated Evaluation Functions](http://link.springer.com/chapter/10.1007/3-540-48957-6_8). CG 1998, [pdf](https://skatgame.net/mburo/ps/glem.pdf)

## 2000 ...

- Dan Heisman (2003). Evaluation Criteria, [pdf](http://www.chesscafe.com/text/heisman27.pdf) from [ChessCafe.com](https://en.wikipedia.org/wiki/ChessCafe.com)
- Jeff Rollason (2005). [Evaluation by Hill-climbing: Getting the right move by solving micro-problems](http://www.aifactory.co.uk/newsletter/2005_03_hill-climbing.htm). AI Factory, Autumn 2005 » Automated Tuning
- Shogo Takeuchi, Tomoyuki Kaneko, Kazunori Yamaguchi, Satoru Kawai (2007). Visualization and Adjustment of Evaluation Functions Based on Evaluation Values and Win Probability. [AAAI 2007](http://www.informatik.uni-trier.de/~ley/db/conf/aaai/aaai2007.html)
- Omid David, Moshe Koppel, Nathan S. Netanyahu (2008). Genetic Algorithms for Mentor-Assisted Evaluation Function Optimization, ACM Genetic and Evolutionary Computation Conference ([GECCO '08](http://www.sigevo.org/gecco-2008/)), pp. 1469-1475, Atlanta, GA, July 2008.
- Omid David, Jaap van den Herik, Moshe Koppel, Nathan S. Netanyahu (2009). Simulating Human Grandmasters: Evolution and Coevolution of Evaluation Functions. ACM Genetic and Evolutionary Computation Conference ([GECCO '09](http://www.sigevo.org/gecco-2009/)), pp. 1483 - 1489, Montreal, Canada, July 2009.
- Omid David (2009). Genetic Algorithms Based Learning for Evolving Intelligent Organisms. Ph.D. Thesis.

## 2010 ...

- Lyudmil Tsvetkov (2010). Little Chess Evaluation Compendium. [2010 pdf](http://www.winboardengines.de/doc/LittleChessEvaluationCompendium-2010-04-07.pdf)
- Omid David, Moshe Koppel, Nathan S. Netanyahu (2011). Expert-Driven Genetic Algorithms for Simulating Evaluation Functions. Genetic Programming and Evolvable Machines, Vol. 12, No. 1, pp. 5--22, March 2011. » Genetic Programming
- Jeff Rollason (2011). [Mixing MCTS with Conventional Static Evaluation](http://www.aifactory.co.uk/newsletter/2011_02_mcts_static.htm). AI Factory, Winter 2011 » Monte-Carlo Tree Search
- Jeff Rollason (2012). [Evaluation options - Overview of methods](http://www.aifactory.co.uk/newsletter/2012_01_evaluation_options.htm). AI Factory, Summer 2012
- Lyudmil Tsvetkov (2012). An Addendum to a Little Chess Evaluation Compendium. [Addendum June 2012 pdf](http://www.winboardengines.de/doc/addendumlcec_2012.pdf), , , [Addendum 4 November 2012 pdf](http://www.winboardengines.de/doc/addendum4lcec_2012.pdf), ,
- Lyudmil Tsvetkov (2012). Little Chess Evaluation Compendium. [July 2012 pdf](http://www.winboardengines.de/doc/LittleChessEvaluationCompendium.pdf) 7,
- Derek Farren, Daniel Templeton, Meiji Wang (2013). Analysis of Networks in Chess. Team 23, Stanford University, [pdf](http://snap.stanford.edu/class/cs224w-2013/projects2013/cs224w-023-final.pdf)

## 2015 ...

- Nera Nesic, Stephan Schiffel (2016). Heuristic Function Evaluation Framework. CG 2016
- Lyudmil Tsvetkov (2017). [The Secret of Chess](http://www.secretofchess.com/). [amazon](https://www.amazon.com/Secret-Chess-Lyudmil-Tsvetkov-ebook/dp/B074M85CVV) 8
- Lyudmil Tsvetkov (2017). Pawns. [amazon](https://www.amazon.com/Pawns-Lyudmil-Tsvetkov-ebook/dp/B074S2MYQV)

# Blog & Forum Posts

## 1993 ...

- [Cray Blitz Evaluation](https://groups.google.com/d/msg/rec.games.chess/J9Pkg9lOpig/tBN5dVRATwsJ) by Robert Hyatt, rgc, March 05, 1993 » Cray Blitz
- [Mobility Measure: Proposed Algorithm](https://groups.google.com/d/msg/rec.games.chess/6vwtkcF6sRU/4M3oOiDNYwgJ) by Dietrich Kappe, rgc, September 23, 1993 » Mobility
- [bitboard position evaluations](https://groups.google.com/d/msg/rec.games.chess/M4CKCmqDNkI/TjVJEQY0GC0J) by Robert Hyatt, rgc, November 17, 1994 » Bitboards

## 1995 ...

- [Value of the pieces](https://groups.google.com/d/msg/rec.games.chess/efBhsZU3J1g/fC7rxV5yuycJ) by Joost de Heer, rgc, February 01, 1995
- [Evaluation function diminishing returns](https://groups.google.com/group/rec.games.chess.computer/browse_frm/thread/4f54813edf18fdcc) by Bruce Moreland, rgcc, February 1, 1997
- [Evaluation function question](https://groups.google.com/group/rec.games.chess.computer/browse_frm/thread/40fe48d492e582bd) by Dave Fotland, rgcc, February 07, 1997
- [computer chess "oracle" ideas...](https://groups.google.com/group/rec.games.chess.computer/browse_frm/thread/99eec6923b0481db) by Robert Hyatt, rgcc, April 01, 1997 » Oracle
- [Evolutionary Evaluation](https://groups.google.com/group/rec.games.chess.computer/browse_frm/thread/77f10f072e907302) by Daniel Homan, rgcc, September 09, 1997 » Automated Tuning
- [Books that help for evaluation](https://www.stmintz.com/ccc/index.php?id=25012) by Guido Schimmels, CCC, August 18, 1998
- [Static evaluation after the "Positional/Real Sacrifice"](https://www.stmintz.com/ccc/index.php?id=80569) by Andrew Williams, CCC, December 03, 1999

## 2000 ...

- [Adding knowledge to the evaluation, what am I doing wrong?](https://www.stmintz.com/ccc/index.php?id=289154) by Albert Bertilsson, CCC, March 13, 2003
- [testing of evaluation function](https://www.stmintz.com/ccc/index.php?id=293815) by Steven Chu, CCC, April 17, 2003 » Engine Testing
- [Question about evaluation and branch factor](https://www.stmintz.com/ccc/index.php?id=328924) by Marcus Prewarski, CCC, November 20, 2003 » Branching Factor
- [STATIC EVAL TEST (provisional)](https://www.stmintz.com/ccc/index.php?id=350516) by Jaime Benito de Valle Ruiz, CCC, February 21, 2004 » Test-Positions

## 2005 ...

- [Re: Zappa Report](https://www.stmintz.com/ccc/index.php?id=475521) by Ingo Althöfer, CCC, December 30, 2005
- [Do you evaluate internal nodes?](http://www.open-aurec.com/wbforum/viewtopic.php?f=4&t=4155#p21292) by Tord Romstad, Winboard Forum, January 16, 2006 » Interior Node
- [question about symmertic evaluation](https://talkchess.com/forum/viewtopic.php?t=13969) by Uri Blass, CCC, May 23, 2007
- [Trouble Spotter](https://talkchess.com/forum3/viewtopic.php?f=7&t=15220) by Harm Geert Muller, CCC, July 19, 2007 » Tactics
- [Search or Evaluation?](http://www.hiarcs.net/forums/viewtopic.php?t=402) by Ed Schröder, Hiarcs Forum, October 05, 2007 » Search versus Evaluation, Search

[Re: Search or Evaluation?](http://www.hiarcs.net/forums/viewtopic.php?p=2944) by Mark Uniacke, Hiarcs Forum, October 14, 2007

- [Problems with eval function](https://talkchess.com/forum3/viewtopic.php?f=7&t=20340) by Fermin Serrano, CCC, March 25, 2008 » Evaluation
- [Evaluation functions. Why integer?](https://talkchess.com/forum/viewtopic.php?t=22817) by oysteijo, CCC, August 06, 2008 » Float, Score
- [Smooth evaluation](https://talkchess.com/forum3/viewtopic.php?f=7&t=24052) by Fermin Serrano, CCC, September 29, 2008
- [Evaluating every node?](https://talkchess.com/forum/viewtopic.php?t=25795) by Gregory Strong, CCC, January 03, 2009
- [Evaluation idea](https://talkchess.com/forum3/viewtopic.php?f=7&t=26700) by Fermin Serrano, CCC, February 24, 2009
- [Accurate eval function](https://talkchess.com/forum3/viewtopic.php?f=7&t=27055) by Fermin Serrano, CCC, March 18, 2009
- [Eval Dilemma](https://talkchess.com/forum/viewtopic.php?t=27299) by Edsel Apostol, CCC, April 03, 2009
- [Linear vs. Nonlinear Evalulation](https://talkchess.com/forum/viewtopic.php?topic_view=threads&p=288424) by Gerd Isenberg, CCC, August 26, 2009
- [Threat information from evaluation to inform q-search](https://talkchess.com/forum/viewtopic.php?p=291259) by Gary, CCC, September 15, 2009 » Quiescence Search

## 2010 ...

- [Correcting Evaluation with the hash table](https://talkchess.com/forum/viewtopic.php?t=32396) by Mark Lefler, CCC, February 05, 2010
- [Re: Questions for the Stockfish team](https://talkchess.com/forum/viewtopic.php?topic_view=threads&p=362888&t=35455) by Milos Stanisavljevic, CCC, July 20, 2010
- [Most important eval elements](https://talkchess.com/forum/viewtopic.php?t=36104) by Tom King, CCC, September 17, 2010
- [Re: 100 long games Rybka 4 vs Houdini 1.03a](https://talkchess.com/forum/viewtopic.php?topic_view=threads&p=374967&t=36421) by Tord Romstad, CCC, November 02, 2010
- [dynamically modified evaluation function](https://talkchess.com/forum/viewtopic.php?t=37191) by Don Dailey, CCC, December 20, 2010

2011

- [Suppose Rybka used Fruits evaluations](http://rybkaforum.net/cgi-bin/rybkaforum/topic_show.pl?tid=22785) by SR, Rybka Forum, August 29, 2011
- [writing an evaluation function](https://talkchess.com/forum/viewtopic.php?t=41621) by Pierre Bokma, CCC, December 27, 2011

2012

- [The evaluation value and value returned by minimax search](https://talkchess.com/forum/viewtopic.php?t=42806) by Chao Ma, CCC, March 09, 2012
- [Multi dimensional score](https://talkchess.com/forum/viewtopic.php?t=43385) by Nicu Ionita, CCC, April 20, 2012
- [Bi dimensional static evaluation](https://talkchess.com/forum/viewtopic.php?t=43386) by Nicu Ionita, CCC, April 20, 2012
- [Theorem proving positional evaluation](https://talkchess.com/forum/viewtopic.php?t=43387) by Nicu Ionita, CCC, April 20, 2012
- [log(w/b) instead of w-b?](https://talkchess.com/forum/viewtopic.php?t=43545) by Gerd Isenberg, CCC, May 02, 2012
- [The value of an evaluation function](https://talkchess.com/forum/viewtopic.php?t=44014) by Ed Schröder, CCC, June 11, 2012

2013

- [eval scale in Houdini](https://talkchess.com/forum/viewtopic.php?t=46879) by Rein Halbersma, CCC, January 14, 2013 » Houdini
- [An idea of how to make your engine play more rational chess](https://talkchess.com/forum/viewtopic.php?t=46993) by Pio Korinth, CCC, January 25, 2013
- [A Materialless Evaluation?](https://talkchess.com/forum/viewtopic.php?t=48252) by Thomas Kolarik, CCC, June 12, 2013
- [A different way of summing evaluation features](https://talkchess.com/forum/viewtopic.php?t=48644) by Pio Korinth, CCC, July 14, 2013 9 10
- [Improve the search or the evaluation?](https://talkchess.com/forum/viewtopic.php?t=49190) by Jens Bæk Nielsen, CCC, August 31, 2013 » Search versus Evaluation
- [Multiple EVAL](https://talkchess.com/forum/viewtopic.php?t=49421) by Ed Schroder, CCC, September 22, 2013
- [floating point SSE eval](https://talkchess.com/forum/viewtopic.php?t=50472) by Marco Belli, CCC, December 13, 2013 » Float, Score

2014

- [5 underestimated evaluation rules](https://talkchess.com/forum/viewtopic.php?t=51012) by Lyudmil Tsvetkov, CCC, January 23, 2014
- [Thoughs on eval terms](https://talkchess.com/forum/viewtopic.php?t=51811) by Fermin Serrano, CCC, March 31, 2014

## 2015 ...

- [Value of a Feature or Heuristic](https://talkchess.com/forum/viewtopic.php?t=55355) by Jonathan Rosenthal, CCC, February 15, 2015
- [Couple more ideas](https://talkchess.com/forum/viewtopic.php?t=55897) by Lyudmil Tsvetkov, CCC, April 05, 2015
- [Most common/top evaluation features?](https://talkchess.com/forum/viewtopic.php?t=55955) by Alexandru Mosoi, CCC, April 10, 2015
- [eval pieces](https://talkchess.com/forum/viewtopic.php?t=56690) by Daniel Anulliero, CCC, June 15, 2015
- [* vs +](https://talkchess.com/forum/viewtopic.php?t=57022) by Stefano Gemma, CCC, July 19, 2015
- [(E)valuation (F)or (S)tarters](https://talkchess.com/forum/viewtopic.php?t=57087) by Ed Schröder, CCC, July 26, 2015

2016

- [Non-linear eval terms](https://talkchess.com/forum/viewtopic.php?t=59091) by J. Wesley Cleveland, CCC, January 29, 2016
- [A bizarre evaluation](https://talkchess.com/forum/viewtopic.php?t=59570) by Larry Kaufman, CCC, March 20, 2016
- [Chess position evaluation with convolutional neural network in Julia](http://int8.io/chess-position-evaluation-with-convolutional-neural-networks-in-julia/) by Kamil Czarnogorski, [Machine learning with Julia and python](http://int8.io/), April 02, 2016 » Deep Learning, Neural Networks
- [Calculating space](https://talkchess.com/forum/viewtopic.php?t=61064) by Shawn Chidester, CCC, August 07, 2016
- [Evaluation values help](https://talkchess.com/forum/viewtopic.php?t=61236) by Laurie Tunnicliffe, CCC, August 26, 2016
- [A database for learning evaluation functions](https://talkchess.com/forum/viewtopic.php?t=61861) by Álvaro Begué, CCC, October 28, 2016 » Automated Tuning, Learning, Texel's Tuning Method
- [Evaluation doubt](https://talkchess.com/forum/viewtopic.php?t=61875) by Fabio Gobbato, CCC, October 29, 2016

2017

- [Bayesian Evaluation Functions](https://talkchess.com/forum/viewtopic.php?t=63181) by Jonathan Rosenthal, CCC, February 15, 2017
- [improved evaluation function](https://talkchess.com/forum/viewtopic.php?t=63408) by Alexandru Mosoi, CCC, March 11, 2017 » Texel's Tuning Method, Zurichess
- [random evaluation perturbation factor](https://talkchess.com/forum/viewtopic.php?t=63803) by Stuart Cracraft, CCC, April 24, 2017
- [horrid positional play in a solid tactical searcher](https://talkchess.com/forum/viewtopic.php?t=63863) by Stuart Cracraft, CCC, April 29, 2017
- [Another attempt at comparing Evals ELO-wise](https://talkchess.com/forum/viewtopic.php?t=64041) by Kai Laskos, CCC, May 22, 2017 » Playing Strength
- [static eval in every node?](https://talkchess.com/forum/viewtopic.php?t=64230) by Erin Dame, CCC, June 09, 2017
- [comparing between search or evaluation](https://talkchess.com/forum/viewtopic.php?t=65403) by Uri Blass, CCC, October 09, 2017» Search
- [Neural networks for chess position evaluation- request](https://talkchess.com/forum/viewtopic.php?t=65715) by Kamil Czarnogorski, CCC, November 13, 2017 » Deep Learning, Neural Networks
- [AlphaGo's evaluation function](https://talkchess.com/forum/viewtopic.php?t=65829) by Jens Kipper, CCC, November 26, 2017
- [Logarithmic Patterns In Evaluations](https://talkchess.com/forum/viewtopic.php?t=65946) by Dennis Sceviour, CCC, December 09, 2017

2018

- [replace the evaluation by playing against yourself](https://talkchess.com/forum/viewtopic.php?t=66413) by Uri Blass, CCC, January 25, 2018 » Fortress
- [Poor man's neurones](https://talkchess.com/forum3/viewtopic.php?f=7&t=67524) by Pawel Koziol, CCC, May 21, 2018 » Neural Networks
- [Xiangqi evaluation](https://talkchess.com/forum3/viewtopic.php?f=7&t=67877) by Harm Geert Muller, CCC, July 01, 2018 » Xiangqi

## 2020 ...

- [romantic-style play](https://talkchess.com/forum3/viewtopic.php?f=7&t=74652) by Stuart Cracraft, CCC, August 02, 2020
- [Engine choosing between sets of piece/square tables](https://prodeo.actieforum.com/t120-engine-choosing-between-sets-of-piece-square-tables) by Pawel Koziol, ProDeo Forum, December 05, 2020 » Rodent, Piece-Square Tables
- [Controlled randomness of evaluation function](https://prodeo.actieforum.com/t123-controlled-randomness-of-evaluation-function) by nescitus, ProDeo Forum, December 06, 2020
- [Manually tuned evaluation](https://talkchess.com/forum3/viewtopic.php?f=7&t=76161) by Maksim Korzh, CCC, December 27, 2020 » Simplified Evaluation Function

2021

- [So what do we miss in the traditional evaluation?](https://talkchess.com/forum3/viewtopic.php?f=2&t=76446) by Ferdinand Mosca, CCC, January 29, 2021 » NNUE
- [HCE and NNUE and vectorisation](https://talkchess.com/forum3/viewtopic.php?f=7&t=76556) by Vivien Clauzon, CCC, February 11, 2021 » NNUE, Minic
- [Idea: use range (evalMin - evalMax) for position evaluation](https://groups.google.com/g/lczero/c/TLCMkkdm1hw/m/erjTVGUqAQAJ) by Mirza Hadzic, LCZero Forum, April 6, 2021

[Re: Idea: use range (evalMin - evalMax) for position evaluation](https://groups.google.com/g/lczero/c/TLCMkkdm1hw/m/SgbGghzhBAAJ) by Álvaro Begué, LCZero Forum, May 28, 2021 11

[Re: Idea: use range (evalMin - evalMax) for position evaluation](https://groups.google.com/g/lczero/c/TLCMkkdm1hw/m/F9JjnN8FBQAJ) by Warren D. Smith, LCZero Forum, May 29, 2021

- [I declare that HCE is dead...](https://talkchess.com/forum3/viewtopic.php?f=2&t=77571) by Andrew Grant, CCC, June 29, 2021 » Ethereal, NNUE
- [Evaluation questions](https://talkchess.com/forum3/viewtopic.php?f=7&t=77952) by Ellie Moore, CCC, August 16, 2021

2023

- [Most Important Evaluation Terms](https://talkchess.com/viewtopic.php?t=81968) by Aditya Chandra, CCC, April 30, 2023

# External Links

## Mathematical Foundations

- [Heuristic from Wikipedia](https://en.wikipedia.org/wiki/Heuristic_(computer_science))
- [Linear combination from Wikipedia](https://en.wikipedia.org/wiki/Linear_combination)
- [Linear independence from Wikipedia](https://en.wikipedia.org/wiki/Linear_independence)
- [Orthogonality from Wikipedia](https://en.wikipedia.org/wiki/Orthogonality)
- [Principal component analysis from Wikipedia](https://en.wikipedia.org/wiki/Principal_component_analysis)

## Chess Evaluation

- [Evaluation function from Wikipedia](https://en.wikipedia.org/wiki/Evaluation_function)
- [Stockfish Evaluation Guide](https://hxim.github.io/Stockfish-Evaluation-Guide/) » Stockfish Evaluation Guide
- [GitHub - gekomad/chess-engine-eval-debugger: Chess engine web evaluator](https://github.com/gekomad/chess-engine-eval-debugger) by Giuseppe Cannella
- [Evaluation: Basics](http://home.hccnet.nl/h.g.muller/eval.html) of Micro-Max by Harm Geert Muller
- [Chess Programming Part VI: Evaluation Functions](http://www.gamedev.net/page/resources/_/technical/artificial-intelligence/chess-programming-part-vi-evaluation-functions-r1208) by François-Dominic Laramée, [gamedev.net](https://en.wikipedia.org/wiki/GameDev.net), October 2000
- [About the Values of Chess Pieces](http://www.chessvariants.com/d.betza/pieceval/index.html) by Ralph Betza

# References

Up one level    Claude Shannon (1949). [Programming a Computer for Playing Chess](http://www.pi.infn.it/%7Ecarosi/chess/shannon.txt). [pdf](http://archive.computerhistory.org/projects/chess/related_materials/text/2-0%20and%202-1.Programming_a_computer_for_playing_chess.shannon/2-0%20and%202-1.Programming_a_computer_for_playing_chess.shannon.062303002.pdf)↩︎ [Re: Linear vs. Nonlinear Evaluation](https://talkchess.com/forum/viewtopic.php?topic_view=threads&p=288501&t=29552) by Tord Romstad, CCC, August 27, 2009↩︎ [Re: Linear vs. Nonlinear Evaluation](https://talkchess.com/forum/viewtopic.php?topic_view=threads&p=288564&t=29552) by Robert Hyatt, CCC, August 27, 2009↩︎ [Re: Books that help for evaluation](https://www.stmintz.com/ccc/index.php?id=25046) by Robert Hyatt, CCC, August 18, 1998↩︎ [Re: Zappa Report](https://www.stmintz.com/ccc/index.php?id=475521) by Ingo Althöfer, CCC, December 30, 2005↩︎ [Re: Evaluation by neural network ?](https://www.stmintz.com/ccc/index.php?id=11893) by Jay Scott, CCC, November 10, 1997↩︎ [An Update of the Addendum to the LittleCompendium](https://talkchess.com/forum/viewtopic.php?t=44265) by Lyudmil Tsvetkov, CCC, July 02, 2012↩︎ [The Secret of Chess](https://talkchess.com/forum/viewtopic.php?t=64776) by Lyudmil Tsvetkov, CCC, August 01, 2017↩︎ [Euclidean distance from Wikipedia](https://en.wikipedia.org/wiki/Euclidean_distance)↩︎ [Principal component analysis from Wikipedia](https://en.wikipedia.org/wiki/Principal_component_analysis)↩︎ Eric B. Baum, Warren D. Smith (1999). [Propagating Distributions Up Directed Acyclic Graphs](https://www.mitpressjournals.org/doi/abs/10.1162/089976699300016881?journalCode=neco). [Neural Computation](https://en.wikipedia.org/wiki/Neural_Computation_%28journal%29), Vol. 11, No. 1↩︎
