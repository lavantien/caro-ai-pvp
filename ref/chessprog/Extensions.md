source: https://chessprogramming.org/Extensions

# Extensions

Home * Search * Selectivity * Extensions

Many programs extend certain moves to try and find better moves faster, or to resolve tactical "noise" resulting from the horizon effect. To extend a move, its search depth (draft) is incremented by some amount, typically one ply.

# Inside the Loop

Some extensions may be determined inside the move loop before or after making the move, the latter case often delayed to the recursively called search routine by some programs:

```
for each move m € of all moves {
  makeMove(m);
  int ext = determineExtension( m, depth, node_type, ...); /* 0 or 1 */
  score = -search(ply + 1, depth + ext - 1, -beta, -alpha);
  unmakeMove(m);
  ...
}
```

# Classification

Bruce Moreland has classified extensions as either win-seeking, loss-seeking, or neutral 1 :

1. Win-seeking extension: If I stop searching now I'll fail low, but I think there might be something good here if I look a little further.
2. Loss-seeking extension: If I stop searching now I'll fail high, but I think I'm in trouble.
3. Neutral extension: This is a forcing sequence, and if I stop searching now I won't know how it ends.

# Types

| type | typical class |
|---|---|
| Botvinnik-Markoff Extension | loss-seeking |
| Capture Extensions | neutral |
| Check Extensions | neutral |
| Mate Threat Extensions | win-seeking |
| One Reply Extensions | loss-seeking |
| Passed Pawn Extensions | neutral |
| PV Extensions | neutral |
| Recapture Extensions | neutral |
| Singular Extensions | neutral |

# Fractional Extensions

This technique involves passing fractional depths to the search function. This is typically implemented by defining one ply to be a number greater than one. Then an extension can be added that does not yet extend the search, but further down the tree may cause an extension when another fractional extension causes the net extension to exceed one ply. Fractional extensions were first described by David Levy's, David Broughton's and Mark Taylor's paper on their SEX Algorithm, in conjunction with "negative" extensions aka. fractional reductions and even LMR 2.

# Conditions/Restrictions

Some programs restrict extensions, with either a maximum limit, or via other conditions, such as depth or iteration. Care must be taken so that the search is not extended infinitely (see search explosion). Some programs vary the extension based on the expected node type. For example, in an expected All-node, it might use 1/2 a ply extension for a pawn to the 7th, but a full ply on the PV-node research.

## Non-reductions

In contemporary, heavily reducing programs former typical extensions are often used in an inverted manner: to flag moves as exempt from reductions.

# See also

- Reductions

Late Move Reductions

- Pre-Search
- Pruning
- SEX Algorithm

# Publications

## 1980 ...

- Hermann Kaindl (1983). Searching to Variable Depth in Computer Chess. Proceedings of [IJCAI 83](http://www.informatik.uni-trier.de/~ley/db/conf/ijcai/ijcai83.html), pp. 760-762. Karlsruhe. [pdf](http://ijcai.org/Past%20Proceedings/IJCAI-83-VOL-2/PDF/039.pdf)
- Thomas Anantharaman, Murray Campbell, Feng-hsiung Hsu (1988). Singular extensions: Adding Selectivity to Brute-Force Searching. AAAI Spring Symposium, Computer Game Playing, pp. 8-13. Also published in ICCA Journal, Vol. 11, No. 4, republished (1990) in [Artificial Intelligence](https://en.wikipedia.org/wiki/Artificial_Intelligence_%28journal%29), Vol. 43, No. 1, pp. 99-109. ISSN 0004-3702.
- David Levy, David Broughton, Mark Taylor (1989). The SEX Algorithm in Computer Chess. ICCA Journal, Vol. 12, No. 1

## 1990 ...

- Thomas Anantharaman (1991). Extension Heuristics. ICCA Journal, Vol. 14, No. 2
- Chun Ye (1992). Experiments in Selective Search Extensions. MSc. thesis, Department of Computing Science, University of Alberta, [pdf](https://era.library.ualberta.ca/public/datastream/get/uuid:e4fbf48d-7603-490f-85cc-5497bbecf5a8/DS1)
- Chun Ye, Tony Marsland (1992). Experiments in Forward Pruning with Limited Extensions. ICCA Journal, Vol. 15, No. 2
- Chun Ye, Tony Marsland (1992). Selective Extensions in Game-Tree Search. Heuristic Programming in AI 3
- Don Beal, Martin C. Smith (1995). Quantification of Search-Extension Benefits. ICCA Journal, Vol. 18, No. 4

## 2000 ...

- Yngvi Björnsson, Tony Marsland (2001). Learning Search Control in Adversary Games. Advances in Computer Games 9, pp. 157-174. [pdf](http://www.ru.is/faculty/yngvi/pdf/BjornssonM01b.pdf)
- Yoshimasa Tsuruoka, Daisaku Yokoyama, Takashi Chikayama (2002). [Game-Tree Search Algorithm based on Realization Probability](http://citeseerx.ist.psu.edu/viewdoc/summary?doi=10.1.1.2.9258). ICGA Journal, Vol. 25, No. 3, [pdf](http://citeseerx.ist.psu.edu/viewdoc/download?doi=10.1.1.2.9258&rep=rep1&type=pdf), [pdf](http://www-tsujii.is.s.u-tokyo.ac.jp/~tsuruoka/papers/icga02.pdf)
- David Levy (2002) [SOME COMMENTS ON REALIZATION PROBABILITIES AND THE SEX ALGORITHM](http://ilk.uvt.nl/icga/journal/contents/content25-3.htm#SOME%20COMMENTS%20ON%20REALIZATION%20PROBABILITIES). ICGA Journal, Vol. 25, No. 3

## 2010 ...

- Pálmi Skowronski, Yngvi Björnsson, Mark Winands (2010). [Automated Discovery of Search-Extension Features](http://www.sciweavers.org/publications/automated-discovery-search-extension-features). Advances in Computer Games 12, [pdf](http://www.hr.is/faculty/yngvi/pdf/SkowronskiBW09.pdf) 3

# Forum Posts

## 1996 ...

- [Fractional depth increments](https://groups.google.com/d/msg/rec.games.chess.computer/1uVIWZFB41k/VUcAUkzyFd0J) by S. Read, rgcc, January 18, 1996
- [Crafty V11.3](https://groups.google.com/d/msg/rec.games.chess.computer/tcjwWnFhXt4/ifkLE7GwfSEJ) by Robert Hyatt, rgcc, October 22, 1996 » Crafty
- ["Suspicious move" extension](https://www.stmintz.com/ccc/index.php?id=12201) by David Eppstein, CCC, November 20, 1997
- [Extensions?!](https://www.stmintz.com/ccc/index.php?id=13993) by Daniel Homan, CCC, January 13, 1998
- [Move Extensions](https://www.stmintz.com/ccc/index.php?id=17954) by Roberto Waldteufel, CCC, May 04, 1998
- [Extend or not extend in a nullmove tree](https://www.stmintz.com/ccc/index.php?id=20167) by Roland Pfister, CCC, June 08, 1998 » Null Move Pruning
- [Selective deepening and Hashtables](https://www.stmintz.com/ccc/index.php?id=21654) by Werner Inmann, CCC, June 30, 1998
- [search extension](https://www.stmintz.com/ccc/index.php?id=21888) by Werner Inmann, CCC, July 08, 1998
- [King danger extensions](https://www.stmintz.com/ccc/index.php?id=43380) by James Robertson, CCC, February 16, 1999
- [full-ply search extension leads to crash city!](https://www.stmintz.com/ccc/index.php?id=45304) by Dave Gomboc, CCC, March 07, 1999
- [Extensions and Futility Pruning](https://www.stmintz.com/ccc/index.php?id=50627) by James Robertson, CCC, May 04, 1999 » Futility Pruning
- [Q. about Rebel extensions](https://www.stmintz.com/ccc/index.php?id=52090) by Rémi Coulom, CCC, May 18, 1999 » Rebel

## 2000 ...

- [What is the use of extensions?](https://www.stmintz.com/ccc/index.php?id=87191) by Leonid, CCC, January 09, 2000
- [No explosions](https://www.stmintz.com/ccc/index.php?id=206802) by Matthias Gemuh, CCC, January 11, 2002 » Search Explosion
- [Extensions](https://www.stmintz.com/ccc/index.php?id=208272) by Benny Antonsson, CCC, January 18, 2002
- [I need help with Search/Selective Extension](https://www.stmintz.com/ccc/index.php?id=247564) by Scott Farrell, CCC, August 24, 2002
- [wacnew.epd & single search improvements (extensions)](http://www.open-aurec.com/wbforum/viewtopic.php?f=18&t=43060) by Stefan Knappe, Winboard Forum, June 19, 2003 » Matador, Win at Chess
- [Problem with extending to maxdepth](https://www.stmintz.com/ccc/index.php?id=303131) by Albert Bertilsson, CCC, June 26, 2003
- [Evaluation-based Reductions and/or Extensions](https://www.stmintz.com/ccc/index.php?id=338851) by Tom Likens, CCC, December 28, 2003 » Reductions
- [extensions + reductions + pruning = confusion](https://www.stmintz.com/ccc/index.php?id=356488) by Johan de Koning, CCC, March 24, 2004 (was [Shredder 8 secret: search depth?](https://www.stmintz.com/ccc/index.php?id=356109))

## 2005 ...

- [Limiting extensions](http://www.open-aurec.com/wbforum/viewtopic.php?f=4&t=1754&p=8190) by David B. Weller, Winboard Forum, February 23, 2005 » GES
- [Worthless extension ideas...](http://www.open-aurec.com/wbforum/viewtopic.php?f=4&t=4952) by mjlef, Winboard Forum, June 06, 2006
- [PVS extension up](http://www.talkchess.com/forum/viewtopic.php?t=14594) by Ed Schröder, CCC, June 21, 2007
- [Delaying Extensions Idea (does anyone do this)?](http://www.talkchess.com/forum/viewtopic.php?t=14860) by Mark Lefler, CCC, July 03, 2007
- [Threat extension](http://www.talkchess.com/forum/viewtopic.php?t=20680) by Harm Geert Muller, CCC, April 15, 2008
- [Search extensions at promising trajectories](http://www.talkchess.com/forum/viewtopic.php?t=21403) by Reinhard Scharnagl, CCC, May 28, 2008
- [extensions](http://www.open-aurec.com/wbforum/viewtopic.php?t=49446) by Daniel Shawul, Winboard programming Forum, August 26, 2008
- [Extensions, anyone?](http://www.talkchess.com/forum/viewtopic.php?t=26632) by Gregory Strong, CCC, February 20, 2009
- [Extensions: everywhere or near the tips?](http://www.talkchess.com/forum/viewtopic.php?t=27017) by Alvaro Cardoso, CCC, March 15, 2009
- [Stupid Extension Problem/Question](http://www.talkchess.com/forum/viewtopic.php?t=31220) by John Merlino, CCC, December 23, 2009 » Repetitions

## 2010 ...

- [Problem with exploding tree because of extensions](http://www.talkchess.com/forum/viewtopic.php?t=31505) by Oliver Brausch, CCC, January 05, 2010 » Search Explosion
- [Restricting extensions to the left most branches?](http://www.talkchess.com/forum/viewtopic.php?t=32940) by Michael Sherwin, CCC, February 27, 2010
- ["Automated Discovery of Search Extensions"](http://www.open-chess.org/viewtopic.php?f=5&t=248) by Mark Watkins, OpenChess - Programming and Technical Discussions, June 22, 2010
- [Loss-seeking extension/threatened pieces](http://www.talkchess.com/forum/viewtopic.php?t=40984) by Evert Glebbeek, CCC, November 03, 2011
- [search extensions](http://www.talkchess.com/forum/viewtopic.php?t=54281) by Robert Hyatt, CCC, November 08, 2014 » Singular Extensions

## 2015 ...

- [Adding an Extension Results in Deeper General Search!](http://www.talkchess.com/forum/viewtopic.php?t=56311) by Steve Maughan, CCC, May 10, 2015 » Passed Pawn Extensions
- [Search extensions](http://www.open-chess.org/viewtopic.php?f=5&t=2968) by ppyvabw, OpenChess Forum, March 17, 2016
- [Extensions in the days of LMR?](http://www.talkchess.com/forum/viewtopic.php?t=59598) by Martin Fierz, CCC, March 22, 2016 » LMR
- [Depth extensions and effect on transposition queries](http://www.talkchess.com/forum/viewtopic.php?t=67131) by Kenneth Jones, CCC, April 16, 2018 » Transposition Table
- [Horizon effect and extensions](http://www.talkchess.com/forum3/viewtopic.php?f=7&t=67823) by Vivien Clauzon, CCC, June 25, 2018 » Horizon Effect
- [delaying tactics: prune or extend?](http://www.talkchess.com/forum3/viewtopic.php?f=7&t=70165) by Harm Geert Muller, CCC, March 10, 2019 » Selectivity, Tactics

# External Links

## Search Extension

- [Search Extension](http://web.archive.org/web/20070607151732/www.brucemo.com/compchess/programming/extensions.htm) from Bruce Moreland's [Programming Topics](http://web.archive.org/web/20070607231311/www.brucemo.com/compchess/programming/index.htm)
- [Computer Chess Programming Theory - Search Extensions](http://www.frayn.net/beowulf/theory.html#extend) by Colin Frayn
- [Programming Details - Slow Chess | Extensions Used, Detailed Description](http://www.3dkingdoms.com/chess/implementation.htm) by Jonathan Kreuzer » Slow Chess

## Misc

- [Extension from Wikipedia](https://en.wikipedia.org/wiki/Extension)
- [Extension (metaphysics) from Wikipedia](https://en.wikipedia.org/wiki/Extension_%28metaphysics%29)
- [Extension (music) from Wikipedia](https://en.wikipedia.org/wiki/Extension_%28music%29)
- [Jazz chord - Extensions from Wikipedia](https://en.wikipedia.org/wiki/Jazz_chord#Extensions)

# References

Up one level    [Search Extension](http://web.archive.org/web/20070607151732/www.brucemo.com/compchess/programming/extensions.htm) from Bruce Moreland's [Programming Topics](http://web.archive.org/web/20070607231311/www.brucemo.com/compchess/programming/index.htm)↩︎ David Levy, David Broughton, Mark Taylor (1989). The SEX Algorithm in Computer Chess. ICCA Journal, Vol. 12, No. 1↩︎ ["Automated Discovery of Search Extensions"](http://www.open-chess.org/viewtopic.php?f=5&t=248) by Mark Watkins, OpenChess - Programming and Technical Discussions, June 22, 2010↩︎
