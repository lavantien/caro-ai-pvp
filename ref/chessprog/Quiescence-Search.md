source: https://chessprogramming.org/Quiescence_Search

# Quiescence Search

Home * Search * Quiescence

Karen Schuman - Quiescence 1

---

1. Quiescence by Karen Schuman from [Artist Karen Schuman’s Personal Mythology](http://www.yogachicago.com/jan06/art.shtml) Reviewed by [Anna Poplawska](http://www.chicagoartcriticsassociation.org/B/poplawska.html)↩︎

Most chess programs, at the end of the main search perform a more limited quiescence search, containing fewer moves. The purpose of this search is to only evaluate "quiet" positions, or positions where there are no winning tactical moves to be made. This search is needed to avoid the horizon effect. Simply stopping your search when you reach the desired depth and then evaluate, is very dangerous. Consider the situation where the last move you consider is QxP. If you stop there and evaluate, you might think that you have won a pawn. But what if you were to search one move deeper and find that the next move is PxQ? You didn't win a pawn, you actually lost a queen. Hence the need to make sure that you are evaluating only quiescent (quiet) positions.

# Limiting Quiescence

Since quiescence search has no depth limit, it's vulnerable to search explosions in the absence of any move ordering. For this reason, it's crucial to introduce simple capture ordering such as MVV-LVA before implementing qsearch. Although individual quiescence searches are typically shallow, they can consume a significant percentage of total nodes, so it is also worthwhile to apply some pruning. Apart from not trying moves with the static exchange evaluation < 0, delta pruning can be used for that purpose.

# Standing Pat

In order to allow the quiescence search to stabilize, we need to be able to stop searching without necessarily searching all available captures. In addition, we need a score to return in case there are no captures available to be played. This is done by a using the static evaluation as a "stand-pat" score (the term is taken from the game of poker, where it denotes playing one's hand without drawing more cards). At the beginning of quiescence, the position's evaluation is used to establish a lower bound on the score. This is theoretically sound because we can usually assume that there is at least one move that can either match or beat the lower bound. This is based on the Null Move Observation - it assumes that we are not in Zugzwang. If the lower bound from the stand pat score is already greater than or equal to beta, we can return the stand pat score (fail-soft) or beta (fail-hard) as a lower bound. Otherwise, the search continues, keeping the evaluated "stand-pat" score as an lower bound if it exceeds alpha, to see if any tactical moves can increase alpha.

# Checks

Some programs search treat checks and check evasions specially in quiescence. The idea behind this is that if the side to move is in check, the position is not quiet, and there is a threat that needs to be resolved. In this case, all evasions to the check are searched. Stand pat is not allowed if we are in check, for two reasons. First, because we are not sure that there is a move that can match alpha--in many positions a check can mean a serious threat that cannot be resolved. Second, because we are searching every move in the position, rather than only captures. Standing pat assumes that even if we finish searching all moves, and none of them increase alpha, one of the non-tactical moves can most likely raise alpha. This is not valid if we search every move. The other case of treating checks specially is the checking moves themselves. Some programs, after searching all the captures in a position without finding a move to raise alpha, will generate non-capture moves that give check. This has to be limited somehow, however, because in most given positions there will be very many long and pointless checking sequences that do not amount to anything. Most programs achieve this limit by delta pruning checks, as well as limiting the generation of checks to the first X plies of quiescence.

# Pseudo Code

```
int Quiesce( int alpha, int beta ) {
    int static_eval = Evaluate();

    // Stand Pat
    int best_value = static_eval;
    if( best_value >= beta )
        return best_value;
    if( best_value > alpha )
        alpha = best_value;

    until( every_capture_has_been_examined )  {
        MakeCapture();
        score = -Quiesce( -beta, -alpha );
        TakeBackMove();

        if( score >= beta )
            return score;
        if( score > best_value )
            best_value = score;
        if( score > alpha )
            alpha = score;
    }

    return best_value;
}
```

# See also

- Bobby's Strategic Quiescence Search
- CPW-Engine_quiescence
- Crossovers
- Delta Pruning
- Generalized Quiescence Search by Don Beal
- Horizon Effect
- Horizon Node
- MVV-LVA
- Quiescent Node
- Search Explosion
- Static Exchange Evaluation
- Swap-off algorithm - SOMA
- Swap-off by Helmut Richter
- Tactical Quiescence Search in Excalibur Mirage
- Vice Video on Quiescence
- Zzzzzz' Quiescence Search

# Publications

## 1975

- Larry Harris (1975) The Heuristic Search And The Game Of Chess - A Study Of Quiescence, Sacrifices, And Plan Oriented Play. [IJCAI](http://www.informatik.uni-trier.de/%7Eley/db/conf/ijcai/index.html) 1975 Tbilisi, Georgia: 334-339. reprinted (1988) in Computer Chess Compendium

## 1980 ...

- Hermann Kaindl (1982). Dynamic Control of the Quiescence Search in Computer Chess. Cybernetics and Systems Research (ed. R. Trappl), pp. 973-977. North-Holland, Amsterdam.
- Hermann Kaindl (1982). Quiescence Search in Computer Chess. SIGART Newsletter, 80, pp. 124-131. Reprinted (1983) in Computer-Game-Playing: Theory and Practice, pp. 39-52. Ellis Horwood Ltd., Chichester.
- Don Beal (1984). Mating Sequences in the Quiescence Search. ICCA Journal, Vol. 7, No. 3
- Prakash Bettadapur (1986). Experiments in Chess Capture Search, M.Sc. Thesis, Department of Computing Science, University of Alberta.
- Prakash Bettadapur (1986). Influence of Ordering on Capture Search. ICCA Journal, Vol. 9, No. 4
- Prakash Bettadapur, Tony Marsland (1988). Accuracy and Savings in Depth-Limited Capture Search. In [International Journal of Man-Machine Studies, 29](http://www.informatik.uni-trier.de/%7Eley/db/journals/ijmms/ijmms29.html#BettadapurM88) (5) pp. 497-502
- Don Beal (1989). Experiments with the Null Move. Advances in Computer Chess 5, a revised version is published (1990) under the title A Generalized Quiescence Search Algorithm. [Artificial Intelligence](https://en.wikipedia.org/wiki/Artificial_Intelligence_%28journal%29), Vol. 43, No. 1, pp. 85-98. ISSN 0004-3702, edited version in (1999). The Nature of MINIMAX Search. Ph.D. thesis, IKAT, ISBN 90-62-16-6348. Chapter 10, pp. 101-116 » Null Move
- Günther Schrüfer (1989). A Strategic Quiescence Search. ICCA Journal, Vol. 12, No. 1 » Bobby's Strategic Quiescence Search

## 1990 ...

- Don Beal (1990). A Generalized Quiescence Search Algorithm. [Artificial Intelligence](https://en.wikipedia.org/wiki/Artificial_Intelligence_%28journal%29), Vol. 43, No. 1, pp. 85-98. ISSN 0004-3702
- Sören Walter Perrey (1991). Mathematische Methoden der Künstlichen Intelligenz: Zur Quiescence-Suche in Spielbäumen. Diplom thesis, Sonderforschungsbereich 343, [E91-006](http://www.mathematik.uni-bielefeld.de/sfb343/preprints/index91.html), [University of Bielefeld](https://en.wikipedia.org/wiki/Bielefeld_University) (German) 1
- Michael Gherrity, Paul Kube (1993). Quiescent Search is Beneficial. Technical Report CS93-289, [University of California, San Diego](https://de.wikipedia.org/wiki/University_of_California,_San_Diego)
- Don Beal (1999). The Nature of MINIMAX Search. Ph.D. thesis, IKAT, ISBN 90-62-16-6348

## 2000 ...

- Jeff Rollason (2000). SUPER-SOMA - Solving Tactical Exchanges in Shogi without Tree Searching. [Lecture Notes In Computer Science](https://en.wikipedia.org/wiki/Lecture_Notes_in_Computer_Science), Vol. 2063, CG 2000, [Word preprint](http://www.aifactory.co.uk/downloads/SUPER-SOMA.doc) 2
- Jeff Rollason (2006). [Looking for Alternatives to Quiescence Search](http://www.aifactory.co.uk/newsletter/2006_03_quiescence_alts.htm). AI Factory, Autumn 2006
- Don Beal (2006). Review of a nullmove-quiescence search mechanism from 1986.  (Draft) 3
- Maarten Schadd, Mark Winands (2009). Quiescence Search for Stratego. In BNAIC 2009, [pdf](http://www.personeel.unimaas.nl/Maarten-Schadd/Papers/2009StrategoBNAIC1.pdf)

# Forum Posts

## 1994 ...

- [Efficient quiescence](https://groups.google.com/group/rec.games.chess/browse_frm/thread/ee86982df38f003c) by Hans Bogaards, rec.games.chess, January 19, 1994
- [Computer Chess: swap down evaluators vs capture search](https://groups.google.com/group/rec.games.chess/browse_frm/thread/dd1c55ecc9f48717) by Jon Dart, rgc, October 24, 1994 » Swap-off algorithm - SOMA

[Re: Computer Chess: swap down evaluators vs capture search](https://groups.google.com/group/rec.games.chess/msg/527be476c5dd22d1) by Deniz Yuret, rgc, October 26, 1994

- [quiescence search problems](https://groups.google.com/group/rec.games.chess.computer/browse_frm/thread/a027ac0e2fb5892e) by Matt Craighead, rgcc, August 01, 1995

[Re: Quiescence search problems](https://groups.google.com/group/rec.games.chess.computer/msg/fedfcfaf26d04dfa) by David Blackman, rgcc, August 3, 1995 » Integrated Bounds and Values

- [quiescent vs non-quiescent node counting](https://groups.google.com/group/rec.games.chess.computer/browse_frm/thread/926eaf0869b6f176) by Robert Hyatt, rgcc, July 01, 1996
- [Deep Quiesence Searching](https://groups.google.com/group/rec.games.chess.computer/browse_frm/thread/6fb02db95638ade1) by Steve Dicks, rgcc, February 22, 1997
- [quiescence search](https://groups.google.com/group/rec.games.chess.computer/browse_frm/thread/ca0300b50438a388) by Andrew Tridgell, rgcc, April 16, 1997 » Check, Crafty
- [Limiting the QSearch](https://www.stmintz.com/ccc/index.php?id=13485) by John Scalo, CCC, December, 30, 1997
- [Quiescence vs swapoff](https://www.stmintz.com/ccc/index.php?id=17016) by Peter Fendrich, CCC, April 15, 1998
- [SEE for forward pruning in Q. Search](https://www.stmintz.com/ccc/index.php?id=63511) by Tom King, CCC, August 04, 1999 » SEE
- [SEE for forward pruning in the Q. search - I'm confused!](https://www.stmintz.com/ccc/index.php?id=64357) by Tom King, CCC, August 11, 1999

## 2000 ...

- [What is the q-search?](https://www.stmintz.com/ccc/index.php?id=86662) by Leonid, CCC, January 07, 2000
- [Qsearch problems...(about sorting and SEE)](https://www.stmintz.com/ccc/index.php?id=141230) by Severi Salminen, CCC, November 26, 2000 » Static Exchange Evaluation

2001

- [Bonus points for side to move in qsearch?](https://www.stmintz.com/ccc/index.php?id=148445) by Leen Ammeraal, CCC, January 06, 2001 » Tempo
- [About limiting Qsearch, again...](https://www.stmintz.com/ccc/index.php?id=204243) by Severi Salminen, CCC, December 29, 2001
- [About qsearch...](https://www.stmintz.com/ccc/index.php?id=203771) by Severi Salminen, CCC, December 27, 2001
- [Qsearch survey](https://www.stmintz.com/ccc/index.php?id=204492) by Severi Salminen, CCC, December 30, 2001

2002

- [Checks in the Qsearch](https://www.stmintz.com/ccc/index.php?id=237893) by Scott Gasch, CCC, June 28, 2002 » Check
- [A question about quiescence search](https://www.stmintz.com/ccc/index.php?id=260485) by Nagendra Singh Tomar, CCC, October 19, 2002
- [Quiescence Explosion](https://www.stmintz.com/ccc/index.php?id=267486) by David Rasmussen, CCC, November 26, 2002
- [Quiescent Explosion](https://www.stmintz.com/ccc/index.php?id=303257) by macaroni, CCC, June 26, 2003
- [Regarding Qsearch with Fractional ply extensions](https://www.stmintz.com/ccc/index.php?id=310897) by Federico Corigliano, CCC, August 11, 2003 » Depth - Fractional Plies
- [real job of the qSearch? find quiet vs stop horizon effect](https://www.stmintz.com/ccc/index.php?id=313206) by Scott Farrell, CCC, August 28, 2003
- [quiescence](https://groups.google.com/group/rec.games.chess.computer/browse_frm/thread/d451877add19a50e) by Noah Roberts, rgcc, September 20, 2003

2004

- [QSearch() as PVS() ?](https://www.stmintz.com/ccc/index.php?id=342287) by Matthias Gemuh, CCC, January 14, 2004
- [Rebel's long checks concept in QS](https://www.stmintz.com/ccc/index.php?id=344282) by milix, CCC, January 23, 2004 » Rebel, Check
- [quiesce node explosion](https://www.stmintz.com/ccc/index.php?id=344566) by Mike Siler, CCC, January 24, 2004
- [Qsearch Checks](https://www.stmintz.com/ccc/index.php?id=385027) by Tor Lattimore, CCC, August 29, 2004 » Check
- [Checks in QSearch](http://www.open-aurec.com/wbforum/viewtopic.php?f=4&t=702&p=2642) by Dan Honeycutt, Winboard Programming Forum, November 23, 2004

## 2005 ...

- [quiescence search / horizon question](https://www.stmintz.com/ccc/index.php?id=445400) by Andrew Shapira, CCC, August 26, 2005
- [How to Best Limit Checks in the Quiescence ?](http://www.talkchess.com/forum/viewtopic.php?p=139285) by Stuart Cracraft, CCC, August 20, 2007 » Check, Checks in Quiescence Search
- [Quiescence Search Explosions](http://www.talkchess.com/forum/viewtopic.php?t=20727) by Mike Leany, CCC, April 18, 2008
- [checks in q-search](http://www.talkchess.com/forum/viewtopic.php?t=23447) by Robert Hyatt, CCC, September 02, 2008
- [Limiting Quiescent Search Depth](http://www.talkchess.com/forum/viewtopic.php?t=28023) by John Merlino, CCC, May 20, 2009
- [Null move in quiescence search idea from Don Beal, 1986](http://www.talkchess.com/forum/viewtopic.php?t=29439) by Eelco de Groot, CCC, Aug 17, 2009 » Null Move Pruning, Don Beal
- [Threat information from evaluation to inform q-search](http://www.talkchess.com/forum/viewtopic.php?p=291259) by Gary, CCC, September 15, 2009
- [Only recaptures in qsearch?](http://www.talkchess.com/forum/viewtopic.php?t=30738) by John Merlino, CCC, November 21, 2009

## 2010 ...

- [Avoiding qsearch explosion](http://www.talkchess.com/forum/viewtopic.php?t=32148) by Marco Costalba, CCC, January 29, 2010
- [Problems when implementing checks in qsearch](http://talkchess.com/forum/viewtopic.php?t=32345) by Luca Hemmerich, CCC, February 03, 2010
- [Standpat and check](http://www.talkchess.com/forum/viewtopic.php?t=32424) by Vlad Stamate, CCC, February 06, 2010 » Standing Pat, Check
- [This is totally weird ... Don't understand at all](http://www.talkchess.com/forum/viewtopic.php?t=36692) by Gregory Strong, CCC, November 13, 2010

2012

- [checks in quies](http://www.talkchess.com/forum/viewtopic.php?t=42971) by Larry Kaufman, CCC, March 22, 2012
- [stand pat or side to move bonus](http://www.talkchess.com/forum/viewtopic.php?t=42982) by Larry Kaufman, CCC, March 22, 2012
- [TT question?](http://www.talkchess.com/forum3/viewtopic.php?f=7&t=43769) by Fermin Serrano, CCC, May 19, 2012 » Transposition Table
- [Some thoughts on QS](http://www.talkchess.com/forum/viewtopic.php?t=44507) by Harm Geert Muller, CCC, July 19, 2012
- [QSearch, checks and the lack of progress...](http://www.talkchess.com/forum/viewtopic.php?t=44599) by Mincho Georgiev, CCC, July 27, 2012
- [Quiescence - Check Evaluation and Depth Control](http://www.talkchess.com/forum/viewtopic.php?t=45942) by Cheney Nattress, CCC, November 10, 2012

2013

- [Quiescence Search and Checkmates](http://www.open-chess.org/viewtopic.php?f=5&t=2212) by CDaley11, OpenChess Forum, January 11, 2013 » Checkmate
- [Quiescent search, and side to move is in check](http://www.talkchess.com/forum/viewtopic.php?t=47162) by Louis Zulli, CCC, February 08, 2013
- [Transposition table usage in quiescent search?](http://www.talkchess.com/forum/viewtopic.php?t=47373) by Jerry Donald, CCC, March 01, 2013 » Transposition Table
- [Pruning in QS](http://www.talkchess.com/forum/viewtopic.php?t=47423) by Harm Geert Muller, CCC, March 06, 2013 » Pruning
- [QS investigation](http://www.talkchess.com/forum/viewtopic.php?t=47436) by Ed Schröder, CCC, March 07, 2013
- [qsearch question](http://www.open-chess.org/viewtopic.php?f=5&t=2402) by nak3c, OpenChess Forum, August 19, 2013
- [about qs](http://www.talkchess.com/forum/viewtopic.php?t=49311) by Daniel Anulliero, CCC, September 11, 2013

2014

- [Positional quiesence](http://www.talkchess.com/forum/viewtopic.php?t=51967) by Harm Geert Muller, CCC, April 12, 2014
- [Transposition table in Q-search](http://www.talkchess.com/forum/viewtopic.php?t=54755) by Alex Ferguson, CCC, December 26, 2014 » Transposition Table

## 2015 ...

- [Detail evaluation within quiescence search](http://www.talkchess.com/forum/viewtopic.php?t=55424) by Reinhard Scharnagl, CCC, February 22, 2015
- [hanging piece at starting quiescence search - how to handle?](http://www.talkchess.com/forum/viewtopic.php?t=55427) by Reinhard Scharnagl, CCC, February 22, 2015 » Hanging Piece
- [Search algorithm in it's simplest forum](http://www.talkchess.com/forum/viewtopic.php?t=55474) by Mahmoud Uthman, CCC, February 25, 2015 » Alpha-Beta
- [Check-extension in QS](http://www.talkchess.com/forum/viewtopic.php?t=55874) by Harm Geert Muller, CCC, April 03, 2015 » Check
- [quiescence search (best practices)](http://www.open-chess.org/viewtopic.php?f=5&t=2852) by thevinenator, OpenChess Forum, July 02, 2015
- [Null Move in Quiescent search](http://www.talkchess.com/forum/viewtopic.php?t=58527) by Laurie Tunnicliffe, CCC, December 09, 2015 » Null Move Pruning, Search Pathology

2016

- [Checks in qsearch - must-have or optional?](http://www.talkchess.com/forum/viewtopic.php?t=59529) by Martin Fierz, CCC, March 15, 2016 » Check, Checks in Quiescence Search
- [Hashing in Qsearch?](http://talkchess.com/forum/viewtopic.php?t=59740) by Martin Fierz, CCC, April 03, 2016 » Transposition Table
- [Quiescence node explosion](http://www.open-chess.org/viewtopic.php?f=5&t=2984) by sandermvdb, OpenChess Forum, June 01, 2016 » Search Explosion
- [Starting with quiescence search](http://www.talkchess.com/forum/viewtopic.php?t=60962) by Luis Babboni, CCC, July 28, 2016
- [Quiescence Search Performance](http://www.talkchess.com/forum/viewtopic.php?t=60964) by David Cimbalista, CCC, July 28, 2016
- [Removing Q-search](http://www.talkchess.com/forum/viewtopic.php?t=61307) by Matthew Lai, CCC, September 02, 2016
- [Searching using slow eval with tactical verification](http://www.talkchess.com/forum/viewtopic.php?t=61348) by Matthew Lai, CCC, September 06, 2016
- [Collecting PVs of Qsearch ?](http://www.talkchess.com/forum/viewtopic.php?t=61796) by Mahmoud Uthman, CCC, October 22, 2016 » Principal Variation, Triangular PV-Table

2017

- [capturing PV in QSearch](http://www.open-chess.org/viewtopic.php?f=5&t=3072) by thevinenator, OpenChess Forum, January 20, 2017 » Principal Variation, Triangular PV-Table
- [Ridiculous QSearch Depth](http://www.talkchess.com/forum/viewtopic.php?t=63326) by Jonathan Rosenthal, CCC, March 03, 2017 » Depth
- [Q search explosion](http://www.talkchess.com/forum/viewtopic.php?t=63590) by Colin Jenkins, CCC, March 30, 2017 » Search Explosion
- [Probe EGT in quiescence?](http://www.talkchess.com/forum/viewtopic.php?t=64030) by Nguyen Pham, CCC, May 20, 2017 » Endgame Tablebases, Xiangqi
- [Is expensive eval required for QS?](http://www.talkchess.com/forum/viewtopic.php?t=64674) by Alexandru Mosoi, CCC, July 21, 2017 » Lazy Evaluation
- [Cutoffs in Quiescence Search](http://www.talkchess.com/forum3/viewtopic.php?f=7&t=64940) by jfern2011, CCC, August 20, 2017 » Beta-Cutoff
- [TT in Qsearch](http://www.talkchess.com/forum3/viewtopic.php?f=7&t=65903) by Laurie Tunnicliffe, CCC, December 05, 2017 » Transposition Table

2019

- [Playing transposition table moves in the Quiescence search](http://www.talkchess.com/forum3/viewtopic.php?f=7&t=69629) by Andrew Grant, CCC, January 17, 2019 » Transposition Table

## 2020 ...

- [Tactical search](http://www.talkchess.com/forum3/viewtopic.php?f=7&t=74170) by Alvaro Cardoso, CCC, June 13, 2020 » Tactics
- [Qsearch() variant](http://www.talkchess.com/forum3/viewtopic.php?f=7&t=75059) by Mike Sherwin, CCC, September 09, 2020
- [Quiescence Search doesn't improve strength](http://www.talkchess.com/forum3/viewtopic.php?f=7&t=76706) by Thomas Jahn, CCC, February 25, 2021
- [For or against the transposition table probe in quiet search?](http://www.talkchess.com/forum3/viewtopic.php?f=7&t=77286) by Eugene Kotlov, CCC, May 11, 2021 » Transposition Table
- [Qsearch dynamic order besides MVV/LVA](http://www.talkchess.com/forum3/viewtopic.php?f=7&t=77380) by Aleks Peshkov, CCC, May 25, 2021 » Move Ordering
- [Futility Pruning and its Relation to Quiescence Search](http://www.talkchess.com/forum3/viewtopic.php?f=7&t=77451) by Jakob Progsch, CCC, June 06, 2021 » Futility Pruning

# External Links

- [Quiescence search from Wikipedia](https://en.wikipedia.org/wiki/Quiescence_search)
- [Quiescence from Wikipedia](https://en.wikipedia.org/wiki/Quiescence)
- [Quiescence Search](http://web.archive.org/web/20070813042640/www.seanet.com/~brucemo/topics/quiescent.htm) from Bruce Moreland's [Programming Topics](http://web.archive.org/web/20070811182741/www.seanet.com/%7Ebrucemo/topics/topics.htm) ([Wayback Machine](https://en.wikipedia.org/wiki/Wayback_Machine))
- [Quiescent Search in REBEL](https://web.archive.org/web/20120331060714/http://www.top-5000.nl/authors/rebel/chess840.htm#QS) from Ed Schröder's [Programmer Stuff](http://www.top-5000.nl/authors/rebel/chess840.htm) ([Wayback Machine](https://en.wikipedia.org/wiki/Wayback_Machine))
- [Pim Jacobs](https://en.wikipedia.org/wiki/Pim_Jacobs), Ruud Jacobs, [Ruud Brink](https://nl.wikipedia.org/wiki/Ruud_Brink), [Wim Overgaauw](https://en.wikipedia.org/wiki/Wim_Overgaauw), Dom Um Romão, Astrud Gilberto - [Meditation](https://en.wikipedia.org/wiki/Meditation_%28song%29), [It Might as Well Be Spring](https://en.wikipedia.org/wiki/It_Might_as_Well_Be_Spring), Telephone Song, [Only Trust Your Heart](https://en.wikipedia.org/wiki/Only_Trust_Your_Heart), [Corcovado (Quiet Nights of Quiet Stars)](https://en.wikipedia.org/wiki/Corcovado_%28song%29), [The Girl From Ipanema](https://en.wikipedia.org/wiki/The_Girl_from_Ipanema); from [Dzjes Zien à la Bossa Nova](http://www.cultura.nl/genres/kunst/2014/brazili-.html), [NCRV](https://en.wikipedia.org/wiki/Nederlandse_Christelijke_Radio_Vereniging) 1965, [YouTube](https://en.wikipedia.org/wiki/YouTube) Video

[Watch on YouTube](https://www.youtube.com/watch?v=9DCaxN6TfH4)

# References

Up one level    Ingo Althöfer (1991). Mathematische Methoden der Künstlichen Intelligenz: Zur Quiescence-Suche in Spielbäumen. Review, ICCA Journal, Vol. 14, No. 2↩︎ Jeff Rollason (2006). [Looking for Alternatives to Quiescence Search](http://www.aifactory.co.uk/newsletter/2006_03_quiescence_alts.htm). AI Factory, Autumn 2006↩︎ courtesy of Don Beal and Carey Bloodworth, [Re: Antique chess programs](http://www.talkchess.com/forum/viewtopic.php?t=58603&start=13) by Carey, CCC, December 16, 2015↩︎
