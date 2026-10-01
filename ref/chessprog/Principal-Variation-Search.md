source: https://chessprogramming.org/Principal_Variation_Search

# Principal Variation Search

Home * Search * Principal Variation Search

John Cage - Variations III, No. 14 1

---

1. [Variations III, No. 14](https://en.wikipedia.org/wiki/File:Cage-variations-iii-14-small.jpg), a 1992 print by John Cage from a series of 57, [John Cage from Wikipedia](https://en.wikipedia.org/wiki/John_Cage) [Fair use](https://en.wikipedia.org/wiki/Fair_use)↩︎

Principal Variation Search (PVS),
 an enhancement to Alpha-Beta, based on null- or zero window searches of none PV-nodes, to prove a move is worse or not than an already safe score from the principal variation.

# The Idea

In most of the nodes we need just a bound, proving that a move is unacceptable for us or for the opponent, and not the exact score. This is needed only in so-called principal variation - a sequence of moves acceptable for both players (i.e. not causing a beta-cutoff anywhere in the path) which is expected to propagate down to the root. If a lower-depth search has already established such a sequence, finding a series of moves whose value is greater than alpha but lower than beta throughout the entire branch, the chances are that deviating from it will do us no good. So in a PV-node only the first move (the one which is deemed best by the previous iteration of an iterative deepening framework) is searched in the full window in order to establish the expected node value.

When we already have a PV-move (defined as the move that raised alpha in a PV-node) we assume we are going to stick with it. To confirm our belief, a null- or zero window search centered around alpha is conducted to test if a new move can be better. If so, with respect to the null window but not with respect to the full window, we have to do a re-search with the full normal window. Since null window searches are cheaper, with a good move ordering we expect to save about 10% of a search effort.

Bruce Moreland's PVS implementation waits until a move is found that improves alpha, and then searches every move after that with a zero window around alpha 1 . The alpha improvement usually occurs at the first move, and always at the leftmost nodes (assuming from left to right traversal) with a most open alpha-beta window of +-oo. In re-searches or with aspiration-windows the first moves may rarely not improve alpha. As pointed out by Edmund Moshammer, Gian-Carlo Pascutto, Robert Hyatt and Vincent Diepeveen 2 , it is recommend to only search the first move with an open window, and then every other move after that with a zero window. A further improvement (similar to that known from the NegaScout algorithm) is possible. Since there is not much to be gained in the last two plies of the normal search, one might disable PVS there, but programs respond differently to that change.

# History

PVS was introduced by Tony Marsland and Murray Campbell in 1982 3 as nomination of Finkel's and Fishburn's routine Palphabeta 4 5 , in Fishburn's 1981 thesis 6 called Calphabeta, which in turn is similar to Judea Pearl's Scout 7 8 :

`An interesting implementation of the alpha-beta algorithm treats the first variation in a special way. The method was originally called``Palphabeta``[FISH80] and then renamed``Calphabeta``[FISH81], but will be referred to here as principal variation search or PVS for short.`

Despite the publications, PVS was already used in 1978, as mentioned by Robert Hyatt 9 :

`I first used PVS in 1978, quite by accident. Murray Campbell and I were discussing this idea at the``ACM event``in Washington, DC. We were running on a Univac, and I suggested that we dial up my local Vax box and make the changes and see how it works. It looked pretty good, with the only odd thing being fail highs on very minor score changes, which was OK. The next round, our Univac developed a memory problem and I switched back to the vax and had a few exciting moments when we came out with a Nxf7!! sort of output, only to see the score rise by 2 or 3 millipawns. Even in 1978 we just searched the first move with normal alpha/beta and then went into the null-window search, just as the code exactly does in``Crafty``...`

John Philip Fishburn in a note, September 2010:

`I was thinking about what goes wrong if you start the entire search with a too-narrow window. If the beta value is too low, then one of the children of the``root``might``fail high``, and you wouldn't know the proper windows to give to the subsequent children of the root. Wait a minute... what if there aren't any subsequent children, i.e. what if the child that failed high was the last child of the root? Then you don't care about the subsequent windows, and in fact you've just proved that the last child is the best move. So when you're on the last child of the root, go all the way by bringing beta down to alpha+1. I was trying to get this published starting in Aug. 1979, and it finally appeared as "An optimization of alpha-beta search" in``SIGART``bulletin Issue 72 (July 1980)`10`. After that came various generalizations where the null window is used generally in the search, also the``fail-soft``algorithm. I was somewhat disappointed in the speedup (or lack thereof) that I measured on``checkers``lookahead trees. However when I went to work at``Bell Labs``in 1981,``Ken Thompson``told me that he had read the SIGART paper, and had sped up``Belle``by 1.5x with``null windows``.`

and subsequently some details about Belle's PVS-implementation ...

`The PVS algorithm in Belle did not do a second search at the root until a``second``fail high occurred. I don’t know whether or not this idea appears in the literature or not. I would hope it does, but I haven’t been following the literature for about 25 years. In other words, Belle is cleverly going for broke: it knows it’s got a high failure, which is the best move so far, but as long as it doesn’t get a second high failure, the first high failure remains the best move, and it can still avoid doing any more full searches.`

# PVS and NegaScout

Most PVS implementations are similar to Reinefeld's NegaScout 11 12 , and are used by most todays chess programs. It is based on the accuracy of the move ordering. Typically, modern chess programs find fail-highs on the first move around 90% of the time. This observation can be used to narrow the window on searches of moves after the first, because there is a high probability that they will be lower than the score of the first move.

Reinefeld's original implementation introduces one additional variable on the stack (only b, since after a = alpha, alpha is not needed any longer), for a slightly simpler control structure than PVS. It has therefor set a new null window at the end of the loop (b = a + 1), but has to consider the move count for the re-search condition though. His implementation trusts the null-window score, even if the re-search doesn't confirm the alpha increase, eventually due to search instability. While re-searching, it uses the narrow window of {score, beta}, while other implementations dealing with search instability, re-search with {alpha, beta}. Practically, due to Quiescence Search, and fail-soft implementations of PVS, the two algorithms are essentially equivalent to each other - they expand the same search tree 13 14 .

## Guido Schimmels

Guido Schimmels in a CCC post on the difference of PVS vs. NegaScout 15 :

`The difference is how they handle re-searches: PVS passes alpha/beta while NegaScout passes the value returned by the null window search instead of alpha. But then you can get a fail-low on the research due to search anonomalies. If that happens NegaScout returns the value from the first search. That means you will have a crippled PV. Then there is a refinement Reinefeld suggests which is to ommit the re-search at the last two plies (depth > 1) - but that won't work in a real program because of search extensions. NegaScout is slightly an ivory tower variant of PVS (IMHO).`

PVS:

```
value = PVS(-(alpha+1),-alpha)
if(value > alpha && value < beta) {
  value = PVS(-beta,-alpha);
}
```

NegaScout:

```
value = NegaScout(-(alpha+1),-alpha)
if(value > alpha && value < beta && depth > 1) {
  value2 = NegaScout(-beta,-value)
  value = max(value,value2);
}
```

## Yngvi Björnsson

Quote by Yngvi Björnsson from CCC, January 05, 2000 16 :

`Search-wise PVS and Negascout are identical (except the deep-cutoffs on the PV you mention), they are just formulated differently. In Negascout the same routine is used for searching both the PV and the rest of the tree, whereas PVS is typically formulated as two routines: PVS (for searching the PV) and NWS (for the null-window searches). Negascout and PVS were developed about the same time in the early '80 (82-83), but independently. I guess, that's part of the reason we know them by different names. Personally, I've always found the PVS/NWS formulation the most intuative, it's easier to understand what's really going on.`

## Dennis Breuker

Quote by Dennis Breuker from CCC, July 28, 2004 17 :

`Q: What's the different between negascout and PVS ? They look like the same algorithm to me.`

`They are identical, see note 15 on page 22 of my thesis`18`:We note that the version of principal-variation search as mentioned by Marsland (1986)`19`is identical to the version of negascout as mentioned by Reinefeld (1989)`20`. We use the 1989 reference instead of 1983`21`, which was the first source of this algorithm, since the algorithm described in Reinefeld (1983) contains minor errors.Dennis`

# Pseudo Code

This demonstrates PVS in a fail-hard framework, where alpha and beta are hard bounds of the returned score.

```
int pvSearch( int alpha, int beta, int depth ) {
   if( depth == 0 ) return quiesce( alpha, beta );
   for ( all moves)  {
      make
      if ( first move ) {
         score = -pvSearch(-beta, -alpha, depth - 1);
      } else {
         score = -pvSearch(-alpha-1, -alpha, depth - 1);
         if ( score > alpha && beta - alpha > 1 )
         // beta - alpha > 1 prevents redundant re-search of Non-PV nodes.
         // Note that in rare cases this conditions is true for PV nodes.
         // Some engines prevent that by setting a boolean PvNode variable
         // to determine if position is in a PV node. But it's generally
         // considered as too rare to cause considereable stength loss.
            score = -pvSearch(-beta, -alpha, depth - 1); // re-search
      }
      unmake
      if( score >= beta )
         return beta;   // fail-hard beta-cutoff
      if( score > alpha ) {
         alpha = score; // alpha acts like max in MiniMax
      }
   }
   return alpha; // fail-hard
}
```

# PVS + ZWS

Often, programmers template PVS to a pure PV-node search and a separate scout search with null windows.

```
enum NodeType {
    NonPV,
    PV
};

template<NodeType nodeType>
int search( int alpha, int beta, int depth ) {
   if( depth == 0 ) return quiesce(alpha, beta);
   constexpr bool PVNode = nodeType != NonPV;
   int bestValue = -oo;
   for ( all moves)  {
      make
      if ( first move ) {
         score = -search<nodeType>(-beta, -alpha, depth - 1);
      } else {
         score = -search<NonPV>(-alpha - 1, -alpha, depth - 1);
         if ( score > alpha && PVNode )
            score = -search<PV>(-beta, -alpha, depth - 1); // re-search
      }
      unmake
      if( score >= beta )
         return score;   // fail-soft beta-cutoff
      if( score > alpha ) {
         alpha = score; // alpha acts like max in MiniMax
      }
   }

   return bestValue;
}
```

# PVS and Aspiration

When implementing PVS together with the aspiration window, one must be aware that in this case also a normal window search might fail, leaving the program with no move and no PV. (Actually this is the reason why I wrote "When we already have a PV move" and not "searching later moves").

- PVS and Aspiration

A state of the art fail-soft PVS implementation, called without aspiration, was posted by Vincent Diepeveen inside the mentioned CCC thread 22 :

```
Call from root:
   rootscore = PVS(-infinite, infinite, depthleft);

int PVS(alfa,beta,depthleft) {
   if( depthleft <= 0 ) return qsearch(alfa, beta);

   // using fail soft with negamax:
   make first move
   bestscore = -PVS(-beta, -alfa, depthleft-1);
   unmake first move
   if( bestscore > alfa ) {
      if( bestscore >= beta )
         return bestscore;
      alfa = bestscore;
   }

   for( all remaining moves ) {
      make move
      score = -PVS(-alfa-1, -alfa, depthleft-1); // alphaBeta or zwSearch
      if( score > alfa && score < beta ) {
         // research with window [alfa;beta]
         score = -PVS(-beta, -alfa, depthleft-1);
         if( score > alfa )
           alfa = score;
      }
      unmake move
      if( score > bestscore ) {
         if( score >= beta )
            return score;
         bestscore = score;
      }
   }
   return bestscore;
}
```

# See also

- Alpha-Beta
- CPW-Engine_search
- Enhanced Forward Pruning
- Iterative Deepening
- Move Ordering
- MTD(f)
- NegaScout
- Null Window
- Principal Variation
- PVS and Aspiration
- Scout

# Publications

## 1980 ...

- Judea Pearl (1980). Asymptotic Properties of Minimax Trees and Game-Searching Procedures. [Artificial Intelligence](https://en.wikipedia.org/wiki/Artificial_Intelligence_%28journal%29), Vol. 14, No. 2
- Judea Pearl (1980). Scout: A Simple Game-Searching Algorithm with Proven Optimal Properties. Proceedings of the First Annual National Conference on Artificial Intelligence. Stanford. [pdf](http://ftp.cs.ucla.edu/pub/stat_ser/scout.pdf)
- Raphael Finkel, John Philip Fishburn (1980). Parallel Alpha-Beta Search on Arachne. IEEE International Conference on Parallel Processing
- John Philip Fishburn (1980). An optimization of alpha-beta search. SIGART Bulletin, Issue 72
- John Philip Fishburn (1981). Analysis of Speedup in Distributed Algorithms. Ph.D. Thesis, [University of Wisconsin-Madison](https://en.wikipedia.org/wiki/University_of_Wisconsin-Madison), [pdf](http://www.cs.wisc.edu/techreports/1981/TR431.pdf), Calphabeta at page 167
- Tony Marsland, Murray Campbell (1982). Parallel Search of Strongly Ordered Game Trees. ACM Computing Surveys, Vol. 14, No. 4, [pdf](http://www.cs.ualberta.ca/%7Etony/OldPapers/strong.pdf)
- Murray Campbell, Tony Marsland (1983). A Comparison of Minimax Tree Search Algorithms. [Artificial Intelligence](https://en.wikipedia.org/wiki/Artificial_Intelligence_%28journal%29), Vol. 20, No. 4, pp. 347-367. ISSN 0004-3702.
- Tony Marsland (1983). Relative Efficiency of Alpha-beta Implementations. Procs. 8th Int. Joint Conf. on Art. Intell., pp. 763-766. Kaufman, Los Altos, [pdf](http://dli.iiit.ac.in/ijcai/IJCAI-83-VOL-2/PDF/040.pdf)
- Alexander Reinefeld (1983). An Improvement to the Scout Tree-Search Algorithm. ICCA Journal, Vol. 6, No. 4, [pdf](http://sc.hlrn.de/reinefeld/bib/83icca.pdf)

## 1985 ...

- Agata Muszycka-Jones, Rajjan Shinghal (1985). An empirical comparison of pruning strategies in game trees. IEEE Transactions on Systems, Man, and Cybernetics, Vol. 15, No. 3
- Tony Marsland (1986). A Review of Game-Tree Pruning. ICCA Journal, Vol. 9, No. 1, [pdf](http://www.cs.ualberta.ca/%7Etony/OldPapers/1986review.pdf)
- Alexander Reinefeld, Tony Marsland (1987). A Quantitative Analysis of Minimal Window Search. [IJCAI-87](http://www.informatik.uni-trier.de/%7Eley/db/conf/ijcai/ijcai87.html), [pdf](http://webdocs.cs.ualberta.ca/~tony/OldPapers/ijcai87.pdf)

## 2000 ...

- Mark Winands, Jaap van den Herik, Jos Uiterwijk, Erik van der Werf (2003). [Enhanced forward pruning](https://research.tilburguniversity.edu/en/publications/enhanced-forward-pruning). JCIS 2003
- Mark Winands, Jaap van den Herik, Jos Uiterwijk, Erik van der Werf (2005). Enhanced Forward Pruning. [Information Sciences](https://en.wikipedia.org/wiki/Information_Sciences_(journal)), Vol. 175, No. 4, [pdf preprint](http://erikvanderwerf.tengen.nl/pubdown/Enhanced%20forward%20pruning.pdf) (with PVS modifications)

# Forum Posts

## 1995 ...

- [Trick Marsland](https://groups.google.com/group/rec.games.chess.computer/browse_frm/thread/3ffa1d89d13f9e86) by Robert Hyatt, rgcc, February 15, 1996
- [Re: Zero-width Window Null Move Search](https://www.stmintz.com/ccc/index.php?id=20868) by Guido Schimmels, CCC, June 18, 1998 » NegaScout
- [Fail-soft with PVS?](https://www.stmintz.com/ccc/index.php?id=45482) by Will Singleton, CCC, March 09, 1999 » Fail-Soft
- [Re: negascout vs pvs](https://www.stmintz.com/ccc/index.php?id=54343) by Dave Gomboc, CCC, June 04, 1999

## 2000 ...

- [PVS and NegaScout](https://www.stmintz.com/ccc/index.php?id=86122) by Gian-Carlo Pascutto, CCC, January 05, 2000
- [A Question on simple Alpha-Beta versus PVS/Negascout](https://www.stmintz.com/ccc/index.php?id=102792) by Andrei Fortuna, CCC, March 21, 2000 » Alpha-Beta, NegaScout
- [What is Negascout and why is MWS PVS?](https://www.stmintz.com/ccc/index.php?id=140872) by Severi Salminen, CCC, November 24, 2000
- [Please explain the difference between PVS and NegaScout](https://www.stmintz.com/ccc/index.php?id=156886) by Severi Salminen, CCC, March 02, 2001
- [QSearch() as PVS() ?](https://www.stmintz.com/ccc/index.php?id=342287) by Matthias Gemuh, CCC, January 14, 2004
- [Fruit - Question for Fabien](https://www.stmintz.com/ccc/index.php?id=354012) by Dan Honeycutt, CCC, March 11, 2004 » Fruit, Node Types, Transposition Table, Principal Variation

[Re: Fruit - Question for Fabien](https://www.stmintz.com/ccc/index.php?id=354016) by Fabien Letouzey, CCC, March 11, 2004

- [Q. Aspiration, PVS, Fail-Soft](https://www.stmintz.com/ccc/index.php?id=373537) by David B. Weller, CCC, July 02, 2004
- [negascout and PVS?](https://www.stmintz.com/ccc/index.php?id=379100) by Peter Alloysius, CCC, July 26, 2004 » NegaScout

## 2005 ...

- [Slight enhancement to PVS](http://www.open-aurec.com/wbforum/viewtopic.php?f=4&t=6558) by Pradu Kannan, Winboard Programming Forum, June 10, 2007
- [Search questions](http://www.open-aurec.com/wbforum/viewtopic.php?f=4&t=6665) by Sven Schüle, Winboard Forum, July 17, 2007 » Fail-Soft
- [Tuning PVS](http://www.talkchess.com/forum3/viewtopic.php?f=7&t=16724&p=147515) by Aleks Peshkov, CCC, September 27, 2007
- [when to try zero window search](http://www.talkchess.com/forum/viewtopic.php?t=24883), CCC, November 14, 2008
- [PVS](http://www.talkchess.com/forum/viewtopic.php?t=26974) by Edmund Moshammer, CCC, March 12, 2009
- [Re: PVS](http://www.talkchess.com/forum/viewtopic.php?topic_view=threads&p=254906&t=26974) by Robert Hyatt, CCC, March 12, 2009
- [Re: PVS](http://www.talkchess.com/forum/viewtopic.php?topic_view=threads&p=255191&t=26974) by Vincent Diepeveen, CCC, March 14, 2009
- [No PVS at low depths?](http://www.talkchess.com/forum/viewtopic.php?t=28266) by Mark Lefler, CCC, June 05, 2009
- [A way to improve PVS](http://www.talkchess.com/forum/viewtopic.php?t=29681) by Sergei S. Markoff, CCC, September 07, 2009

## 2010 ...

- [The strengths and weaknesses of PVS](http://www.talkchess.com/forum/viewtopic.php?topic_view=threads&p=356836&t=35022) by Edmund Moshammer, CCC, June 18, 2010
- [Memory-PV-Search](http://www.talkchess.com/forum/viewtopic.php?t=38413) by Onno Garms, CCC, March 13, 2011 » Onno
- [PV Search and Transposition Table](http://www.talkchess.com/forum/viewtopic.php?t=46499) by Cheney Nattress, CCC, December 20, 2012
- [principle variation search](http://www.open-chess.org/viewtopic.php?f=5&t=2208) by nak3c, OpenChess Forum, January 09, 2013
- [Implementing pvs](http://www.open-chess.org/viewtopic.php?f=5&t=2218) by CDaley11, OpenChess Forum, January 13, 2013
- [Question about PVS and nodes type](http://www.talkchess.com/forum3/viewtopic.php?f=7&t=48137) by Patrice Duhamel, CCC, May 28, 2013 » Node Types
- [Improvement from PVS](http://www.talkchess.com/forum/viewtopic.php?t=53626) by Matthew Lai, CCC, September 09, 2014
- [Your experience with PVS + Aspiration window](http://www.talkchess.com/forum/viewtopic.php?t=53972) by Fabio Gobbato, CCC, October 07, 2014 » Aspiration Windows, PVS and Aspiration

## 2015 ...

- [Question on standard implementation of PVS+NWS](http://www.talkchess.com/forum/viewtopic.php?t=55709) by Rob Williamson, CCC, March 19, 2015
- [PVS/NegaScout: Actual benefits](http://www.talkchess.com/forum/viewtopic.php?t=60719) by Vincent Tang, CCC, July 06, 2016
- [bound type in PVS ?](http://www.talkchess.com/forum/viewtopic.php?t=62913) by Mahmoud Uthman, CCC, January 23, 2017 » Bound, Exact Score
- [LMR and PVS](http://www.open-chess.org/viewtopic.php?f=5&t=3084) by thevinenator, OpenChess Forum, February 10, 2017 » Late Move Reductions
- [PVS & Embla](http://www.talkchess.com/forum/viewtopic.php?t=65490) by Folkert van Heusden, CCC, October 19, 2017 » Embla
- [out of time in PVS](http://www.talkchess.com/forum3/viewtopic.php?f=7&t=69252) by Louis Mackenzie-Smith, CCC, December 13, 2018

## 2020 ...

- [Principal Variation Search vs. Transposition Table](http://www.talkchess.com/forum3/viewtopic.php?f=7&t=75549) by Marcel Vanthoor, CCC, October 26, 2020 » Principal Variation

# External Links

- [Principal Variation Search](http://web.archive.org/web/20070809015901/www.seanet.com/~brucemo/topics/pvs.htm) from Bruce Moreland's [Programming Topics](http://web.archive.org/web/20070811182741/www.seanet.com/%7Ebrucemo/topics/topics.htm)
- [Lecture notes for February 2, 1999 Variants of Alpha-Beta Search](https://www.ics.uci.edu/~eppstein/180a/990202b.html) by David Eppstein
- [NegaScout or Principal Variation Search from Wikipedia](https://en.wikipedia.org/wiki/Negascout)

# Video Tutorial

- A summary description of PVS and how it works by Jonathan Warkentin, [YouTube](https://en.wikipedia.org/wiki/YouTube) Video

[Watch on YouTube](https://www.youtube.com/watch?v=1YdBLgmoV_E)

# References

Up one level    [Principal Variation Search](http://web.archive.org/web/20040427015506/brucemo.com/compchess/programming/pvs.htm) from Bruce Moreland's [Programming Topics](http://web.archive.org/web/20040403211728/brucemo.com/compchess/programming/index.htm)↩︎ [PVS](http://www.talkchess.com/forum/viewtopic.php?t=26974) by Edmund Moshammer, CCC, March 12, 2009↩︎ Tony Marsland, Murray Campbell (1982). Parallel Search of Strongly Ordered Game Trees. ACM Computing Surveys, Vol. 14, No. 4, [pdf reprint](http://www.cs.ualberta.ca/%7Etony/OldPapers/strong.pdf)↩︎ Raphael Finkel, John Philip Fishburn (1980). Parallel Alpha-Beta Search on Arachne. IEEE International Conference on Parallel Processing, pp. 235-243.↩︎ [Re: Fruit - Question for Fabien](https://www.stmintz.com/ccc/index.php?id=354016) by Fabien Letouzey, CCC, March 11, 2004↩︎ John Philip Fishburn (1981). Analysis of Speedup in Distributed Algorithms. Ph.D. Thesis, [University of Wisconsin-Madison](https://en.wikipedia.org/wiki/University_of_Wisconsin-Madison), [pdf](http://www.cs.wisc.edu/techreports/1981/TR431.pdf), Calphabeta at page 167↩︎ Judea Pearl (1980). Asymptotic Properties of Minimax Trees and Game-Searching Procedures. [Artificial Intelligence](https://en.wikipedia.org/wiki/Artificial_Intelligence_%28journal%29), Vol. 14, No. 2↩︎ Judea Pearl (1980). Scout: A Simple Game-Searching Algorithm with Proven Optimal Properties. Proceedings of the First Annual National Conference on Artificial Intelligence. Stanford. [pdf](http://ftp.cs.ucla.edu/pub/stat_ser/scout.pdf)↩︎ [Re: PVS](http://www.talkchess.com/forum/viewtopic.php?topic_view=threads&p=254906&t=26974) by Robert Hyatt from CCC, March 12, 2009↩︎ `John Philip Fishburn``(``1980``).``An optimization of alpha-beta search``, SIGART Bulletin, Issue 72`↩︎ [NegaScout - A Minimax Algorithm faster than AlphaBeta](http://sc.hlrn.de/reinefeld/Research/nsc.html)↩︎ Alexander Reinefeld (1983). An Improvement to the Scout Tree-Search Algorithm. ICCA Journal, Vol. 6, No. 4, [pdf](http://www.top-5000.nl/ps/An%20improvement%20to%20the%20scout%20tree%20search%20algorithm.pdf)↩︎ Yngvi Björnsson (2002). Selective Depth-First Game-Tree Search. Ph.D. thesis, University of Alberta↩︎ Mark Winands, Jaap van den Herik, Jos Uiterwijk, Erik van der Werf (2003). Enhanced forward pruning. Accepted for publication. [pdf](http://www.personeel.unimaas.nl/m-winands/documents/Enhanced%20forward%20pruning.pdf)↩︎ [Re: Zero-width Window Null Move Search](https://www.stmintz.com/ccc/index.php?id=20868) by Guido Schimmels, CCC, June 18, 1998↩︎ [Re: PVS and NegaScout](https://www.stmintz.com/ccc/index.php?id=86134) by Yngvi Björnsson, CCC, January 05, 2000↩︎ [Negascout == PVS (with references)](https://www.stmintz.com/ccc/index.php?id=379441) by Dennis Breuker, CCC, July 28, 2004↩︎ `Dennis M. Breuker``(``1998``).`[`Ph.D. thesis: Memory versus Search in Games`](http://www.dennisbreuker.nl/thesis/index.html)↩︎ `Tony Marsland``(``1986``).``A Review of Game-Tree Pruning.````ICCA Journal, Vol. 9, No. 1``,`[`pdf`](http://www.cs.ualberta.ca/%7Etony/OldPapers/1986review.pdf)↩︎ [`NegaScout - A Minimax Algorithm faster than AlphaBeta`](http://sc.hlrn.de/reinefeld/Research/nsc.html)↩︎ `Alexander Reinefeld``(``1983``).``An Improvement to the Scout Tree-Search Algorithm.````ICCA Journal, Vol. 6, No. 4``,`[`pdf`](http://www.top-5000.nl/ps/An%20improvement%20to%20the%20scout%20tree%20search%20algorithm.pdf)↩︎ [Re: PVS](http://www.talkchess.com/forum/viewtopic.php?topic_view=threads&p=255191&t=26974) by Vincent Diepeveen, CCC, March 14, 2009↩︎
