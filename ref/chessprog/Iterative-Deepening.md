source: https://chessprogramming.org/Iterative_Deepening

# Iterative Deepening

Home * Search * Iterative Deepening

Iterative deepening search 1

---

1. [Iterative deepening （反復深化法）](http://ray.sakura.ne.jp/search_problem/i_deepening.html)↩︎

Iterative deepening (ID) has been adopted as the basic time management strategy in depth-first searches, but has proved surprisingly beneficial as far as move ordering is concerned in alpha-beta and its enhancements. It has been noticed, that even if one is about to search to a given depth, that iterative deepening is faster than searching for the given depth immediately. This is due to dynamic move ordering techniques such as; PV-, hash- and refutation moves determined in previous iteration(s), as well the history heuristic.

# How it Works

It works as follows: the program starts with a one ply search, then increments the search depth and does another search. This process is repeated until the time allocated for the search is exhausted. In case of an unfinished search, the program always has the option to fall back to the move selected in the last iteration of the search. Yet if we make sure that this move is searched first in the next iteration, then overwriting the new move with the old one becomes unnecessary. This way, also the results from the partial search can be accepted - though in case of a severe drop of the score it is wise to allocate some more time, as the first alternative is often a bad capture, delaying the loss instead of preventing it. Iterative deepening, using a transposition table, embed the depth-first algorithms like alpha-beta into a framework with best-first characteristics.

# History

Iterative or progressive deepening was first mentioned by Adriaan de Groot in Thought and Choice in Chess 1 . Richard Korf on its "discovery" in Depth-first Iterative-Deepening: An Optimal Admissible Tree Search 2 3 :

`Depth-first iterative-deepening has no doubt been rediscovered many times independently. The first use of the algorithm that is documented in the literature is in``Slate``and``Atkin's````Chess 4.5``program`4`.``Berliner```5`has observed that breadth-first search is inferior to the iterative-deepening algorithm.``Winston```6`shows that for two-person game searches where only terminal-node static evaluations are counted in the cost, the extra computation required by iterative-deepening is insignificant.``Pearl``initially suggested the iterative-deepening extension of``A*``, and``Berliner``and``Goetsch```7`have implemented such an algorithm concurrently with this work.`

Tony Marsland in Computer Chess and Search on the History of ID 8 :

`In the early 1970’s several people tried a variety of ways to control the exponential growth of the tree search. A simple fixed depth search is inflexible, especially if it must be completed within a specified time. This difficulty was noted by``Scott```9`who reported in 1969 on the effective use of an iterated search.``Jim Gillogly``, author of the``Tech``chess program`10`, coined the term iterative deepening to distinguish a full-width search to increasing depths from the progressively more focused search described by``de Groot``. About the same time``David Slate``and``Larry Atkin``(1977) sought a better time control mechanism, and introduced an improved iterated search for carrying out a progressively deeper and deeper analysis. For example, an iterated series of 1-ply, 2-ply, 3-ply ... searches is carried out, with each new search first retracing the best path from the previous iteration and then extending the search by one ply. Early experimenters with this scheme were surprised to find that the iterated search often required less time than an equivalent direct search...`

# See also

- Alpha-Beta
- Aspiration Windows
- Best-First
- Depth-First
- Effective Branching Factor
- Internal Iterative Deepening
- Move Ordering
- MTD(f)
- Odd-Even Effect
- Time Management
- Transposition Table

# Publications

## 1965 ...

- Adriaan de Groot (1965). Thought and Choice in Chess. Mouton & Co Publishers, The Hague, The Netherlands. Second edition 1978. ISBN 90-279-7914-6. ([amazon](https://www.amazon.com/gp/reader/9027979146/ref=sib_dp_pt#reader-link))
- John J. Scott (1969). A Chess Playing Program. in [Machine Intelligence 4](http://www.doc.ic.ac.uk/~shm/MI/mi4.html) (ed. Donald Michie), pp. 255-265 11

## 1970 ...

- James Gillogly (1972). The Technology Chess Program. Artificial Intelligence, Vol. 3, pp. 145-163. ISSN 0004-3702. Reprinted (1988) in Computer Chess Compendium
- David Slate, Larry Atkin (1977). CHESS 4.5 - The Northwestern University Chess Program. Chess Skill in Man and Machine, reprinted (1988) in Computer Chess Compendium

## 1980 ...

- William Fink (1982). An Enhancement to the Iterative, Alpha-Beta, Minimax Search Procedure. ICCA Newsletter, Vol. 5, No. 1
- Hans Berliner (1983). Search, Artificial Intelligence Syllabus, Department of Computer Science, Carnegie Mellon University
- Richard Korf (1984). The Complexity of Brute Force Search. Technical Report, Department of Computer Science, Columbia University 12 13
- Richard Korf (1985). Depth-first Iterative-Deepening: An Optimal Admissible Tree Search. [CiteSeerX](http://citeseerx.ist.psu.edu/viewdoc/summary?doi=10.1.1.91.288)

## 1990 ...

- Matthew L. Ginsberg, William D. Harvey (1990). Iterative Broadening. AAAI 90, [pdf](https://www.aaai.org/Papers/AAAI/1990/AAAI90-033.pdf)
- Matthew L. Ginsberg (1993). [Essentials of artificial intelligence](https://searchworks.stanford.edu/view/2746445). [Morgan Kaufmann Publishers](https://en.wikipedia.org/wiki/Morgan_Kaufmann_Publishers), 3.3 Iterative Deepening
- Alexander Reinefeld, Tony Marsland (1994). Enhanced Iterative-Deepening Search. Transactions on Pattern Analysis and Machine Intelligence, Vol. 16, No. 7, [pdf](http://webdocs.cs.ualberta.ca/%7Etony/RecentPapers/pami94.pdf)

# Forum Posts

## 1988

- [Re: "Iterative Deeping" reference wanted](http://fox.cs.vt.edu/VAD1/DOWN/AILIST/V8/N138) by Ira Baxter, AIList Digest, December 06, 1988

## 1990 ...

- [Re: Computer Chess and alpha-beta pruning](https://groups.google.com/d/msg/rec.games.chess/XQWb-ZjSsy0/gsXMq42a-FAJ) by Johannes Fürnkranz, rgc, September 22, 1993 » Alpha-Beta
- [Tricks in iterative deepening; was Re: AEGON '97](https://groups.google.com/group/rec.games.chess.computer/browse_frm/thread/530959055901588a) by Ingo Althöfer, rgcc, April 26, 1997 » Aegon 1997
- [Iterative Deepening](https://groups.google.com/d/msg/rec.games.chess.computer/24sL-_prZzw/3km3MA9ADxoJ) by John Scalo, rgcc, June 27, 1997
- [pv score oscillation](https://www.stmintz.com/ccc/index.php?id=10903) by Willie Wood, CCC, October 18, 1997
- [Selective deepening and Hashtables](https://www.stmintz.com/ccc/index.php?id=21654) by Werner Inmann, CCC, June 30, 1998

## 2000 ...

- [Iterative deepening deep increment](https://www.stmintz.com/ccc/index.php?id=156310) by Alvaro Jose Povoa Cardoso, CCC, February 27, 2001
- [iterative deepening and branching factor](http://www.talkchess.com/forum/viewtopic.php?t=14963) by J. Wesley Cleveland, CCC, July 09, 2007 » Effective Branching Factor
- [Even more search questions](http://www.open-aurec.com/wbforum/viewtopic.php?f=4&t=6666) by Sven Schüle, Winboard Forum, July 17, 2007

## 2010 ...

- [Node counts at a given depth/iteration in search](http://www.open-chess.org/viewtopic.php?f=5&t=1403) by BB+, OpenChess Forum, May 23, 2011
- [Bigger steps when branching factor < 2?](http://www.talkchess.com/forum/viewtopic.php?t=41001) by Marcel van Kervinck, CCC, November 05, 2011 » Effective Branching Factor
- [Restarting iterative deepening](http://www.talkchess.com/forum/viewtopic.php?t=58542) by Harm Geert Muller, CCC, December 09, 2015 » Aspiration Windows, Fail-Low
- [How do you signal stalemate in iterative deepening?](http://www.talkchess.com/forum/viewtopic.php?t=59281) by Kenneth Jones, CCC, February 17, 2016 » Stalemate
- [Iterative Deepening](http://www.open-chess.org/viewtopic.php?f=5&t=2959) by kickstone, OpenChess Forum, February 26, 2016
- [TT and Iterative Deepening](http://www.open-chess.org/viewtopic.php?f=5&t=2961) by kuket15, OpenChess Forum, February 26, 2016 » Transposition Table
- [Iterative Deepening Question](http://www.talkchess.com/forum/viewtopic.php?t=60916) by David Cimbalista, CCC, July 23, 2016 » Aspiration Windows
- [(I)ID and PV dropout](http://www.talkchess.com/forum/viewtopic.php?t=64321) by Harm Geert Muller, CCC, June 17, 2017 » Fail-Low, Internal Iterative Deepening

# External Links

- [Iterative deepening depth-first search from Wikipedia](https://en.wikipedia.org/wiki/Iterative_deepening_depth-first_search)
- [Iterative deepening notes](http://cseweb.ucsd.edu/%7Eelkan/130fall99/itdeep.html) by Charles Elkan
- [Chapter 3 Solving Problems by Searching](http://homepages.ius.edu/RWISMAN/C463/html/Chapter3.htm) from [C463 Artificial Intelligence Syllabus](http://homepages.ius.edu/RWISMAN/C463/html/syllabus.htm) by [Raymond F. Wisman](http://homepages.ius.edu/RWISMAN/)
- [µ-Max Search: Iterative Deepening](http://home.hccnet.nl/h.g.muller/deepen.html) by Harm Geert Muller
- Michał Urbaniak - [Always Ready](https://www.discogs.com/de/Micha%C5%82-Urbaniak-Urbaniak/release/1641078) (1977), [YouTube](https://en.wikipedia.org/wiki/YouTube) Video

feat. Michał Urbaniak, [Zbigniew Namysłowski](https://en.wikipedia.org/wiki/Zbigniew_Namys%C5%82owski), Urszula Dudziak, [Kenny Kirkland](https://en.wikipedia.org/wiki/Kenny_Kirkland), [Tony Bunn](https://en.wikipedia.org/wiki/Tony_Bunn), [Laurenda Featherstone](https://www.discogs.com/artist/608833-Lurenda-Featherstone)

[Watch on YouTube](https://www.youtube.com/watch?v=Dav_CKDVGIM)

# References

Up one Level    Adriaan de Groot (1965). Thought and Choice in Chess. Mouton & Co Publishers, The Hague, The Netherlands. Second edition 1978. ISBN 90-279-7914-6. ([amazon](https://www.amazon.com/gp/reader/9027979146/ref=sib_dp_pt#reader-link))↩︎ Richard Korf (1985). Depth-first Iterative-Deepening: An Optimal Admissible Tree Search. [CiteSeerX](http://citeseerx.ist.psu.edu/viewdoc/summary?doi=10.1.1.91.288)↩︎ Tsan-sheng Hsu (2010). Depth-First Iterative-Deepening: An Optimal Admissible Tree Search by R. E. Korf, slides as [pdf](http://www.iis.sinica.edu.tw/~tshsu/tcg2010/slides/slide3.pdf)↩︎ `David Slate``,``Larry Atkin``(``1977``).``CHESS 4.5 - The Northwestern University Chess Program.````Chess Skill in Man and Machine``, reprinted (``1988``) in``Computer Chess Compendium`↩︎ `Hans Berliner``(``1983``).``Search``. Artificial Intelligence Syllabus, Department of Computer Science,``Carnegie Mellon University`↩︎ `Patrick Winston``(``1984``).`[`Artificial Intelligence`](http://people.csail.mit.edu/phw/Books/index.html#AI)`. Addison-Wesley, Reading, MA`↩︎ `Hans Berliner``,``Gordon Goetsch``(``1984``).``A quantitative study of search methods and the effect of constraint satisfaction``. Tech. Report CMU-CS-84-147, Department of Computer Science,``Carnegie Mellon University``, Pittsburgh, PA.`↩︎ Tony Marsland (1992). Computer Chess and Search. Encyclopedia of Artificial Intelligence (2nd ed.) John Wiley & Sons, Inc. [pdf preprint](http://webdocs.cs.ualberta.ca/~tony/RecentPapers/Marsland-CCandSearch-1991.pdf)↩︎ `John J. Scott``(``1969``).``A chess-playing program``.`[`Machine Intelligence 4`](http://www.doc.ic.ac.uk/~shm/MI/mi4.html)↩︎ `James Gillogly``(``1972``).``The Technology Chess Program``. Artificial Intelligence, Vol. 3, pp. 145-163. ISSN 0004-3702. Reprinted (``1988``) in``Computer Chess Compendium`↩︎ [Re: "Iterative Deeping" reference wanted](http://fox.cs.vt.edu/VAD1/DOWN/AILIST/V8/N138) by Ira Baxter, AIList Digest, December 06, 1988↩︎ quoted by Hans Berliner, Gordon Goetsch (1985). A Study of Search Methods : The Effect of Constraint Satisfaction and Adventurousness. [pdf](https://www.ijcai.org/Proceedings/85-2/Papers/083.pdf)↩︎ Paper is mentioned by Richard Korf at Judea Pearl Symposium, 2010↩︎
