source: https://chessprogramming.org/Stockfish

# Stockfish

Home * Engines * Stockfish

Stockfish logo 1 Stockfish 12 logo 2

Stockfish,
 an UCI compatible open source chess engine developed by Tord Romstad, Marco Costalba, Joona Kiiski and Gary Linscott 3, licensed under the GPL v3.0. Marco forked the project from version 2.1 of Tord's engine Glaurung, first announced by Marco in November 8, 2008 4, and in early 2009 Joona's Smaug, a further Glaurung 2.2 derivative, was incorporated 5. Starting among the top twenty engines, Stockfish has quickly climbed in strength to become the world strongest chess entity as of 2018 - at least concerning the AlphaZero hype 6, public available chess entity. The name "Stockfish" reflects the ancestry of the engine. Tord is Norwegian and Marco Italian, and there is a long history of [stockfish](https://en.wikipedia.org/wiki/Stockfish) trade from Norway to Italy (to Marco's home town of [Vicenza](https://en.wikipedia.org/wiki/Vicenza), in fact). Stockfish is also referred to as another famous "little fish", the then strongest chess engine Rybka. In 2011, Marco Costalba and Joona Kiiski stepped down as Stockfish maintainers 7. From that, the project is being developed and maintained by the Stockfish community.

A synergy effect with the Shogi community led to the promising branch of Stockfish NNUE, courtesy of Nodchip, who introduced NNUE to Stockfish in 2019 8. On September 02, 2020, Stockfish 12 was released with a huge jump in playing strength due to NNUE and further tuning of the engine 9. The release of Stockfish 13 on February 19, 2021, has been triggered by the start of sales of the Fat Fritz 2 engine by ChessBase, based on a recent development version of Stockfish with minor modifications 10. Stockfish 14, released on July 02, 2021, further improved due to efforts by Tomasz Sobczyk and Gary Linscott in designing a new NNUE architecture in conjunction with a GPU accelerated trainer written in [PyTorch](https://en.wikipedia.org/wiki/PyTorch). Further, the collaboration with the Leela Chess Zero team payed off, in providing billions of positions to train the new NNUE 11.

Stockfish 16, released June 30, 2023, removes the classical evaluation from the engine and focuses on NNUE neural networks.12

# Platforms

Since Stockfish is written in C++, it may be compiled and built for various processors and operating systems. The main source code of Stockfish could be compiled directly into Command Line Interface program. Some programmers have added code to change it into a Graphical User Interface one, which may be compulsory to run on some platforms such as iOS.

## Command Line Interface

- Stockfish for Android, Linux, macOS, and Windows was officially built by the developer team and published on both [Stockfish website](https://stockfishchess.org/) and [Stockfish GitHub](https://github.com/official-stockfish/Stockfish)

## Stockfish with built-in Graphical User Interface

We list only some programs that are popular and license-compliant (released with source code):

- Stockfish for macOS was also built and published on Mac App Store by Daylen Yang, who is also responsible for the Stockfish website
- Stockfish for iOS was built by Tord Romstad 13.

- Stockfish for iOS and watchOS was built with the app BanksiaGUI for iOS by Nguyen Pham
- Stockfish for Android was built with the app Droidfish by Peter Österlund

# Fishtest

The Stockfish Testing Framework dubbed Fishtest 14 is a [web application](https://en.wikipedia.org/wiki/Web_application) written by Gary Linscott 15 16, based on a [SETI@home](https://en.wikipedia.org/wiki/SETI@home) kind of [volunteer computing](https://en.wikipedia.org/wiki/Volunteer_computing). Fishtest is mainly written in Python under the Pyramid Application Development Framework 17, and distributes games across different machines to reduce the test latency and increase throughput. Started in early 2013 with Stockfish 3.0, Fishtest has hundreds of contributors, as of July 2024, 2226 testers and 363 developers 18 active in testing ideas and tweaks 19, to make Stockfish the strongest chess entity of the world 20.

# Evaluation Guide

Since April 2017 the interactive Stockfish Evaluation Guide is available to explore Stockfish's evaluation with a JavaScript implementation running in a [browser](https://en.wikipedia.org/wiki/Web_browser) 21. One may enter a FEN string of a position, to get the resulting score of the main evaluation term considering the game phases within its tapered evaluation, and may navigate through the tree of subterms and features with its particular characteristics for the given position 22, also supporting Stockfish NNUE nets 23.

# Tournament Play

Stockfish is top contender of the prestigious Top Chess Engines Competition (TCEC), reaching the superfinals since season 4, and established its world number one status in winning TCECs, leaving its commercial rivals Komodo and Houdini behind. Since season 14 in early 2019, Stockfish competes with the deep learning Leela Chess Zero engines, whose playing strength triggered a motivation boost in the developing community to improve Stockfish further.

# GM+Rybka vs. Stockfish

On July 19, 2014, Stockfish 5 played a four-game match versus [Daniel Naroditsky](https://en.wikipedia.org/wiki/Daniel_Naroditsky) plus Rybka 3 (2008), 45 minutes plus 30-second increment. Stockfish won 3½ - ½ 24 25 . A few weeks later the experiment continued with [Hikaru Nakamura](https://en.wikipedia.org/wiki/Hikaru_Nakamura) in [Burlingame, California](https://en.wikipedia.org/wiki/Burlingame,_California) 26 . Supported two games by Rybka 3, Nakamura lost ½ - 1½, two games with pawn odds (Stockfish both Black without h- and b-pawn) ended ½ - 1½ in favour of Stockfish 5 as well. It played the latest development build compiled for OS X running on a 3 GHz 8-Core Mac Pro 27 .

# Selected Features

28

## Board Representation

- 8x8 Board
- Bitboards with Little-Endian Rank-File Mapping (LERF)
- Magic Bitboards

BMI2 - PEXT Bitboards (not recommend for AMD [Ryzen](https://en.wikipedia.org/wiki/Ryzen) 29 prior to [Zen 3](https://en.wikipedia.org/wiki/Zen_3))

- Piece-Lists until Stockfish 12 30 31 32

## Search

- Iterative Deepening
- Aspiration Windows
- Improving Heuristic
- Parallel Search using Threads
  - YBWC prior to Stockfish 7
  - Lazy SMP since Stockfish 7, January 2016

- Principal Variation Search
- Transposition Table
  - Shared Hash Table
  - 10 Bytes per Entry, 3 Entries per Cluster
  - Depth-preferred Replacement Strategy
  - No PV-Node probing
  - Prefetch

- Move Ordering
  - Continuation History
    - Counter Moves History since Stockfish 7, January 2016 33

  - Capture History
  - History Heuristic
  - MVV/LVA
  - SEE

- Selectivity
  - Extensions
    - Restricted Singular Extensions
    - Capture Extensions

  - Pruning
    - Futility Pruning
    - Move Count Based Pruning
    - Null Move Pruning
      - Dynamic Depth Reduction based on depth and value
      - Static Null Move Pruning
      - Verification search at high depths

    - ProbCut
    - SEE Pruning

  - Reductions
    - Late Move Reductions
    - Internal Iterative Reductions
    - Razoring

  - Quiescence Search

## Evaluation


See also Evaluation Philosophy 34 35

## Classical Evaluation

Classical Evaluation (traditional hand-crafted evaluation) has been removed since version 16.

- Tapered Eval
- Score Grain: ~1/256 of a pawn unit
- Material
  - Point Values
    - Midgame: 198, 817, 836, 1270, 2521
    - Endgame: 258, 846, 857, 1278, 2558

  - Bishop Pair
  - Imbalance Tables
  - Material Hash Table

- Piece-Square Tables
- Space
- Mobility
  - Trapped Pieces
  - Rooks on (Semi) Open Files

- Outposts
- Pawn Structure
  - Pawn Hash Table
  - Backward Pawn
  - Doubled Pawn
  - Isolated Pawn
  - Phalanx
  - Connected Pawns
  - Passed Pawn

- King Safety
  - Attacking King Zone
  - Pawn Shelter
  - Pawn Storm
  - Square Control

- Evaluation Patterns

## Misc

- Chess960
- Stockfish's Tuning Method

SPSA

- Syzygy Bases

# Release Dates

## 2008

- Stockfish 1.0 - November 02, 2008
- Stockfish 1.01 - November 03, 2008
- Stockfish 1.1 - December 06, 2008
- Stockfish 1.1a - December 08, 2008
- Stockfish 1.2 - December 29, 2008

## 2009

- Stockfish 1.3 - May 02, 2009
- Stockfish 1.3.1 - May 03, 2009
- Stockfish 1.4 - July 05, 2009
- Stockfish 1.5 - October 04, 2009
- Stockfish 1.5.1 - October 11, 2009
- Stockfish 1.6 - December 25, 2009
- Stockfish 1.6.1 - December 25, 2009
- Stockfish 1.6.2 - December 31, 2009

## 2010 ...

- Stockfish 1.6.3 - February 02, 2010
- Stockfish 1.7 - April 08, 2010
- Stockfish 1.7.1 - April 10, 2010
- Stockfish 1.8 - July 02, 2010
- Stockfish 1.9 - October 02, 2010
- Stockfish 1.9.1 - October 05, 2010

2011

- Stockfish 2.0 - January 01, 2011
- Stockfish 2.0.1 - January 04, 2011
- Stockfish 2.1 - May 04, 2011
- Stockfish 2.1.1 - May 08, 2011
- Stockfish 2.2 - December 29, 2011

2012

- Stockfish 2.2.1 - January 06, 2012
- Stockfish 2.2.2 - January 14, 2012
- Stockfish 2.3 - September 15, 2012
- Stockfish 2.3.1 - September 22, 2012

2013

- Stockfish 3 - April 30, 2013
- Stockfish 4 - August 20, 2013
- Stockfish DD - November 29, 2013
- Stockfish 5 - May 31, 2014

## 2015 ...

- Stockfish 6 - January 27, 2015
- Stockfish 7 - January 02, 2016
- Stockfish 8 - November 01, 2016
- Stockfish 9 - February 01, 2018
- Stockfish 10 - November 29, 2018

## 2020 ...

- Stockfish 11 - January 18, 2020
- Stockfish 12 - September 02, 2020
- Stockfish 13 - February 19, 2021
- Stockfish 14 - July 02, 2021
- Stockfish 15 - April 18, 2022
- Stockfish 15.1 - December 04, 2022
- Stockfish 16 - June 30, 2023
- Stockfish 16.1 - February 24, 2024
- Stockfish 17 - September 06, 2024

## 2025 ...

- Stockfish 17.1 - March 30, 2025

- Stockfish 18 - January 31, 2026

# Ports

- asmFish
- CFish
- DroidFish
- Fat Titz
- Portfish
- Rustfish
- Stockfish-js 36

# Derivatives

- Brainfish
- Crystal
- DON
- Eman
- Fat Fritz 2.0
- Houdini
- McBrain
- ShashChess
- Sting
- SugaR

# Authors

## Founders of the Stockfish project and Fishtest infrastructure

- Marco Costalba
- Joona Kiiski
- Gary Linscott
- Tord Romstad

## All other authors of the code

There are 196 authors, counted to version 15.1.

- Contributors

# Elo Progress

of Stockfish in first 10 years 37

# See also

- Evaluation Philosophy
- Glaurung
- Leela Chess Zero
- Raspberry Turk
  - HalfKP (Stockfish 12)
  - HalfKAv2 (Stockfish 14)

# Publications

- Arno Nickel (2012). [Die schöne neue Welt der Schachengines](http://www.edition-marco-shop.de/epages/64079634.sf/de_DE/?ObjectPath=/Shops/64079634/Categories/Schachgeschehen/Computerschach). [SCHACH](http://www.zeitschriftschach.de/) 2,3,5,6 2012, [pdf](http://www.edition-marco-shop.de/WebRoot/Store14/Shops/64079634/5177/F0A3/C389/D0DD/3A71/C0A8/2935/25F6/Die_schoene_neue_Welt_der_Schachengines.pdf) (German) 38
- Oleg Arenz (2012). Monte Carlo Chess. B.Sc. thesis, Darmstadt University of Technology, advisor Johannes Fürnkranz, [pdf](http://www.ke.tu-darmstadt.de/lehre/arbeiten/bachelor/2012/Arenz_Oleg.pdf) » Monte-Carlo Tree Search
- Tamal T. Biswas, Kenneth W. Regan (2015). Quantifying Depth and Complexity of Thinking and Knowledge. [ICAART 2015](http://www.icaart.org/EuropeanProjectSpace.aspx?y=2015), [pdf](http://www.cse.buffalo.edu/~regan/papers/pdf/BiReICAART15CR.pdf)
- Tamal T. Biswas, Kenneth W. Regan (2015). Measuring Level-K Reasoning, Satisficing, and Human Error in Game-Play Data. IEEE [ICMLA 2015](http://www.icmla-conference.org/icmla15/), [pdf preprint](http://www.cse.buffalo.edu/~regan/papers/pdf/BiRe15_ICMLA2015.pdf)
- Shu Yokoyama, Tomoyuki Kaneko, Tetsuro Tanaka (2015). Parameter-Free Tree Style Pipeline in Asynchronous Parallel Game-Tree Search. Advances in Computer Games 14 , [pdf](http://www.graco.c.u-tokyo.ac.jp/~kaneko/papers/acg2015-yokoyama.pdf) » P-GPP
- Jean-Marc Alliot (2017). Who is the Master? ICGA Journal, Vol. 39, No. 1, [draft as pdf](http://www.alliot.fr/CHESS/draft-icga-39-1.pdf) 39
- Bill Jordan (2020). Calculation versus Intuition: Stockfish versus Leela. [amazon](https://www.amazon.com/Calculation-versus-Intuition-Stockfish-Leela-ebook/dp/B08LYBQDMB/) » TCEC, Leela Chess Zero

# Forum Posts

## 2008 ...

- [Stockfish 1.0](http://www.talkchess.com/forum/viewtopic.php?t=24675) by Marco Costalba, CCC, November 02, 2008
- [Please drop Stockfish](http://www.talkchess.com/forum/viewtopic.php?t=24771) by Marco Costalba, CCC, November 07, 2008

2009

- [Re: Stockfish - Glaurung](http://wbec-ridderkerk.forumotion.com/wbec-ridderkerk-news-info-f1/stockfish-glaurung-t402.htm) by Tord Romstad, [WBEC-Ridderkerk forum](http://wbec-ridderkerk.forumotion.com/forum.htm), September 05, 2009
- [Stockfish 1.5.1](http://www.talkchess.com/forum/viewtopic.php?t=30051) by Marco Costalba, CCC, October 08, 2009

## 2010 ...

- [Stockfish 1.7](http://www.talkchess.com/forum/viewtopic.php?t=33677) by Marco Costalba, CCC, April 08, 2010
- [Stockfish-1.7.0 Hyper-threading Detection](http://www.talkchess.com/forum/viewtopic.php?t=33705) by Louis Zulli, CCC, April 09, 2010 » Thread
- [stockfish fail high fail low](http://www.talkchess.com/forum/viewtopic.php?t=33779) by Uri Blass, CCC, April 13, 2010
- [MTD experiment with stockfish 1.7.1](http://www.talkchess.com/forum/viewtopic.php?t=33813) by Vratko Polák, CCC, April 15, 2010
- [about stockfish and logic](http://www.talkchess.com/forum/viewtopic.php?t=33842) by Uri Blass, CCC, April 17, 2010
- [Stockfish - material balance/imbalance evaluation](http://www.talkchess.com/forum/viewtopic.php?t=34159) by Ralph Stoesser, CCC, May 05, 2010
- [Qsearch of Stockfish 1.7.1](http://www.talkchess.com/forum/viewtopic.php?t=34286) by Ferdinand Mosca, CCC, May 13, 2010
- [Stockfish do_move + undo_move](http://www.talkchess.com/forum/viewtopic.php?t=34670) by Matthew Purland, CCC, June 02, 2010
- [static null move pruning is stockfish](http://www.talkchess.com/forum/viewtopic.php?t=34909) by Tom King, CCC, June 13, 2010
- [Stockfish - single evasion extensions](http://www.talkchess.com/forum/viewtopic.php?t=35186) by Ralph Stoesser, CCC, June 27, 2010
- [Stockfish 1.8 JA available](http://www.talkchess.com/forum/viewtopic.php?t=35246) by Jim Ablett, CCC, July 02, 2010
- [stockfish 1.8 - Eval hash gone?](http://www.talkchess.com/forum/viewtopic.php?t=35284) by Edward Yu, CCC, July 04, 2010
- [Stockfish Singular Extension, does it make sense?](http://www.talkchess.com/forum/viewtopic.php?t=35419) by Volker Böhm, CCC, July 08, 2010
- [Stockfish 1.8 tweaks](http://www.talkchess.com/forum/viewtopic.php?t=35355) by Vratko Polák, CCC, July 09, 2010
- [Stockfish question](http://www.open-chess.org/viewtopic.php?f=3&t=423) by Rebel, OpenChess Programming Forum, July 10, 2010
- [Taken from CCC (Stockfish & mainlines)](http://www.open-chess.org/viewtopic.php?f=5&t=434) by Rebel, OpenChess Programming Forum, July 12, 2010
- [backward pawns in Stockfish](http://www.talkchess.com/forum/viewtopic.php?t=35459) by Marek Kwiatkowski, CCC, July 16, 2010
- [Questions for the Stockfish team](http://www.talkchess.com/forum/viewtopic.php?t=35455) by Michael Sherwin, CCC, July 16, 2010
- [Stockfish 1.8 - eval cache](http://www.talkchess.com/forum/viewtopic.php?t=35496) by Ralph Stoesser, CCC, July 18, 2010 » Evaluation Hash Table
- [Stockfish null move pre-condition](http://www.talkchess.com/forum/viewtopic.php?t=35543) by Rein Halbersma, CCC, July 22, 2010 » Null Move Pruning
- [Stockfish for 39 dollars](http://www.talkchess.com/forum/viewtopic.php?t=35901) by Matthias Gemuh, CCC, August 26, 2010
- [Stockfish 1.9 JA update available](http://www.talkchess.com/forum/viewtopic.php?t=36239) by Jim Ablett, CCC, October 02, 2010
- [mobility evaluation of stockfish](http://www.talkchess.com/forum/viewtopic.php?t=36307) by Uri Blass, CCC, October 09, 2010

2011

- [Stockfish 2.0 Available](http://www.talkchess.com/forum/viewtopic.php?t=37399) by Jim Ablett, CCC, January 01, 2011
- [Stockfish 2.0.0 tests](http://www.talkchess.com/forum/viewtopic.php?t=37450) by Harun Taner, CCC, January 04, 2011
- [Stockfish "Use Sleeping Threads" Test](http://www.talkchess.com/forum/viewtopic.php?t=37468) by Louis Zulli, CCC, January 05, 2011
- [StockFish engine](http://www.talkchess.com/forum/viewtopic.php?t=37573) by Andriy Dzyben, CCC, January 11, 2011
- [Designing an analysis friendly Stockfish?](http://www.open-chess.org/viewtopic.php?f=5&t=1042) by Uly, Open Chess Programming Forum, January 28, 2011
- [Why are the Ippo derivative stronger than Stockfish?](http://www.talkchess.com/forum/viewtopic.php?t=38198) by Larry Kaufman, CCC, 24 February, 2011
- [Transposition Table updates in Stockfish](http://www.talkchess.com/forum/viewtopic.php?t=38740) by Onno Garms, CCC, April 12, 2011 » Transposition Table
- [Stockfish random generator (rkiss.h)](http://www.talkchess.com/forum/viewtopic.php?t=38760) by Martin Sedlak, CCC, Apr 15, 2011 » Bob Jenkins
- [futility pruning in stockfish](http://www.talkchess.com/forum/viewtopic.php?t=39169) by Engin Üstün, CCC, May 25, 2011 » Futility Pruning
- [Stockfish clones in the AppStore: it's becoming a plague...](http://www.talkchess.com/forum/viewtopic.php?t=39214) by Julien Marcel, CCC, May 28, 2011 » Clones
- [Root node search in Stockfish](http://www.talkchess.com/forum/viewtopic.php?t=39346) by Onno Garms, CCC, June 12, 2011 » Move Ordering, Root
- [Grandmaster prefers Stockfish evals](http://www.talkchess.com/forum/viewtopic.php?t=40562) by Albert Silver, CCC, September 29, 2011
- [Stockfish on github](http://www.talkchess.com/forum/viewtopic.php?t=40610) by Marco Costalba, CCC, October 02, 2011
- [Stockfish's tuning method](http://www.talkchess.com/forum/viewtopic.php?t=40662) by Joona Kiiski, CCC, October 07, 2011 » Stockfish's Tuning Method

2012

- [StockFish LS with LimitStrength feature](http://www.talkchess.com/forum/viewtopic.php?t=41732) by Alexander Schmidt, CCC, January 01, 2012
- [Stockfish Code ( Piece Value's)](http://www.talkchess.com/forum/viewtopic.php?t=41916) by Nolan Denson, CCC, January 10, 2012 » Point Value
- [Stockfish hash implementation](http://www.talkchess.com/forum/viewtopic.php?t=41917) by Jon Dart, CCC, January 10, 2012 » Transposition Table
- [Stockfish 2.2.2 JA update available](http://www.talkchess.com/forum/viewtopic.php?t=41999) by Jim Ablett, CCC, January 14, 2012
- [CLOP on Stockfish](http://www.talkchess.com/forum/viewtopic.php?p=454327) by Gary, CCC, March 10, 2012 » CLOP
- [optimal aspiration window for stockfish question](http://www.talkchess.com/forum/viewtopic.php?t=42841) by Uri Blass, CCC, March 12, 2012 » Aspiration Windows
- [Raspberry Pi / Stockfish dedicated chess computer/board](http://www.talkchess.com/forum/viewtopic.php?t=44901) by Jean-Francois Romang, CCC, August 26, 2012 » Raspberry Pi, Dedicated Chess Computers
- [Stockfish 2.3 update available](http://www.talkchess.com/forum/viewtopic.php?t=45163) by Jim Ablett, CCC, September 15, 2012

2013

- [10 Lessons to be Learned from todays Top Engines](http://rybkaforum.net/cgi-bin/rybkaforum/topic_show.pl?tid=26392) by Josef, Rybka Forum, January 03, 2013 » Houdini, Komodo
- [Stockfish 3 Official JA Windows/Linux builds available](http://www.talkchess.com/forum/viewtopic.php?t=47881) by Jim Ablett, CCC, April 30, 2013
- [Fishtest Distributed Testing Framework](http://www.talkchess.com/forum/viewtopic.php?t=47885) by Marco Costalba, CCC, May 01, 2013 » Fishtest
- [Re: History pruning / move ordering question](http://www.talkchess.com/forum/viewtopic.php?t=47953&start=10) by Joona Kiiski, CCC, May 12, 2013 » Countermove Heuristic
- [Stockfish 3 PA_GTB](http://open-chess.org/viewtopic.php?f=7&t=2322) by Jeremy Bernstein, OpenChess Forum, May 15, 2013
- [Probcut](http://www.talkchess.com/forum/viewtopic.php?p=518426) by Gary, CCC, May 24, 2013 » ProbCut
- [Stockfish bug](http://www.talkchess.com/forum/viewtopic.php?t=48149) by Steven Atkinson, CCC, May 30, 2013 » Repetitions
- [The Ultimate Stockfish!](http://www.talkchess.com/forum/viewtopic.php?t=48602) by Mike Scheidl, CCC, July 09, 2013
- [use sleeping threads](http://www.talkchess.com/forum/viewtopic.php?t=48612) by Don Dailey, CCC, July 10, 2013 » Parallel Search, Thread
- [Stockfish 4](http://www.talkchess.com/forum/viewtopic.php?t=49035) by Marco Costalba, CCC, August 20, 2013
- [18 days from SF4 release and about ~30+ ELO gain!](http://www.talkchess.com/forum/viewtopic.php?t=49283) by Alexandre Meirelles Souza, CCC, September 08, 2013
- [How much of Stockfish code is still from Tord Romstad?](http://www.talkchess.com/forum/viewtopic.php?t=49375) by Jouni Uski, CCC, September 16, 2013
- [Syzygy tablebases, work in Stockfish?](http://www.talkchess.com/forum/viewtopic.php?t=49439) by Jose Mº Velasco, CCC, September 23, 2013 » Syzygy Bases
- [Stockfish search](http://www.talkchess.com/forum/viewtopic.php?t=49854) by Harm Geert Muller, CCC, October 28, 2013 » Principal Variation
- [Some food for thought](http://hiarcs.net/forums/viewtopic.php?t=6425) by Spacious Mind, Hiarcs Forum, November 11, 2013 » Stockfish vs. Tasc CM32 512K The King 2.2
- [Stockfish scaling](http://www.talkchess.com/forum/viewtopic.php?t=50083) by Ed Schröder, CCC, November 15, 2013
- [Stockfish depth vs. others; challenge](http://www.talkchess.com/forum/viewtopic.php?t=50220) by Larry Kaufman, CCC, November 24, 2013 » Depth
- [Stockfish DD: a new official release](http://www.talkchess.com/forum/viewtopic.php?t=50275) by Marco Costalba, CCC, November 29, 2013 » TCEC Season 5, dedicated to Don Dailey
- [Stockfish Syzygy: how to interpret mates?](http://www.talkchess.com/forum/viewtopic.php?t=50296) by Jouni Uski, CCC, December 01, 2013 » Syzygy Bases, Mate Scores
- [Is SF DD greater efficiency would be null move pruning?](http://www.talkchess.com/forum/viewtopic.php?t=50587) by Jonathan Lee, CCC, December 22, 2013 » Null Move Pruning

2014

- [Help me to test an idea for Stockfish](http://www.talkchess.com/forum/viewtopic.php?t=50742) by Robert Tournevisse, CCC, January 03, 2014 » Piece-Square Tables, Tapered Eval
- [Stockfish seems definitely the strongest engine](http://www.talkchess.com/forum/viewtopic.php?t=50989) by Kai Laskos, CCC, January 21, 2014
- [Stockfish Mac app](http://www.talkchess.com/forum/viewtopic.php?t=50992) by Daylen Yang, CCC, January 22, 2014 » Macintosh
- [Stockfish goes EGBB](http://www.talkchess.com/forum/viewtopic.php?t=51096) by Daniel Shawul, CCC, January 29, 2014 » Scorpio Bitbases
- [fixing the null move search "bug"](http://www.talkchess.com/forum/viewtopic.php?t=51129) by Uri Blass, CCC, February 01, 2014 » Null Move Pruning
- [Disabling Null Move Pruning in Stockfish](http://www.talkchess.com/forum/viewtopic.php?t=51291) by Louis Zulli, CCC, February 15, 2014 » Null Move Pruning
- [Threads-Test](http://www.talkchess.com/forum/viewtopic.php?t=51655) by Andreas Strangmüller, CCC, March 18, 2014 » Thread, Parallel Search
- [Stockfish haswell optimized build](http://www.talkchess.com/forum/viewtopic.php?t=51879) by Jean-Francois Romang, CCC, April 06, 2014 » BMI2
- [Huge simplification](http://www.talkchess.com/forum/viewtopic.php?t=52117&start=1) by Lyudmil Tsvetkov, CCC, April 25, 2014 » Pawn Chain
- [Stockfish zero evals](http://www.talkchess.com/forum/viewtopic.php?t=52204) by Larry Kaufman, CCC, May 02, 2014
- [Threads-Test - SF, Zappa, Komodo - 1 vs. 2, 4, 8, 16 Threads](http://www.talkchess.com/forum/viewtopic.php?t=52219) by Andreas Strangmüller, CCC, May 04, 2014 » Thread, Stockfish, Zappa, Komodo
- [investigating why stockfish is strong idea](http://www.talkchess.com/forum/viewtopic.php?t=52231) by Uri Blass, CCC, May 05, 2014
- [Threads factor: Komodo, Houdini, Stockfish and Zappa](http://www.talkchess.com/forum/viewtopic.php?p=570955) by Andreas Strangmüller, CCC, May 17, 2014 » Komodo, Houdini, Stockfish, Zappa
- [Goodbye CLOP, hello SPSA](https://groups.google.com/d/msg/fishcooking/WNrxeXAJ6VI/ZkCnRv4I_qEJ) by Gary Linscott, FishCooking, May 17, 2014 » CLOP, SPSA
- [Stockfish 5](http://www.talkchess.com/forum/viewtopic.php?t=52487) by Marco Costalba, CCC, May 31, 2014
- [Stockfish Status Report](http://www.talkchess.com/forum/viewtopic.php?t=52781) by Louis Zulli, CCC, June 27, 2014
- [GM and Rybka vs. Stockfish](http://www.talkchess.com/forum/viewtopic.php?t=53228) by Robert Maddox, CCC, August 09, 2014 » GM+Rybka vs. Stockfish
- [Nakamura vs Stockfish, public match 8/23](http://www.talkchess.com/forum/viewtopic.php?t=53315) by Jesse L, CCC, August 17, 2014
- [Using the Transposition Table for long searches](https://groups.google.com/d/msg/fishcooking/6nNXAQQAXOE/FXs2chqDargJ) by Theodr Elwurtz, FishCooking, September 22, 2014 » Transposition Table
- [Rule of the square](https://groups.google.com/d/msg/fishcooking/T7OFWxD4LK8/pzurkRQNLjwJ) by Mikael, FishCooking, September 24, 2014 » Rule of the Square
- [Using the Stockfish position evaluation score to predict victory probability](https://chesscomputer.tumblr.com/post/98632536555/using-the-stockfish-position-evaluation-score-to/embed) by unavoidablegrain, [Tumblr](https://en.wikipedia.org/wiki/Tumblr), September 28, 2014 » Pawn Advantage, Win Percentage, and Elo
- [Threads test incl. Stockfish 5 and Komodo 8](http://www.talkchess.com/forum/viewtopic.php?t=53995) by Andreas Strangmüller, CCC, October 09, 2014
- [Threads test - Stockfish 5 against Komodo 8](http://www.talkchess.com/forum/viewtopic.php?t=54009) by Andreas Strangmüller, CCC, October 10, 2014 » Thread, Parallel Search, Stockfish, Komodo
- [Stockfish and accurate PV](http://www.talkchess.com/forum/viewtopic.php?t=54750) by Matthew Lai, CCC, December 25, 2014 » Principal Variation
- [Stockfish 32-bit and hardware instructions on MSVC++](http://www.talkchess.com/forum/viewtopic.php?t=54798) by Syed Fahad, CCC, December 30, 2014 » BitScan, Population Count

## 2015 ...

- [Stockfish in Lozza UIs](http://www.talkchess.com/forum/viewtopic.php?t=54891) by Colin Jenkins, CCC, January 07, 2015 » Lozza, Stockfish-js 40
- [SF6 has been released](http://www.talkchess.com/forum/viewtopic.php?t=55122) by Joona Kiiski, CCC, January 27, 2015
- [Stockfish 6 is impressive in Behting study](http://www.talkchess.com/forum/viewtopic.php?t=55167) by Jouni Uski, CCC, January 31, 2015 » Behting Study
- [Stockfish with 16 threads - big news?](http://www.talkchess.com/forum/viewtopic.php?t=55352) by Louis Zulli, CCC, February 15, 2015 » Thread

[Explanation for non-expert?](http://www.talkchess.com/forum/viewtopic.php?t=55368) by Louis Zulli, CCC, February 16, 2015 » Parallel Search

- [Stockfish still scales poorly?](http://www.talkchess.com/forum/viewtopic.php?t=55402) by Louis Zulli, CCC, February 20, 2015
- [Measuring SF idle time](http://www.talkchess.com/forum/viewtopic.php?t=55409) by Louis Zulli, CCC, February 21, 2015
- [Better NPS scaling for Stockfish](http://www.talkchess.com/forum/viewtopic.php?t=55494) by Louis Zulli, CCC, February 27, 2015
- [Stockfish Questions](http://www.talkchess.com/forum/viewtopic.php?t=55510) by Syed Fahad, CCC, February 28, 2015
- [Best Stockfish NPS scaling yet](http://www.talkchess.com/forum/viewtopic.php?t=55536) by Louis Zulli, CCC, March 02, 2015
- [Stockfish contempt factor](http://www.talkchess.com/forum/viewtopic.php?t=55623) by Kai Laskos, CCC, March 10, 2015 » Contempt Factor
- [Improving SF passer code](http://www.talkchess.com/forum/viewtopic.php?t=55792) by Lyudmil Tsvetkov, CCC, March 26, 2015 » Connected Passed Pawns
- [Problem with SF6 and Syzygy TB](http://www.talkchess.com/forum/viewtopic.php?t=55846) by Forrest Hoch, CCC, April 01, 2015 » Syzygy Bases
- [Empirical results with Lazy SMP, YBWC, DTS](http://www.talkchess.com/forum/viewtopic.php?t=56019) by Kai Laskos, CCC, April 16, 2015 » Lazy SMP, YBWC, DTS
- [The effective speedup from 1 to 8 cpus for SF and Komodo](http://www.talkchess.com/forum/viewtopic.php?t=56543) by Adam Hair, CCC, May 31, 2015 » Parallel Search, Komodo
- [New Stockfish with Lazy_SMP, but what about the TC bug ?](http://www.talkchess.com/forum/viewtopic.php?t=58056) by Ernest Bonnem, CCC, October 26, 2015 » Parallel Search, TCEC Season 8
- [Binary for TCEC superfinal](https://goo.gl/QLcPAF) by Kiran Panditrao, FishCooking, October 30, 2015 » TCEC Season 8
- [SF binaries for TCEC superfinal](http://www.talkchess.com/forum/viewtopic.php?t=58103) by Marco Costalba, CCC, October 31, 2015
- [Stockfish dev 091115 for ANDROID](http://www.talkchess.com/forum/viewtopic.php?t=58210) by Nathanael Russell, CCC, November 09, 2015 » Android
- [Stockfish now benefits from hyperthreading](http://www.talkchess.com/forum/viewtopic.php?t=58236) by Dmitri Gusev, CCC, November 12, 2015 » Thread
- [Stockfish 7 beta 1](http://www.talkchess.com/forum/viewtopic.php?t=58703) by Joona Kiiski, CCC, December 27, 2015
- [Another GHI example in SF (maybe)](http://www.open-chess.org/viewtopic.php?f=5&t=2942) by BB+, OpenChess Forum, December 30, 2015 » Graph History Interaction

2016

- [Stockfish 7](http://www.talkchess.com/forum/viewtopic.php?t=58779) by Joona Kiiski, CCC, January 02, 2016
- [Threads test incl. Stockfish 7](http://www.talkchess.com/forum/viewtopic.php?t=58887) by Andreas Strangmüller, CCC, January 11, 2016 » Thread, Parallel Search
- [Stockfish 7 progress](http://www.talkchess.com/forum/viewtopic.php?t=58935) by Carl Lumma, CCC, January 16, 2016
- [Oddity around depths 7-8 with Stockfish 6 & 7](http://www.talkchess.com/forum/viewtopic.php?t=58990) by Ken Regan, CCC, January 21, 2016
- [Stockfish 7 and partial 6 piece syzygy problem?](http://www.talkchess.com/forum/viewtopic.php?t=59407) by Jouni Uski, CCC, March 01, 2016

[Re: Stockfish 7 and partial 6 piece syzygy problem?](http://www.talkchess.com/forum/viewtopic.php?t=59407&start=12) by Marco Costalba, CCC, September 01, 2016

- [Computer Chess Progress: Stockfish 7 vs Ruffian 1.0.5](http://www.talkchess.com/forum/viewtopic.php?t=59543) by Martin Fierz, CCC, March 17, 2016 » Ruffian
- [Natural TB](http://www.talkchess.com/forum/viewtopic.php?t=60312) by Marco Costalba, CCC, May 29, 2016 » Syzygy Bases
- [Stockfish eval output](http://www.talkchess.com/forum/viewtopic.php?t=61250) by Erin Dame, CCC, August 27, 2016 » Wrong Color Bishop and Rook Pawn
- [Re: Beginner's guide to graphical profiling](http://www.talkchess.com/forum/viewtopic.php?t=61373&start=2) by Marco Costalba, CCC, September 10, 2016 » Profiling
- [ELO inflation ha ha ha](http://www.talkchess.com/forum/viewtopic.php?t=61444) by Henk van den Belt, CCC, September 16, 2016 » Delphil, Match Statistics, Playing Strength, TCEC Season 9 41
- [pin-aware see](https://groups.google.com/d/msg/fishcooking/S_4E_Xs5HaE/mS3VTnuEFgAJ) by Ronald de Man, FishCooking, September 14, 2016 » SEE - The Swap Algorithm, Pin
- [Illegal moves in SEE](https://groups.google.com/d/msg/fishcooking/9mcmjnyqbAQ/S6mDA0QsAAAJ) by Stephane Nicolet, FishCooking, September 22, 2016 » SEE - The Swap Algorithm
- [Stockfish underpromotes much more often than Komodo](http://www.talkchess.com/forum/viewtopic.php?t=61601) by Kai Laskos, CCC, October 05, 2016 » Komodo, Match Statistics, Promotions
- [couple of questions about stockfish code ?](http://www.talkchess.com/forum/viewtopic.php?t=61850) by Mahmoud Uthman, CCC, October 26, 2016 » SIMD and SWAR Techniques, Tapered Eval
- [Stockfish 8](https://groups.google.com/d/msg/fishcooking/LCoojE9O5jU/h6xgvg2EBgAJ) by Marco Costalba, FishCooking, November 01, 2016
- [Stockfish 8 official](http://www.talkchess.com/forum/viewtopic.php?t=61924) by Marco Costalba, CCC, November 01, 2016
- [NUMA test compilation](https://groups.google.com/d/msg/fishcooking/7hHC075ZnnM/IaITCiLaBwAJ) by Joachim Müller, FishCooking, November 05, 2016 » NUMA
- [Stockfish 8 - Double time control vs. 2 threads](http://www.talkchess.com/forum/viewtopic.php?t=62146) by Andreas Strangmüller, CCC, November 15, 2016 » Doubling TC, Diminishing Returns, Playing Strength, Lazy SMP
- [Stockfish benchmark data](http://www.open-chess.org/viewtopic.php?f=3&t=3044) by Adam Hair, OpenChess Forum, November 27, 2016
- [The new chess rules (5-fold repetition and 75-move draw)](https://groups.google.com/d/msg/fishcooking/M2bkzC3MuFQ/N3pHK4DcAgAJ) by Lyudmil Antonov, FishCooking, November 29, 2016 » Repetitions, Fifty-move Rule
- [Scoutfish: powerful chess query tool](http://www.talkchess.com/forum/viewtopic.php?t=62452) by Marco Costalba, CCC, December 11, 2016 » Databases, Portable Game Notation, Scoutfish

2017

- [SF Progression since Fishtest inception](http://www.talkchess.com/forum/viewtopic.php?t=62822) by Adam Hair, CCC, January 14, 2017 » Fishtest
- [Re: Chessprogams with the most chessknowing](http://www.talkchess.com/forum/viewtopic.php?t=54697&start=50) by Marco Costalba, CCC, February 19, 2017 » Search versus Evaluation
- [Stockfish bench in i486 & Pentium 75mhz !](http://www.talkchess.com/forum3/viewtopic.php?f=2&t=63857) by hammerklavier, CCC, April 29, 2017

Re: Stockfish bench ...

- [Symmetric multiprocessing (SMP) scaling - SF8 and K10.4](http://www.talkchess.com/forum/viewtopic.php?t=63903) by Andreas Strangmüller, CCC, May 05, 2017 » Lazy SMP, Komodo
- [Symmetric multiprocessing (SMP) scaling - SF8 Contempt=10](http://www.talkchess.com/forum/viewtopic.php?t=63967) by Andreas Strangmüller, CCC, May 13, 2017 » SMP, Contempt Factor
- [Stockfish version with hash saving capability](http://www.talkchess.com/forum/viewtopic.php?t=64720) by Daniel José Queraltó, CCC, July 25, 2017 » Persistent Hash Table
- [Natural TB (take 2)](http://www.talkchess.com/forum/viewtopic.php?t=60312&start=240) by Marco Costalba, CCC, August 22, 2017 » Syzygy Bases
- [Approximating Stockfish's Evaluation by PSQTs](http://www.talkchess.com/forum/viewtopic.php?t=64972) by Thomas Dybdahl Ahle, CCC, August 23, 2017 » Regression, Piece-Square Tables
- [Stockfish no progress in 2month and half , why ?](http://www.talkchess.com/forum/viewtopic.php?t=65017) by Jean Baptiste, CCC, August 28, 2017
- [Stockfish testing at STC and LTC: one question](http://www.talkchess.com/forum/viewtopic.php?t=65216) by Jouni Uski, CCC, September 19, 2017
- [Scaling from FGRL results with top 3 engines](http://www.talkchess.com/forum/viewtopic.php?t=65288) by Kai Laskos, CCC, September 26, 2017 » FGRL, Houdini, Komodo
- [AlphaZero vs Stockfish](http://www.talkchess.com/forum/viewtopic.php?t=65919) by Bigler, CCC, December 06, 2017 » AlphaZero vs. Stockfish
- [A branch to test the Monte Carlo algorithm in Stockfish](https://groups.google.com/forum/#!topic/fishcooking/AE4EgWQ20dY) by Stephane Nicolet, FishCooking, December 06, 2017 » Monte-Carlo Tree Search, AlphaZero
- [Reactions about AlphaZero from top GMs...](http://www.talkchess.com/forum/viewtopic.php?t=65934) by Norman Schmidt, CCC, December 08, 2017 » AlphaZero: Reactions From Top GMs, Stockfish Author, Tord Romstad
- [MCTS wrapper for StockFish](https://groups.google.com/d/msg/fishcooking/rMCfc8zMerc/F01WuNtDCgAJ) by Jonathan Baxter, FishCooking, December 19, 2017 » Monte-Carlo Tree Search

2018

- [Stockfish 8 - Initial position until depth 59](http://www.talkchess.com/forum/viewtopic.php?t=66340) by Andreas Strangmüller, CCC, January 16, 2018 » Initial Position
- [New Stockfish contempt](http://www.talkchess.com/forum/viewtopic.php?t=66444) by Jouni Uski, CCC, January 29, 2018 » Contempt Factor
- [Contributors in the last two years](https://groups.google.com/d/msg/fishcooking/_FW_RIowarw/y1e-qMEXAgAJ) by Stephane Nicolet, FishCooking, Jnauary 30, 2018
- [Stockfish 9](http://www.talkchess.com/forum/viewtopic.php?t=66470) by Marco Costalba, CCC, February 01, 2018
- [Elo measurement of contempt in SF in self-play](http://www.talkchess.com/forum/viewtopic.php?t=66793) by Michel Van den Bergh, CCC, March 10, 2018 » Contempt, Playing Strength
- [Stockfish 180113 - Initial position until depth 65](http://www.talkchess.com/forum/viewtopic.php?t=66935) by Andreas Strangmüller, CCC, March 27, 2018 » Initial Position
- [Stockfish and serious hardware: 384 threads](http://www.talkchess.com/forum3/viewtopic.php?f=2&t=67932) by Jouni Uski, CCC, July 08, 2018 » Thread
- [Stockfish 10 - Call for Binaries](https://groups.google.com/d/msg/fishcooking/kJ6vNKyp6h8/zwRnc-i7CwAJ) by Daylen Yang, FishCooking, November 29, 2018
- [Re: piece lists advantage with bit-boards?](http://www.talkchess.com/forum3/viewtopic.php?f=7&t=69364&start=12) by Ronald de Man, CCC, December 26, 2018 » Piece-Lists, asmFish

2019

- [Re: What's the best Lazy SMP logic?](http://www.talkchess.com/forum3/viewtopic.php?f=7&t=69507&start=1) by Sven Schüle, CCC, January 06, 2019 » Lazy SMP in Stockfish
- [Training the trainer: how is it done for Stockfish?](http://www.talkchess.com/forum3/viewtopic.php?f=7&t=70069) by Marc-Philippe Huget, CCC, March 01, 2019 » Monte-Carlo Tree Search
- [Some NUMA data for Stockfish-dev and Cfish-dev](http://www.talkchess.com/forum3/viewtopic.php?f=7&t=71027) by Louis Zulli, CCC, June 17, 2019 » NUMA, CFish
- [Why does stockfish randomise draw evaluations?](http://www.talkchess.com/forum3/viewtopic.php?f=7&t=71707) by Vincent Tang, CCC, September 01, 2019 » Draw, Draw Evaluation, Draw Score, Search with Random Leaf Values
- [Help needed testing vectorized Stockfish pawns.cpp...](https://groups.google.com/d/msg/fishcooking/xGM9K7wd5rM/pmx2MVX-BwAJ) by Nick Pelling, FishCooking, September 23, 2019
- [mg vs eg eval](https://groups.google.com/d/msg/fishcooking/znU1a7aZ2XI/yJDFtOQnAwAJ) by Joost VandeVondele, FishCooking, October 06, 2019 » Middlegame, Endgame, Tapered Eval
- [Stockfish contempt testing](https://groups.google.com/d/msg/fishcooking/liMe2Ho53j8/GP9l07hSBAAJ) by Leonardo Ljubičić, FishCooking, October 29, 2019 » Contempt
- [some questions about singular search in Stockfish](http://www.talkchess.com/forum3/viewtopic.php?f=7&t=72231) by Jon Dart, CCC, November 01, 2019 » Singular Extensions
- ["stat score bonus" in stockfish](http://www.talkchess.com/forum3/viewtopic.php?f=7&t=72232) by Vivien Clauzon, CCC, November 01, 2019
- [Stockfish 10 was released 29.11.2018](http://www.talkchess.com/forum3/viewtopic.php?f=2&t=72480) by Jouni Uski, CCC, December 01, 2019

## 2020 ...

- [lazy smp behaviour of stockfish](https://groups.google.com/d/msg/fishcooking/9X3lDH83tlk/DtRtuFMOCAAJ) by Daniel Shawul, FishCooking, January 05, 2020 » Lazy SMP
- [Stockfish 11](https://www.talkchess.com/forum3/viewtopic.php?f=2&t=72837) by Stephane Nicolet, CCC, January 18, 2020
- [Stockfish Reverts 5 Recent Patches](https://www.talkchess.com/forum3/viewtopic.php?f=7&t=72962) by Deberger, CCC, February 01, 2020

[Re: Stockfish Reverts 5 Recent Patches](https://www.talkchess.com/forum3/viewtopic.php?f=7&t=72962&start=6) by Michel Van den Bergh, CCC, February 02, 2020 » SPRT

- [Stockfish and latest +6 ELO patch!](https://www.talkchess.com/forum3/viewtopic.php?f=2&t=73273) by Jouni Uski, CCC, March 05, 2020 » Distance, Space-Time Tradeoff 42
- [Null move](https://www.talkchess.com/forum3/viewtopic.php?f=7&t=73753) by Robert Pope, CCC, April 24, 2020 » Null Move Pruning
- [Stockfish_dev is probably stronger than Sargon 1978 v1.00](https://www.talkchess.com/forum3/viewtopic.php?f=2&t=74037) by Kai Laskos, CCC, May 29, 2020 » Sargon
- [Stockfish has included WDL stats in engine output](https://www.talkchess.com/forum3/viewtopic.php?f=7&t=74339) by Deberger, CCC, July 02, 2020 » Pawn Advantage, Win Percentage, and Elo
- [stockfishNNUE vs others (TCEC 18 bonus)](https://groups.google.com/d/msg/fishcooking/EBKQSrb9I08/5xasTnnSCAAJ) by Warren D. Smith, FishCooking, July 14, 2020
- [Stockfish 12](https://groups.google.com/d/msg/fishcooking/TJHsiI61yQ4/liQoZ-AzAgAJ) by Joost VandeVondele, FishCooking, September 02, 2020
- [Stockfish 12 is released today!](https://www.talkchess.com/forum3/viewtopic.php?f=2&t=74974) by Nay Lin Tun, CCC, September 02, 2020
- [Stockfish 12 has arrived!](https://www.talkchess.com/forum3/viewtopic.php?f=2&t=74978) by daniel71, CCC, September 02, 2020
- [Re: Stockfish bench in i486 & Pentium 75mhz !](http://www.talkchess.com/forum3/viewtopic.php?f=2&t=63857&start=14) by Vincent Lejeune, CCC, October 11, 2020 » Stockfish bench ...
- [Re: Raspberry Pi 4 compiled chess engines](http://www.talkchess.com/forum3/viewtopic.php?f=7&t=75841&start=8) by Rasmus Althoff, CCC, November 16, 2020 » Raspberry Pi

2021

- [Stockfish 13](https://groups.google.com/g/fishcooking/c/AzYDbbv-Coo) by Joost VandeVondele, FishCooking, February 19, 2021
- [Stockfish 13 merged on github](https://www.talkchess.com/forum3/viewtopic.php?f=2&t=76639) by Joshua Shriver, CCC, February 19, 2021
- [Setting up Stockfish on a server](https://www.talkchess.com/forum3/viewtopic.php?f=7&t=76977) by Jon12345, CCC, March 29, 2021 » Chess Server
- [Joking FTW, Seriously](https://lczero.org/blog/2021/04/joking-ftw-seriously/) by borg, LCZero blog, April 25, 2021
- [The importance of open data](https://lczero.org/blog/2021/06/the-importance-of-open-data/) by borg , LCZero blog, June 15, 2021
- [will Tcec allow Stockfish with a Leela net to play?](https://www.talkchess.com/forum3/viewtopic.php?f=2&t=77503) by Wilson, CCC, June 17, 2021 » TCEC Season 21

[Re: will Tcec allow Stockfish with a Leela net to play?](https://www.talkchess.com/forum3/viewtopic.php?f=2&t=77503&start=55) by Connor McMonigle, CCC, June 17, 2021

- [Stockfish 14 release round the corner](https://www.talkchess.com/forum3/viewtopic.php?f=2&t=77599) by Prasanna Bandihole, CCC, July 02, 2021
- [Before things become more messy than they already are](https://www.talkchess.com/forum3/viewtopic.php?f=2&t=77602) by Ed Schröder, CCC, July 02, 2021
- [Stockfish 14 has been released](https://www.talkchess.com/forum3/viewtopic.php?f=2&t=77605) by Madeleine Birchfield, CCC, July 02, 2021
- [Stockfish: Our lawsuit against ChessBase](https://www.talkchess.com/forum3/viewtopic.php?f=2&t=77762) by Kurt Lanc, CCC, July 20, 2021
- [The Great Stockfish NPS Debate](https://www.talkchess.com/forum3/viewtopic.php?f=2&t=78030) by Dietrich Kappe, CCC, August 27, 2021

2022

- [Stockfish search](https://www.talkchess.com/forum3/viewtopic.php?f=7&t=79588) by Werewolf, CCC, March 26, 2022 » Lazy SMP
- [Stockfish 15 is ready](https://www.talkchess.com/forum3/viewtopic.php?f=2&t=79713) by Mehmet Karaman, CCC, April 19, 2022
- [Stockfish 15's Immortal Game?](https://www.talkchess.com/forum3/viewtopic.php?f=2&t=79793) by Graham Banks, CCC, May 01, 2022
- [Are tablebases useless for Stockfish15?](https://www.talkchess.com/forum3/viewtopic.php?f=2&t=80608) by Jouni, CCC, September 02, 2022
- [Stockfish 15.1 is ready](https://www.talkchess.com/forum3/viewtopic.php?f=2&t=81105) by Mehmet Karaman, CCC, December 05, 2022
- [SF branching factor](https://www.talkchess.com/forum3/viewtopic.php?f=2&t=81108) by Jouni, CCC, December 05, 2022

2023

- [Stockfish randomicity](https://www.talkchess.com/forum3/viewtopic.php?f=7&t=82620) by amchess, CCC, September 21, 2023 » Search
- [Why is Stockfish removing killer moves in move ordering?](https://talkchess.com/viewtopic.php?t=84049) by Paul Jérôme--Filio, CCC, June 24, 2024 » Search

# Blog Posts

- [Stockfish Blog](https://blog.stockfishchess.org/)

## 2014

- [One chess champion per laptop](http://tech.mit.edu/V133/N62/chess.html) by [Roberto Perez-Franco](http://www.mit.edu/~roberto/), MIT's [The Tech](https://en.wikipedia.org/wiki/The_Tech_%28newspaper%29), January 15, 2014 » TCEC Season 5

## 2015 ...

- [And then there were two](http://en.chessbase.com/post/john-hartmann-and-then-there-were-two) by [John Hartmann](http://en.chessbase.com/author/john-hartmann), ChessBase News, June 09, 2015 » Komodo, Stockfish
- [Depth of Satisficing](https://rjlipton.wordpress.com/2015/10/06/depth-of-satisficing/) by Ken Regan, [Gödel's Lost Letter and P=NP](https://rjlipton.wordpress.com/), October 06, 2015 » Depth, Match Statistics, Pawn Advantage, Win Percentage, and Elo, Stockfish, Komodo 45
- [A Chess Firewall at Zero?](https://rjlipton.wordpress.com/2016/01/21/a-chess-firewall-at-zero/) by Ken Regan, [Gödel's Lost Letter and P=NP](https://rjlipton.wordpress.com/), January 21, 2016
- [Stockfish 8](https://stockfishchess.org/blog/2016/stockfish-8/), November 01, 2016
- [Stockfish 9](https://stockfishchess.org/blog/2018/stockfish-9/), February 09, 2018
- [Stockfish 10](https://stockfishchess.org/blog/2018/stockfish-10/), December 01, 2018

## 2020 ...

- [Stockfish 11](https://stockfishchess.org/blog/2020/stockfish-11/), The Stockfish Team, January 15, 2020
- [Stockfish 12](https://stockfishchess.org/blog/2020/stockfish-12/), The Stockfish Team, September 02, 2020
- [Stockfish 13](https://stockfishchess.org/blog/2021/stockfish-13/), The Stockfish Team, February 19, 2021
- [Stockfish 14](https://stockfishchess.org/blog/2021/stockfish-14/), The Stockfish Team, July 02, 2021
- [Our lawsuit against ChessBase](https://stockfishchess.org/blog/2021/our-lawsuit-against-chessbase/), The Stockfish Team, July 20, 2021 » ChessBase, Fat Fritz 2, Houdini 6
- [Stockfish 14.1](https://stockfishchess.org/blog/2021/stockfish-14-1/), The Stockfish Team, October 28, 2021

# External Links

## Chess Engine

- [Stockfish - Open Source Chess Engine](https://stockfishchess.org/)
- [official-stockfish/Stockfish · GitHub](https://github.com/official-stockfish/Stockfish)
- [zamar · GitHub](https://github.com/zamar) by Joona Kiiski
- [NCM Stockfish Dev Builds](https://nextchessmove.com/dev-builds)
- [Stockfish Development Versions](https://abrok.eu/stockfish/) hosted by Roman Korba
- [Stockfish (chess) from Wikipedia](https://en.wikipedia.org/wiki/Stockfish_%28chess%29)

## Issues

- [Issues · official-stockfish/Stockfish · GitHub](https://github.com/official-stockfish/Stockfish/issues)

## Pull Requests

- [Pull requests · official-stockfish/Stockfish · GitHub](https://github.com/official-stockfish/Stockfish/pulls)

## Testing

- [Get Involved - Stockfish - Powerful Open Source Chess Engine](https://stockfishchess.org/get-involved/)
- [Stockfish Testing Framework](https://tests.stockfishchess.org/tests) » Fishtest
- [Stockfish Evaluation Guide](https://hxim.github.io/Stockfish-Evaluation-Guide/) » Stockfish Evaluation Guide

[Stockfish Evaluation Guide - NNUE](https://hxim.github.io/Stockfish-Evaluation-Guide/?p=nnue)

- [GitHub - glinscott/fishtest: Stockfish testing](https://github.com/glinscott/fishtest)

[Creating my first test · glinscott/fishtest Wiki · GitHub](https://github.com/glinscott/fishtest/wiki/Creating-my-first-test)

[Fishtest mathematics · glinscott/fishtest Wiki · GitHub](https://github.com/glinscott/fishtest/wiki/Fishtest-mathematics)

- [SPSA Tuner for Stockfish Chess Engine](https://github.com/zamar/spsa) » SPSA
- [FishCooking - Google Groups](https://groups.google.com/forum/#!forum/fishcooking) a discussion group for developers and testers of Stockfish chess engine
- [Adam's Computer Chess Pages: Stockfish Progression](https://adamsccpages.blogspot.com/p/sf-framework-history.html) by Adam Hair » Fishtest

## Rating Lists

- [Stockfish](http://www.computerchess.org.uk/ccrl/4040/cgi/compare_engines.cgi?family=Stockfish&print=Rating+list&print=Results+table&print=LOS+table&print=Ponder+hit+table&print=Eval+difference+table&print=Comopp+gamenum+table&print=Overlap+table&print=Score+with+common+opponents) from CCRL 40/15
- [Stockfish](http://computerchess.org.uk/ccrl/404/cgi/compare_engines.cgi?family=Stockfish&print=Rating+list&print=Results+table&print=LOS+table&print=Ponder+hit+table&print=Eval+difference+table&print=Comopp+gamenum+table&print=Overlap+table&print=Score+with+common+opponents) in CCRL Blitz

## Matches

- [Can a GM and Rybka beat Stockfish?](https://www.chess.com/article/view/how-rybka-and-i-tried-to-beat-the-strongest-chess-computer-in-the-world) by GM [Daniel Naroditsky](https://en.wikipedia.org/wiki/Daniel_Naroditsky), Chess.com, August 08, 2014 » GM+Rybka vs. Stockfish
- [Stockfish Outlasts "Rybkamura"](https://www.chess.com/news/stockfish-outlasts-nakamura-3634) by [FM Mike Klein](https://www.chess.com/article/view/chesscom-player-profiles-fm-mikeklein), Chess.com, August 24, 2014
- [AlphaZero: Reactions From Top GMs, Stockfish Author](https://www.chess.com/news/view/alphazero-reactions-from-top-gms-stockfish-author) by Peter Doggers, Chess.com, December 08, 2017 » AlphaZero vs. Stockfish

## Interviews

- [Computerschach, Interview with Tord Romstad (Norway), Joona Kiiski (Finland) and Marco Costalba (Italy)](http://www.schach-welt.de/schach/computerschach/interviews/romstad-kiiski-costalba-eng) by Frank Quisinsky, March 29, 2010
- [Stockfish 4 to play in the new season of TCEC | Chessdom - Short interview with the Stockfish team](http://www.chessdom.com/stockfish-4-to-play-in-the-new-season-of-tcec/), August 22, 2013 » TCEC, TCEC Season 5

## Videos

- How do modern chess engines work? | Video, Talk by Daylen Yang, [TNG | Big Techday 8](http://www.tngtech.com/tng-ueber-uns/bigtechday/big-techday-8.html), June 12, 2015
- Parallelism and Selectivity in Game Tree Search | Video, Talk by Tord Romstad, [TNG | Big Techday 8](http://www.tngtech.com/tng-ueber-uns/bigtechday/big-techday-8.html), June 12, 2015
- How Modern Chess Programs Work | Video by Tord Romstad, [flatMap(Oslo)](http://2017.flatmap.no/talks/romstad/), May 02, 2017

## Misc

- [Stockfish from Wikipedia](https://en.wikipedia.org/wiki/Stockfish)
- [Lofoten Stockfish Museum from Wikipedia](https://en.wikipedia.org/wiki/Lofoten_Stockfish_Museum)
- [Postcards from the Lofoten Islands](https://ruthhorowitz.wordpress.com/2012/05/29/postcards-from-the-lofoten-islands/) from [Giving Up The Ghost](https://ruthhorowitz.wordpress.com/), May 29, 2012 » Stockfish and Gulls

# References

Up one Level    The Stockfish icon was designed by [Klein Maetschke](http://iamkle.in/), [About - Stockfish](https://stockfishchess.org/about/)↩︎ [Stockfish - Open Source Chess Engine](https://stockfishchess.org/), The Stockfish 12 icon was designed by [Klein Maetschke](http://iamkle.in/), [About - Stockfish](https://stockfishchess.org/about/)↩︎ [Stockfish 7](http://www.talkchess.com/forum/viewtopic.php?t=58779) by Joona Kiiski, CCC, January 02, 2016↩︎ [Stockfish 1.0](http://www.talkchess.com/forum/viewtopic.php?t=24675) by Marco Costalba, CCC, November 02, 2008↩︎ [Re: Smaug: a new chess engine based on glaurung](http://www.talkchess.com/forum/viewtopic.php?t=26971&start=1) by Marco Costalba, CCC, March 12, 2009↩︎ David Silver, Thomas Hubert, Julian Schrittwieser, Ioannis Antonoglou, Matthew Lai, Arthur Guez, Marc Lanctot, Laurent Sifre, Dharshan Kumaran, Thore Graepel, Timothy Lillicrap, Karen Simonyan, Demis Hassabis (2017). Mastering Chess and Shogi by Self-Play with a General Reinforcement Learning Algorithm. [arXiv:1712.01815](https://arxiv.org/abs/1712.01815)↩︎ [Stockfish on github](http://www.talkchess.com/forum/viewtopic.php?t=40610) by Marco Costalba, CCC, October 02, 2011↩︎ [Stockfish NN release (NNUE)](http://www.talkchess.com/forum3/viewtopic.php?f=2&t=74059) by Henk Drost, CCC, May 31, 2020↩︎ [Stockfish 12](https://stockfishchess.org/blog/2020/stockfish-12/), The Stockfish Team, [Stockfish Blog](https://blog.stockfishchess.org/), September 02, 2020↩︎ [Stockfish 13](https://stockfishchess.org/blog/2021/stockfish-13/), The Stockfish Team, February 19, 2021↩︎ [Stockfish 14](https://stockfishchess.org/blog/2021/stockfish-14/), The Stockfish Team, July 02, 2021↩︎ [GitHub - Stockfish commit, Remove classical evaluation](https://github.com/official-stockfish/Stockfish/commit/af110e02ec96cdb46cf84c68252a1da15a902395)↩︎ [About - Stockfish](https://stockfishchess.org/about/)↩︎ [glinscott/fishtest · GitHub](https://github.com/glinscott/fishtest)↩︎ [Get Involved - Stockfish - Powerful Open Source Chess Engine](http://stockfishchess.org/get-involved/)↩︎ [Fishtest Distributed Testing Framework](http://www.talkchess.com/forum/viewtopic.php?t=47885) by Marco Costalba, CCC, May 01, 2013↩︎ [The Pyramid Web Framework — The Pyramid Web Framework v1.5](http://docs.pylonsproject.org/projects/pyramid/en/latest/)↩︎ [Stockfish Testing Framework - Users](http://tests.stockfishchess.org/users)↩︎ [Stockfish Testing Framework](http://tests.stockfishchess.org/tests)↩︎ [Adam's Computer Chess Pages: Stockfish Progression](https://adamsccpages.blogspot.com/p/sf-framework-history.html) by Adam Hair↩︎ [Re: How far away are we from deep learning Stockfish, Komodo](http://www.talkchess.com/forum/viewtopic.php?t=64025&start=27) by Gary, CCC, May 21, 2017↩︎ [Stockfish Evaluation Guide](https://hxim.github.io/Stockfish-Evaluation-Guide/)↩︎ [Stockfish Evaluation Guide - NNUE](https://hxim.github.io/Stockfish-Evaluation-Guide/?p=nnue)↩︎ [Can a GM and Rybka beat Stockfish?](https://www.chess.com/article/view/how-rybka-and-i-tried-to-beat-the-strongest-chess-computer-in-the-world) by GM [Daniel Naroditsky](https://en.wikipedia.org/wiki/Daniel_Naroditsky), Chess.com, August 08, 2014↩︎ [GM and Rybka vs. Stockfish](http://www.talkchess.com/forum/viewtopic.php?t=53228) by Robert Maddox, CCC, August 09, 2014↩︎ [Nakamura vs Stockfish, public match 8/23](http://www.talkchess.com/forum/viewtopic.php?t=53315) by Jesse L, CCC, August 17, 2014↩︎ [Stockfish Outlasts "Rybkamura"](https://www.chess.com/news/stockfish-outlasts-nakamura-3634) by [FM Mike Klein](https://www.chess.com/article/view/chesscom-player-profiles-fm-mikeklein), Chess.com, August 24, 2014↩︎ if not mentioned otherwise, based on the sources of Stockfish 6↩︎ [Ryzen and BMI2: Strange behavior and high latencies](https://www.reddit.com/r/Amd/comments/60i6er/ryzen_and_bmi2_strange_behavior_and_high_latencies/) by DonnieTinyHands, [Reddit](https://en.wikipedia.org/wiki/Reddit), March 20, 2017↩︎ [Stockfish/position.h at sf_12 · official-stockfish/Stockfish · GitHub](https://github.com/official-stockfish/Stockfish/blob/sf_12/src/position.h#L193)↩︎ [Remove piece lists by syzygy1 · Pull Request #3247 · official-stockfish/Stockfish · GitHub](https://github.com/official-stockfish/Stockfish/pull/3247)↩︎ [Re: piece lists advantage with bit-boards?](http://www.talkchess.com/forum3/viewtopic.php?f=7&t=69364&start=12) by Ronald de Man, CCC, December 26, 2018↩︎ [Re: Stockfish 7 progress](http://www.talkchess.com/forum/viewtopic.php?t=58935&start=2) by Lucas Braesch, CCC, January 17, 2016↩︎ [The Art of Evaluation](http://www.talkchess.com/forum/viewtopic.php?topic_view=threads&p=135133&t=15504) by Tord Romstad, CCC, August 2, 2007↩︎ [Stockfish Evaluation Guide](https://hxim.github.io/Stockfish-Evaluation-Guide/)↩︎ [exoticorn/stockfish-js · GitHub](https://github.com/exoticorn/stockfish-js)↩︎ [Cscuile's Sheets](https://docs.google.com/spreadsheets/d/1ZAIuHR6n-5JTxKQc0XUSx1jyUrgVEcj8DNLKA7-urBw/edit#gid=201239930)↩︎ Part 1 covers Houdini, Rybka, Komodo, Stockfish, Critter, Naum, Chiron and Spike↩︎ [Who is the Master?](http://www.alliot.fr/CHESS/ficga.html.en) from Jean-Marc Alliot's [professional website](http://www.alliot.fr/fpro.html.en)↩︎ [exoticorn/stockfish-js · GitHub](https://github.com/exoticorn/stockfish-js)↩︎ [Delphil 3.3b2 (2334) - Stockfish 030916 (3228), TCEC Season 9 - Rapid, Round 11](http://tcec.chessdom.com/archive.php?se=9&rapid&ga=163), September 16, 2016↩︎ [Use equations for PushAway and PushClose · official-stockfish/Stockfish@5a7b45e · GitHub](https://github.com/official-stockfish/Stockfish/commit/5a7b45eac9dedbf7ebc61d9deb4dd934058d1ca1#diff-4cd6bcdb505b124d7bdc612c4789dc26L57-R59)↩︎ [Update default net to nn-8a08400ed089.nnue by Sopel97 · Pull Request #3474 · official-stockfish/Stockfish · GitHub](https://github.com/official-stockfish/Stockfish/pull/3474) by Tomasz Sobczyk↩︎ [Sopel97 (Tomasz Sobczyk) · GitHub](https://github.com/Sopel97)↩︎ [Regan's latest: Depth of Satisficing](http://www.talkchess.com/forum/viewtopic.php?t=57890) by Carl Lumma, CCC, October 09, 2015↩︎ [An info](http://www.talkchess.com/forum3/viewtopic.php?f=2&t=74560) by Sylwy, CCC, July 25, 2020↩︎
