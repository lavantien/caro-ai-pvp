source: https://chessprogramming.org/Time_Management

# Time Management

Home * Search * Time Management

Roland La Tuffo Barcsik - Time out 1

---

1. [The Streatham & Brixton Chess Blog: Chess in Art Postscript: Chess-in-Artists do for Christmas](http://streathambrixtonchess.blogspot.de/2009/11/chess-in-art-postscript-chess-in_22.html) The site Roland La Tuffo Barczik Hungarian artist, La Tuffo's Artwork, Barcsik's Chess Art no longer available↩︎

Time management refers to algorithms and heuristics to allocate time for searching a move under [time control](https://en.wikipedia.org/wiki/Time_control) requirements in a game of chess. The player to move consumes his time, and if he exceeds his time limit, the game is lost on demand of the opponent player, or in automatic computer chess play by an arbiter instance.

Iterative deepening in conjunction with its predictable effective branching factor allows a flexible time management either to terminate the current iteration and to fall back on best move and PV of the previous iteration, or to decide about termination prior to start a new iteration or to search a next root-move.

# Time Controls

[Fast chess](https://en.wikipedia.org/wiki/Fast_chess) is usually played with an immediate [sudden death](https://en.wikipedia.org/wiki/Sudden_death_%28sport%29#Board_games) time control, while others may have one or more regular time controls before the sudden death control applies there as well.

## Sudden Death

Sudden death refers to a requirement that all the remaining moves, rather than a fixed number of moves, need to be played within the remaining time. Typically programs estimate the game will last further 25..40 moves, and divide the remaining time by this number.

## Cyclic Time Controls

Cyclic time controls define a number of moves to be made in a fixed amount of time. We don't have to make estimations for how long the game will last. However, some time should be saved in order to have a buffer in case later moves warrant longer thinking times.

## Time Control with Increment

To avoid time trouble where often blunders decide the game, chess champions, most notable [Bobby Fischer](https://en.wikipedia.org/wiki/Bobby_Fischer) and David Bronstein, proposed a delay or increment of time for each move made. It requires a special delay clock, which became handy with the [development of digital chess clocks](https://en.wikipedia.org/wiki/Game_clock#Early_development_of_digital_game_clocks) in the 70s and 80s 1 .

Time trouble is also an issue in computer chess, either due to operators in over the board chess, boosted by deviation of internal and external clock, or in transmitting move transfer latencies in automatic play, where it is quite common nowadays to play with increment per move.

### Fischer Time

In 1988 [Bobby Fischer](https://en.wikipedia.org/wiki/Bobby_Fischer) proposed an unconditional increment per move, no matter whether the delay was exhausted or not. With Fischer time one may therefor increase the remaining time if one moves faster than the delay 2 .

### Bronstein Time

Bronstein's time works similar, but never increases the remaining time. It was for instance used during the late Aegon Tournaments.

# Basic TM

A basic time management technique is to use base / 20 + increment / 2 time per move. This is very competitive with advanced time management schemes, and is typically chosen as the foundation to build upon.

# Enhancements

Human chess players often wonder about the inflexible time management of various programs. A basic time management scheme might be enhanced in several ways, considering dynamic, statistical as well as static features of the search, the best move and its PV.

## Soft Bound

Drawing on the observation that terminating a depth halfway is wasteful, a large enhancement to a basic time management scheme is to introduce two levels of time allocation. Namely, one optimium time threshold (soft bound), and one maximum time threshold (hard bound). During search, the optimum time threshold is checked on each iteration of iterative deepening, while the maximum time threshold is checked periodically, usually by every set amount of nodes.

## Considerations

- How often did the best move change during the (last N) previous iterations?
- The score function over iterations and best moves, increase or decrease and/or oscillation, score amplitude, etc.
- The ratio of the size of the subtree under the best move versus the size of the whole search tree
- Only one obvious way to recapture in an otherwise quiet position

## Premature Termination

- Only one legal move
- Prior a start of a new iteration, the relation of elapsed and allocated time (f.i. > 50%) 3

## Historical Techniques

These techniques saw use in older programs, but have fallen out of use with the advent of modern testing and tuning.

- Fail low situations, a severe drop of the score may cause programs to allocate "panic time" to hopefully solve the critical situation
- During the first moves out of the opening book programs often allocate more time

For instance, Robert Hyatt gave following formula from Cray Blitz in Using Time Wisely 4

```
   nMoves =  min( numberOfMovesOutOfBook, 10 );
   factor = 2 -  nMoves / 10
   target = timeLeft / numberOfMovesUntilNextTimeControl
   time   = factor * target
```

inspired by following graph of human timing from several grandmaster tournament games

Note that good time management for humans differs greatly from good time management for engines, and this graph should not be taken as a way to shape your engine's time usage.

- New and therefor likely not singular best moves, but statically "suspect", like weakening the pawn structure or a sacrifice favors to allocate extra time and to start a further iteration, even if the score is fine.

# Losing on Time

- Tech 2 versus Ribbit at ACM 1974
- Duchess vs. T. Belle at ACM 1974
- Chess Tiger X - Pharaon 2.65 at Massy 2002

# See also

- CPW-Engine_chronos
- Iterative Deepening
- Playing Strength
- Pondering
- Search Explosion
- Search Statistics

# Publications

- Robert Hyatt (1984). Using Time Wisely. ICCA Journal, Vol. 7, No. 1
- Robert Hyatt, Albert Gower, Harry Nelson (1985). Using Time Wisely, revisited (extended abstract). Proceedings of the 1985 ACM annual conference on The range of computing: mid-80's perspective, p. 271, Denver, Colorado. ISBN 0-89791-170-9.
- Shaul Markovitch, Yaron Sella (1993). [Learning of Resource Allocation Strategies for Game Playing](https://onlinelibrary.wiley.com/doi/abs/10.1111/j.1467-8640.1996.tb00254.x). IJCAI 1993, [pdf](https://www.ijcai.org/Proceedings/93-2/Papers/020.pdf)
- Ingo Althöfer, Chrilly Donninger, Ulf Lorenz, Valentin Rottmann (1994). On Timing, Permanent Brain and Human Intervention. Advances in Computer Chess 7
- Chrilly Donninger (1994). A la Recherche du Temps Perdu: 'That was easy'. ICCA Journal, Vol. 17, No. 1
- Levente Kocsis, Jos Uiterwijk, Jaap van den Herik (2000). [Learning Time Allocation using Neural Networks](http://link.springer.com/chapter/10.1007/3-540-45579-5_11). CG 2000, [postscript](http://zaphod.aml.sztaki.hu/papers/kocsis-CG00.ps)
- Vladan Vučković, Rade Šolak (2009). [Time Management Procedure in Computer Chess](http://facta.junis.ni.ac.rs/acar/acar200901/acar2009-07.html). [Facta Universitatis, Automatic Control and Robotics, Vol. 8, No. 1](http://facta.junis.ni.ac.rs/acar/acar200901/acar200901toc.html)
- Rade Šolak, Vladan Vučković (2009). Time Management during a Chess Game. ICGA Journal, Vol. 32 No. 4
- Shih-Chieh Huang, Rémi Coulom, Shun-Shii Lin (2011). Time Management for Monte-Carlo Tree Search Applied to the Game of Go. TAAI 2010, [pdf](http://remi.coulom.free.fr/Publications/TimeManagement.pdf)
- Hendrik Baier, Mark Winands (2011). [Time Management for Monte-Carlo Tree Search in Go](http://link.springer.com/chapter/10.1007/978-3-642-31866-5_4). Advances in Computer Games 13
- Hendrik Baier, Mark Winands (2016). Time Management for Monte Carlo Tree Search. IEEE Transactions on Computational Intelligence and AI in Games, Vol. 8, No. 3, [draft as pdf](https://dke.maastrichtuniversity.nl/m.winands/documents/time_management_for_monte_carlo_tree_search.pdf)

# Forum Posts

## 1993 ...

- [Open Letter To Chess Computer Programmers](https://groups.google.com/group/rec.games.chess/browse_frm/thread/c7bf58b3c59273e2/f388d174febe18d2) by Kevin Gowen, rec.games.chess, December 26, 1993
- [Computer chess strategy](https://groups.google.com/group/rec.games.chess.computer/browse_frm/thread/5ecf6f473db3eeb6) by Al, rgcc, April 22, 1997
- [Time usage](https://www.stmintz.com/ccc/index.php?id=12827) by John Bartkiw, CCC, December 08, 1997
- [How to program search timeout ?](https://www.stmintz.com/ccc/index.php?id=14872) by Rudolf Posch, CCC, February 04, 1998
- [How much time per move?](https://www.stmintz.com/ccc/index.php?id=15406) by Andrew Williams, CCC, March 02, 1998
- [Time control legend](https://www.stmintz.com/ccc/index.php?id=18553) by Don Dailey, CCC, May 13, 1998

## 2000 ...

- [Question: Fail low at root and time management](https://www.stmintz.com/ccc/index.php?id=95710) by William Bryant, CCC, February 08, 2000 » Fail-Low, Root
- [new idea on managing time using depth reduction at root](https://www.stmintz.com/ccc/index.php?id=282702) by Scott Farrell, CCC, February 08, 2003
- [finding when a move is obvious](https://www.stmintz.com/ccc/index.php?id=359869) by Eric Oldre, CCC, April 13, 2004
- [question about fixing the time management of movei](https://www.stmintz.com/ccc/index.php?id=378905) by Uri Blass, CCC, July 25, 2004

## 2005 ...

- [Where to put timeout() code in search?](http://www.talkchess.com/forum/viewtopic.php?p=131908) by Stuart Cracraft, CCC, July 18, 2007
- [obvious/easy move](http://www.talkchess.com/forum/viewtopic.php?t=20125) by Charles Roberson, CCC, March 12, 2008
- [Crafty (and others?) time management question](http://www.talkchess.com/forum/viewtopic.php?t=27446) by John Merlino, CCC, April 14, 2009
- [Time managment on ponder hit](http://www.talkchess.com/forum/viewtopic.php?t=28438) by Mathieu Pagé, CCC, June 16, 2009 » Pondering
- [Info from timeout search](http://www.talkchess.com/forum/viewtopic.php?t=30326) by Michel Van den Bergh, CCC, October 26, 2009

## 2010 ...

- [As though they were pondering](http://www.talkchess.com/forum/viewtopic.php?t=35554) by Gabor Szots, CCC, July 23, 2010 » Pondering
- [Move on Hash Hit](http://www.open-chess.org/viewtopic.php?f=5&t=588) by kingliveson, OpenChess Forum, August 18, 2010 » Pondering
- [New Time Controls for WB](http://www.talkchess.com/forum/viewtopic.php?t=35931) by Matthias Gemuh, CCC, August 30, 2010 » Chess Engine Communication Protocol, WinBoard, ChessGUI
- [Repeating moves to add time](http://www.talkchess.com/forum3/viewtopic.php?f=7&t=38608) by Fermin Serrano, CCC, March 31, 2011

2012

- [Sudden death time controls](http://www.talkchess.com/forum/viewtopic.php?t=43636) by Larry Kaufman, CCC, May 10, 2012
- [Winboard protocol and fractional increments](http://www.talkchess.com/forum/viewtopic.php?t=45325) by Jon Dart, CCC, September 25, 2012 » Chess Engine Communication Protocol, WinBoard
- [Adjustable search pruning depending on time control](http://www.talkchess.com/forum/viewtopic.php?t=46503) by Jerry Donald, CCC, December 20, 2012 » Pruning, Late Move Reductions

2013

- ["panic time" and "easy moves"](http://www.talkchess.com/forum/viewtopic.php?t=47242) by Robert Hyatt, CCC, February 16, 2013
- [Yet another time allocation heuristic](http://www.talkchess.com/forum/viewtopic.php?t=47251) by Steven Edwards, CCC, February 17, 2013
- [easy-hard moves (again)](http://www.talkchess.com/forum/viewtopic.php?t=47442) by Robert Hyatt, CCC, March 08, 2013
- [Easy easy move](http://www.talkchess.com/forum/viewtopic.php?t=48824) by Harm Geert Muller, CCC, August 02, 2013
- [out-of-time: what to do?](http://www.talkchess.com/forum/viewtopic.php?t=48903) by Folkert van Heusden, CCC, August 09, 2013
- [Losing on time](http://www.talkchess.com/forum/viewtopic.php?t=50705) by Gregory Strong, CCC, December 31, 2013

2014

- [Time control comparison between engines](http://www.talkchess.com/forum/viewtopic.php?t=50718) by Ed Schroder, CCC, January 01, 2014
- [fixed time control management](http://www.talkchess.com/forum/viewtopic.php?t=51135) by Daniel Shawul, CCC, February 01, 2014
- [Question about Time Management](http://www.open-aurec.com/wbforum/viewtopic.php?f=4&t=53060) by ambrooks1, Winboard Forum, February 06, 2014
- [Move time and compiler optimization](http://www.talkchess.com/forum/viewtopic.php?t=51720) by Edmund Moshammer, CCC, March 23, 2014
- [Playing strength development - increasing time control](http://www.talkchess.com/forum/viewtopic.php?t=52021) by Andreas Strangmüller, CCC, April 17, 2014
- [Time based contempt](http://www.talkchess.com/forum/viewtopic.php?t=52069) by Michel Van den Bergh, April 20, 2014 » Contempt Factor
- [Time Management](http://www.talkchess.com/forum/viewtopic.php?t=52464) by Larry Kaufman, CCC, May 29, 2014

## 2015 ...

- [Elo gain and optimal time management](http://www.talkchess.com/forum/viewtopic.php?t=54939) by Kai Laskos, CCC, January 11, 2015
- [What's the fastest time control you can effectively test at?](http://www.talkchess.com/forum/viewtopic.php?t=56534) by Jordan Bray, CCC, May 30, 2015

2016

- [Time management trick](http://www.talkchess.com/forum3/viewtopic.php?f=7&t=60869) by Michael Sherwin, CCC, July 19, 2016
- [Photographing Chess Clock](http://www.talkchess.com/forum/viewtopic.php?t=61672) by Harm Geert Muller, CCC, October 10, 2016
- [Doubling of time control](http://www.talkchess.com/forum/viewtopic.php?t=61784) by Andreas Strangmüller, CCC, October 21, 2016 » Doubling TC, Diminishing Returns, Playing Strength, Komodo
- [New idea for "easy move detection"](http://www.talkchess.com/forum/viewtopic.php?t=61976) by Rasmus Althoff, CCC, November 05, 2016 » CT800
- [Stockfish 8 - Double time control vs. 2 threads](http://www.talkchess.com/forum/viewtopic.php?t=62146) by Andreas Strangmüller, CCC, November 15, 2016 » Doubling TC, Diminishing Returns, Playing Strength, Stockfish
- [On time management](http://www.talkchess.com/forum/viewtopic.php?t=62586) by Rasmus Althoff, CCC, December 24, 2016

2017

- [Time managment ?](http://www.talkchess.com/forum/viewtopic.php?t=63362) by Mahmoud Uthman, CCC, March 07, 2017
- [Time management ideas](http://www.open-chess.org/viewtopic.php?f=5&t=3098) by lucasart, OpenChess Forum, April 03, 2017
- [Invariance with time control of rating schemes](http://www.talkchess.com/forum/viewtopic.php?t=64683) by Kai Laskos, CCC, July 22, 2017 5
- [Time Managment translating to SMP](http://www.talkchess.com/forum/viewtopic.php?t=66099) by Andrew Grant, CCC, December 23, 2017 » Parallel Search

2018 ...

- [Time control envelope in top engines could be improved?](http://www.talkchess.com/forum/viewtopic.php?t=66821) by Kai Laskos, CCC, March 13, 2018 » Match Statistics, Playing Strength
- [easy move?](http://www.talkchess.com/forum3/viewtopic.php?f=7&t=68692) by Folkert van Heusden, CCC, October 19, 2018

[Re: easy move?](http://www.talkchess.com/forum3/viewtopic.php?f=7&t=68692&start=8) by Álvaro Begué, CCC, October 19, 2018

- [UCI pondering and time management](http://www.talkchess.com/forum3/viewtopic.php?f=7&t=72686) by Vivien Clauzon, CCC, December 30, 2019 » UCI, Pondering

## 2020 ...

- [CECP "time" and "otim"](http://www.talkchess.com/forum3/viewtopic.php?f=7&t=75944) by Marcel Vanthoor, CCC, November 30, 2020 » CECP

# External Links

- [Time Allocation](http://web.archive.org/web/20070607151549/www.brucemo.com/compchess/programming/time.htm) from Bruce Moreland's [Programming Topics](http://web.archive.org/web/20070607231311/www.brucemo.com/compchess/programming/index.htm)
- [Time control from Wikipedia](https://en.wikipedia.org/wiki/Time_control)
- [Time management from Wikipedia](https://en.wikipedia.org/wiki/Time_management)
- [Timing with a Neural Networks](http://web.archive.org/web/20050328050743/http://www.playwitharena.com:80/) by Volker Annuss, Arena News-Ticker, Page 6, 86, FQ, March 23, 2005 ([Wayback Machine](https://en.wikipedia.org/wiki/Wayback_Machine))
- [Time Management During a Chess Game](http://web.archive.org/web/20100615030345/http://www.chesscafe.com/text/time.txt) by Dan Heisman ([Wayback Machine](https://en.wikipedia.org/wiki/Wayback_Machine))
- [Time of check to time of use from Wikipedia](https://en.wikipedia.org/wiki/Time_of_check_to_time_of_use)
- [Buddy Rich](https://en.wikipedia.org/wiki/Buddy_Rich) - [Time Check](https://en.wikipedia.org/wiki/Don_Menza#Educator_and_composer), at The Top of the [Plaza](https://en.wikipedia.org/wiki/Midtown_Plaza_%28Rochester%29) in [Rochester, NY](https://en.wikipedia.org/wiki/Rochester,_New_York), February 6, 1973, [YouTube](https://en.wikipedia.org/wiki/YouTube) Video

[Watch on YouTube](https://www.youtube.com/watch?v=5reK-_e-02Q)

- [Hiromi’s Sonicbloom](https://en.wikipedia.org/wiki/Hiromi_Uehara#Hiromi.27s_Sonicbloom) - [Time Difference](https://en.wikipedia.org/wiki/Time_Control), 2007, [YouTube](https://en.wikipedia.org/wiki/YouTube) Video

Hiromi Uehara, [Martin Valihora](https://en.wikipedia.org/wiki/Martin_Valihora), [Tony Grey](https://en.wikipedia.org/wiki/Tony_Grey), David Fiuczynski

[Watch on YouTube](https://www.youtube.com/watch?v=cUUcHuy12ZQ)

- Hiromi Uehara, David Fiuczynski, [Tony Grey](https://en.wikipedia.org/wiki/Tony_Grey), [Jordan Perlson](http://www.linkedin.com/pub/jordan-perlson/18/562/b1b) - [Time Out](https://en.wikipedia.org/wiki/Time_Control), [XI Festival de Jazz de San Javier](https://es.wikipedia.org/wiki/Festival_Internacional_de_Jazz_de_San_Javier), 2008, [YouTube](https://en.wikipedia.org/wiki/YouTube) Video

[Watch on YouTube](https://www.youtube.com/watch?v=GzOSJZ7xGKM)

# References

Up one Level    [History of the clocks](http://digitalgametechnology.com/site/index.php/History/clock-history.html) from [DGT - Digital Game Technology](http://digitalgametechnology.com/site/)↩︎ [Digital chess clock - Google Patent Search](https://www.google.com/patents?vid=4884255)↩︎ [question about fixing the time management of movei](https://www.stmintz.com/ccc/index.php?id=378905) by Uri Blass from CCC, July 25, 2004↩︎ Robert Hyatt (1984). Using Time Wisely. ICCA Journal, Vol. 7, No. 1↩︎ [Normalized Elo](http://hardy.uhasselt.be/Toga/normalized_elo.pdf) (pdf) by Michel Van den Bergh↩︎
