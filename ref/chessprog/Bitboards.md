source: https://chessprogramming.org/Bitboards

# Bitboards

Home * Board Representation * Bitboards

Samuel Bak - Boards Meeting 1

---

1. Samuel Bak - Boards Meeting, Oil on Canvas, 39 x 32". [Chess in the Art of Samuel Bak](http://chgs.elevator.umn.edu/asset/viewAsset/57f3b6787d58ae5f74bf8ba9#57f3b6d77d58ae5574bf8bcf), [Center for Holocaust & Genocide Studies](http://chgs.elevator.umn.edu/), University of Minnesota↩︎

Bitboards,
 also called bitsets or bitmaps, or better Square Sets, are among other things used to represent the board inside a chess program in a piece centric manner. Bitboards, are in essence, [finite sets](https://en.wikipedia.org/wiki/Finite_set) of up to [64](https://en.wikipedia.org/wiki/64_%28number%29) [elements](https://en.wikipedia.org/wiki/Element_%28mathematics%29) - all the squares of a chessboard, one bit per square. Other board games with greater board sizes may be use set-wise representations as well 1, but classical chess has the advantage that one 64-bit word or register covers the whole board. Even more bitboard friendly is Checkers with 32-bit bitboards and less piece-types than chess 2 3.

# The Board of Sets

To represent the board we typically need one bitboard for each piece-type and color - likely encapsulated inside a class or structure, or as an array of bitboards as part of a position object. A one-bit inside a bitboard implies the existence of a piece of this piece-type on a certain square - one to one associated by the bit-position.

- Square Mapping Considerations
- Standard Board-Definition

# Bitboard Basics

Of course bitboards are not only about the existence of pieces - it is a general purpose, set-wise data-structure fitting in one 64-bit register. For example, a bitboard can represent things like attack- and defend sets, move-target sets and so on.

## General Bitboard Techniques

The fundamental bitboard basics.

- General Setwise Operations
- Population Count
- BitScan
- Flipping Mirroring and Rotating
- Fill Algorithms

## Pattern and Attacks

This is basically about chess, how to calculate attack-sets and various pattern for evaluation and move generation purposes.

- Pawn Pattern and Properties
- Knight Pattern
- King Pattern
- Sliding Piece Attacks including rotated and magic bitboards
- Square Attacked By
- X-ray Attacks
- Checks and Pinned Pieces
- Design Principles

## Move Generation Issues

Bitboard aspects on move generation and static exchange evaluation (SEE).

- Bitboard Serialization
- Pieces versus Directions
- DirGolem
- SEE - The Swap Algorithm
- Attack and Defend Maps

## Miscellaneous

- Backtracking - Eight Queens puzzle with Bitboards
- De Bruijn Sequence Generator
- Quad-Bitboards
- Traversing Subsets of a Set

## Defining Bitboards

To be aware of their scalar 64-bit origin, we use so far a type defined unsigned integer U64 in our C or C++ source snippets, the scalar 64-bit long in Java. Feel free to define a distinct type or wrap U64 into classes for better abstraction and type-safety during compile time. The macro C64 will append a suffix to 64-bit constants as required by some compilers:

`typedef unsigned __int64 U64; // for the old microsoft compilers`
 `typedef unsigned long long U64; // supported by MSC 13.00+ and C99`
 `#define C64(constantU64) constantU64##ULL`

# Bitboard-History

The general approach of bitsets was proposed by Mikhail R. Shura-Bura in 1952 4 5. The bitboard method for holding a board game appears to have been invented also in 1952 by Christopher Strachey using White, Black and King bitboards in his checkers program for the Ferranti Mark 1, and in the mid 1950's by Arthur Samuel in his checkers program as well. In computer chess, bitboards were first described by Georgy Adelson-Velsky et al. in 1967 6, reprinted 1970 7 . Bitboards were used in Kaissa and in Chess. The invention and publication of Rotated Bitboards by Robert Hyatt 8 and Peter Gillgasch with Ernst A. Heinz in the 90s was another milestone in the history of bitboards. Steffan Westcott's innovations, too expensive on 32-bit x86 processors, should be revisited with x86-64 and SIMD instructions in mind. With the advent of fast 64-bit multiplication along with faster memory, Magic Bitboards as proposed by Lasse Hansen 9 and refined by Pradu Kannan 10 have surpassed Rotated.

# Analysis

The use of bitboards has spawned numerous discussions about their costs and benefits. The major points to consider are:

- Bitboards can have a high information density.
- Single populated or even empty Bitboards have a low information density.
- Bitboards are weak in answering questions like what piece if any resides on square x. One reason to keep a redundant mailbox board representation with some additional update costs during make/unmake.
- Bitboards can operate on all squares in parallel using bitwise instructions. This is one of the main arguments used by proponents of bitboards, because it allows for a flexibility in evaluation.
- Bitboards are rather handicapped on 32 bit processors, as each bitwise computation must be split into two or more instructions 11 . As most modern processors are now 64 bit, this point is somewhat diminished 12 .
- Bitboards often rely on bit-twiddling and various optimization tricks and special instructions for certain hardware architectures, such as bitscan and population count. Optimal code requires machine dependent [header-files](https://en.wikipedia.org/wiki/Header_file) in C/C++. Portable code is likely not optimal for all processors.
- Some operations on bitboards are less general, f.i. shifts. This requires additional code overhead.

# Publications

## 1970 ...

- Georgy Adelson-Velsky, Vladimir Arlazarov, Alexander Bitman, Alexander Zhivotovsky, Anatoly Uskov (1970). [Programming a Computer to Play Chess](http://iopscience.iop.org/0036-0279/25/2/R07). [Russian Mathematical Surveys, Vol. 25](http://iopscience.iop.org/0036-0279/25/2), pp. 221-262.
- David Slate, Larry Atkin (1977). CHESS 4.5 - The Northwestern University Chess Program. Chess Skill in Man and Machine, reprinted (1988) in Computer Chess Compendium » Chess

## 1980 ...

- Zdenek Zdráhal, Ivan Bratko, Alen Shapiro (1981). [Recognition of Complex Patterns Using Cellular Arrays](http://comjnl.oxfordjournals.org/content/24/3/263.abstract). [The Computer Journal, Vol. 24, No. 3](http://comjnl.oxfordjournals.org/content/24/3.toc), pp. 263-270
- Stuart Cracraft (1984). Bitmap move generation in Chess. ICCA Journal, Vol. 7, No. 3
- Burton Wendroff (1985). Attack Detection and Move Generation on the X-MP/48. ICCA Journal, Vol. 8, No. 2
- Arch D. Robison, Brian J. Hafner, Steven Skiena (1989). [Eight Pieces Cannot Cover a Chess Board](http://comjnl.oxfordjournals.org/content/32/6/567.abstract). [The Computer Journal](https://en.wikipedia.org/wiki/The_Computer_Journal), Vol. 32, No. 6, [pdf](http://comjnl.oxfordjournals.org/content/32/6/567.full.pdf)

## 1990 ...

- Ernst A. Heinz (1997). [How DarkThought Plays Chess](http://people.csail.mit.edu/heinz/dt/node2.html). ICCA Journal, Vol. 20, No. 3 » DarkThought
- Robert Hyatt (1999). [Rotated Bitmaps, a New Twist on an Old Idea](http://www.craftychess.com/hyatt/bitmaps.html). ICCA Journal, Vol. 22, No. 4 » Rotated Bitboards 13

## 2000 ...

- David Rasmussen (2004). Parallel Chess Searching and Bitboards. Master's thesis, [ps](http://www2.imm.dtu.dk/pubdb/views/edoc_download.php/3267/ps/imm3267.ps) » Parallel Search
- Borko Bošković, Sašo Greiner, Janez Brest, Viljem Žumer (2005). [The Representation of Chess Game](http://ieeexplore.ieee.org/xpl/freeabs_all.jsp?arnumber=1491153). Proceedings of the 27th International Conference on Information Technology Interfaces
- Pablo San Segundo, Ramón Galán (2005). [Bitboards: A New Approach](http://www.actapress.com/Abstract.aspx?paperId=18953). [AIA 2005](http://www.informatik.uni-trier.de/~ley/db/conf/aia/aia2005.html#SegundoG05)
- Pablo San Segundo, Ramón Galán, Fernando Matía, Diego Rodríguez-Losada, Agustín Jiménez (2006). [Efficient Search Using Bitboard Models](http://dl.acm.org/citation.cfm?id=1191130). [ICTAI 2006](http://www.informatik.uni-trier.de/~ley/db/conf/ictai/ictai2006.html#SegundoGMRJ06), [pdf](http://www.intelligentcontrol.es/diego/publications/SanSegundo_Ictai06.pdf)
- Fridel Fainshtein (2006). An Orthodox k-Move Problem-Composer for Chess Directmates. M.Sc. thesis, Bar-Ilan University, [pdf](http://www.problemschach.de/KMOVEComposer.pdf), Appendix D - 64-bit Representation, pp. 105
- Fridel Fainshtein, Yaakov HaCohen-Kerner (2006). A Chess Composer of Two-Move Mate Problems. ICGA Journal, Vol. 29, No. 1, [pdf](http://homedir.jct.ac.il/~kerner/pdf_docs/ICGA_computer_composer.pdf), Appendix E: 64-bit representation, pp. 22
- Reijer Grimbergen (2007). Using Bitboards for Move Generation in Shogi. ICGA Journal, Vol. 30, No. 1, [pdf](http://www2.teu.ac.jp/gamelab/RESEARCH/ICGAJournal2007.pdf) » Move Generation, Shogi
- James Glenn, [David Binkley](http://www.cs.loyola.edu/~binkley/) (2008) An Investigation of Hierarchical Bit Vectors. [New Topics in Theoretical Computer Science](https://www.novapublishers.com/catalog/product_info.php?products_id=6555), [pdf](http://www.cs.loyola.edu/~binkley/papers/tcsrt08-hbit-vectors.pdf)
- Shi-Jim Yen, Jung-Kuei Yang (2009). The Bitboard Design and Bitwise Computing in Connect Six. 14th Game Programming Workshop » Connect6
- Fritz Reul (2009). New Architectures in Computer Chess. Ph.D. Thesis, [pdf](https://pure.uvt.nl/ws/files/1098572/Proefschrift_Fritz_Reul_170609.pdf)

## 2010 ...

- Stefano Carlini (2010). Arimaa, a New Challenge for Artificial Intelligence. M.Sc. thesis, [University of Modena and Reggio Emilia](https://en.wikipedia.org/wiki/University_of_Modena_and_Reggio_Emilia), [pdf](http://arimaa.com/arimaa/papers/StefanoCarlini/Arimaa2.pdf) » Chapter 4, Bitboards in Arimaa
- Shi-Jim Yen, Jung-Kuei Yang, Kuo-Yuan Kao, Tai-Ning Yang (2012). [Bitboard Knowledge Base System and Elegant Search Architectures for Connect6](http://www.sciencedirect.com/science/article/pii/S0950705112001293). [Knowledge-Based Systems](https://www.journals.elsevier.com/knowledge-based-systems/), Vol. 34 » Connect6
- Cameron Browne, Stephen Tavener (2013). [Life in the Fast Lane](http://www.aifactory.co.uk/newsletter/2012_02_fast_lane.htm). AI Factory » [Conway's Game of Life](https://en.wikipedia.org/wiki/Conway%27s_Game_of_Life) within a Bitboard
- Jung-Kuei Yang, [Ping-Jung Tseng](https://dblp.uni-trier.de/pers/hd/t/Tseng:Ping=Jung) (2013). [Bitboard Connection Code Design for Connect6](https://ieeexplore.ieee.org/document/6783902). TAAI 2013 » Connect6
- Cameron Browne (2014). Bitboard Methods for Games. ICGA Journal, Vol. 37, No. 2
- Yen-Chi Chen, Shun-Shii Lin (2019). A fast nonogram solver that won the TAAI 2017 and ICGA 2018 tournaments. ICGA Journal, Vol. 41, No. 1 » Nonogram

# Forum Posts

## 1994

- [bitboard move generation](https://groups.google.com/d/msg/rec.games.chess/vvl1nLv1MD8/oHVKdLXuiaUJ) by Robert Hyatt, rgc, October 25, 1994
- [bitboard move generator](https://groups.google.com/d/msg/rec.games.chess/106wKFeI8BA/zNuzu-2aMowJ) by Joël Rivat, rgc, November 13, 1994
- [bitboard position evaluations](https://groups.google.com/d/msg/rec.games.chess/M4CKCmqDNkI/TjVJEQY0GC0J) by Robert Hyatt, rgc, November 17, 1994 » Evaluation

## 1995 ...

- [Chess programming using bitboards](https://groups.google.com/group/rec.games.chess.computer/browse_frm/thread/71f7b5ee3764f082) by Joël Rivat, rgcc, August 18, 1995
- [Bit Board Bonkers??](https://groups.google.com/group/rec.games.chess.computer/browse_frm/thread/834fa3c273fafffe/cab7c12ea99e9a35) by Dave, rec.games.chess.computer, July 28, 1997
- [Efficient Bitboard Implementation on 32-bit Architecture](http://www.stmintz.com/ccc/index.php?id=30562) by Roberto Waldteufel, CCC, October 25, 1998
- [Bitboard question](http://www.stmintz.com/ccc/index.php?id=34506) by Werner Inmann, CCC, December 02, 1998
- [Bitboards](http://www.stmintz.com/ccc/index.php?id=34852) by Frank Phillips, CCC, December 05, 1998
- [bitboards in java?](http://www.stmintz.com/ccc/index.php?id=48176) by vitor, CCC, April 06, 1999 » Java
- [BitBoards](http://www.stmintz.com/ccc/index.php?id=53446) by Frank Phillips, CCC, May 29, 1999
- [Bitboard user's information request](http://www.stmintz.com/ccc/index.php?id=71880) by Robert Hyatt, CCC, October 05, 1999 » Rotated Bitboards 14

## 2000 ...

- [To bitboard or not to bitboard?](http://www.stmintz.com/ccc/index.php?id=313504) by Tord Romstad, CCC, August 30, 2003
- [How important are Bitboards?](http://www.stmintz.com/ccc/index.php?id=352040) by Martin Schreiber, CCC, February 29, 2004
- [questions for bitboard experts](http://www.open-aurec.com/wbforum/viewtopic.php?f=4&t=516Two) by Tord Romstad, Winboard Forum, November 06, 2004 » In Between, Piece-Lists

## 2005 ...

- [Bitboard question](http://www.open-aurec.com/wbforum/viewtopic.php?f=4&t=4521) by Tord Romstad, Winboard Forum, March 14, 2006
- [Yet another new bitboard move generation method](http://www.open-aurec.com/wbforum/viewtopic.php?f=4&t=5623) by Zach Wegner, Winboard Forum, September 22, 2006 » Titboards

[Re: Yet another new bitboard move generation method](http://www.open-aurec.com/wbforum/viewtopic.php?f=4&t=5623&start=6) by Harm Geert Muller, Winboard Forum, October 01, 2006 15

- [Speedup with bitboards on 64-bit CPUs](http://www.talkchess.com/forum/viewtopic.php?t=13426) by Tord Romstad, CCC, April 27, 2007
- [Speedup by bitboards](http://www.open-aurec.com/wbforum/viewtopic.php?f=4&t=6651) by Onno Garms, Winboard Forum, July 13, 2007
- [BitBoard representations of the board](http://www.talkchess.com/forum/viewtopic.php?t=17138) by Uri Blass, CCC, October 14, 2007
- [compact bitboard move generator](http://www.talkchess.com/forum3/viewtopic.php?f=7&t=19837) by Robert Hyatt, CCC, February 25, 2008 » Bitboard Serialization, Move Generation
- [Bitboards / move generation on larger boards](http://www.talkchess.com/forum/viewtopic.php?t=25917) by Gregory Strong, CCC, January 09, 2009
- [Bitboard techniques in Xiangqi](http://www.talkchess.com/forum/viewtopic.php?t=26527) by Harm Geert Muller, CCC, February 12, 2009 » Chinese Chess
- [Bitboards using 2 DOUBLE's ?](http://www.talkchess.com/forum/viewtopic.php?t=28207) by Carey, CCC, June 02, 2009 » Double

## 2010 ...

- [Bitboard implementation, how much time?](http://www.talkchess.com/forum/viewtopic.php?t=42108) by Ed Schröder, CCC, January 22, 2012
- [64 bits for 64 squares ?](http://macechess.blogspot.de/2013/04/64-bits-for-64-squares.html) by Thomas Petzke, [mACE Chess](http://macechess.blogspot.de/), April 28, 2013 » Population Count
- [Bitboard Tricks for Large Chess Variants](http://www.talkchess.com/forum/viewtopic.php?t=54208) by Ed Trice, CCC, November 01, 2014

## 2015 ...

- [Bitboard database code samples](http://www.talkchess.com/forum/viewtopic.php?t=56476) by Steven Edwards, CCC, May 25, 2015 » Symbolic
- [M42 - A C++ library for Bitboard attack mask generation](http://www.talkchess.com/forum/viewtopic.php?t=60007) by Syed Fahad, CCC, April 30, 2016
- [Checkers Bitboard representation](http://www.talkchess.com/forum/viewtopic.php?t=64487) by Pranav Deshpande, CCC, July 02, 2017 » Checkers
- [Bitboards and Java](http://www.talkchess.com/forum/viewtopic.php?t=65724) by Fred Hamilton, CCC, November 14, 2017 » Java
- [Re: Pawn move generation in bitboards](http://www.talkchess.com/forum3/viewtopic.php?f=7&t=72461&start=3) by Álvaro Begué, CCC, December 05, 2019 » C++, Pawn Pattern and Properties

## 2020 ...

- [M42 - C++ Library for Bitboard Attack Mask Generation](http://www.talkchess.com/forum3/viewtopic.php?f=7&t=73830) by Syed Fahad, CCC, May 04, 2020 16
- [Bitboard board representation](http://www.talkchess.com/forum3/viewtopic.php?f=7&t=76083) by Elias Nilsson, CCC, December 17, 2020
- [Thought bitboards was faster :-)](http://www.talkchess.com/forum3/viewtopic.php?f=7&t=76548) by Daniel Anulliero, CCC, February 10, 2021
- [Are Bitboards More Intoxicating Than They Are Good?](http://www.talkchess.com/forum3/viewtopic.php?f=7&t=76690) by Mike Sherwin, CCC, February 24, 2021
- [Best bitboard design?](http://www.talkchess.com/forum3/viewtopic.php?f=7&t=77299) by Martin Bryant, CCC, May 13, 2021
- [The cost of check & discovered check in bitboards](https://www.talkchess.com/forum3/viewtopic.php?t=78160) by Bill Beame, CCC, September 13, 2021
- [Bitboards ?: C# vs C++](https://www.talkchess.com/forum3/viewtopic.php?f=7&t=78680) by Bill Beame, CCC, November 17, 2021 » C#, C++
- [Move generation for bitboards](https://www.talkchess.com/forum3/viewtopic.php?f=7&t=79365) by Elias Nilsson, CCC, February 16, 2022

# Viewer & Calculator

- Bibob
- [Bitboard Calculator](https://gekomad.github.io/Cinnamon/BitboardCalculator) by Giuseppe Cannella
- [Free Chess Bitboard Viewer - Computer Chess Programming](http://www.chessprogramming.net/computerchess/free-chess-bitboard-viewer/) by Steve Maughan
- [New free tool : Bitboards Helper](http://www.chess2u.com/t2159-new-free-tool-bitboards-helper) by Julien Marcel

# External Links

## Descriptions

- [Bitboards from Wikipedia](https://en.wikipedia.org/wiki/Bitboard)
- [Bit-Array from Wikipedia](https://en.wikipedia.org/wiki/Bit_array)
- [Bitboard-History from Wikipedia](https://en.wikipedia.org/wiki/Bitboard#History)
- [Chess board representations](https://craftychess.com/hyatt/boardrep.html) by Robert Hyatt
- [Bitboards (aka bitmaps)](https://web.archive.org/web/20081007034904/http://webpages.charter.net/tlikens/bitmaps/bit_intro.html) by Tom Likens ([Wayback Machine](https://en.wikipedia.org/wiki/Wayback_Machine), 2008)
- [An Introduction to BITBOARDS](http://www.fzibi.com/cchess/bitboards.htm) by Franck Zibi
- [Bitwise Optimization in Java: Bitfields, Bitboards, and Beyond](https://web.archive.org/web/20050205014648/http://www.onjava.com/pub/a/onjava/2005/02/02/bitsets.html) by Glen Pepicelli, ([Wayback Machine](https://en.wikipedia.org/wiki/Wayback_Machine), 2005), [O'Reilly's](https://en.wikipedia.org/wiki/O%27Reilly_Media) [OnJava.com](https://web.archive.org/web/20050203015229/http://onjava.com/) » Java, Bit-Twiddling
- [Chess and Bitboards](https://pages.cs.wisc.edu/~psilord/blog/data/chess-pages/index.html) by [Peter Keller](https://pages.cs.wisc.edu/~psilord/)
- [Position Representation - Computer Architecture and Languages Laboratory](https://labraj.feri.um.si/en/Position_Representation), University of Maribor
- [Newest 'bitboard' Questions](https://stackoverflow.com/questions/tagged/bitboard) - [Stack Overflow](https://en.wikipedia.org/wiki/Stack_Overflow)

## Libraries

- [GitHub - sinandredemption/M42: C++ Library for Bitboard Attack Mask Generation](https://github.com/sinandredemption/M42) by Syed Fahad
- [GitHub - kz04px/libchess: C++ chess library](https://github.com/kz04px/libchess)

## Misc

- [Setunion](https://www.halloherne.de/artikel/1-september-setunion-in-den-flottmann-hallen-38570.htm?k=tick) - [Malletmania](https://inherne.net/weltspitze-des-vibraphon-jazz/), Flottmann-Hallen 17, March 10, 2019, [YouTube](https://en.wikipedia.org/wiki/YouTube) Video

[Kerstin Fabry](http://www.saxophonquartett-quattro-venti.de/#about), [Christian Ribbe](https://www.lokalkompass.de/tag/christian-ribbe), [Elmar Dissinger](https://www.folkwang-uni.de/home/theater/studiengaenge/physical-theatre/lehrende/detail-lehrende/personen-detail/elmar-dissinger/), [Martin Siehoff](https://de.wikipedia.org/wiki/Pee_Wee_Bluesgang), [Carlotta Ribbe](https://www.halloherne.de/artikel/carlotta-ribbe-und-setunion-31511.htm), [Ludger Bollinger](http://guitartist-quartett.com/quartett_ludger.htm)

[Watch on YouTube](https://www.youtube.com/watch?v=srfRiWZcCg4)

# References

Up one level    Reijer Grimbergen (2007). Using Bitboards for Move Generation in Shogi. ICGA Journal, Vol. 30, No. 1, [pdf](http://www2.teu.ac.jp/gamelab/RESEARCH/ICGAJournal2007.pdf)↩︎ [Checker Bitboards Tutorial](http://www.3dkingdoms.com/checkers/bitboards.htm) by Jonathan Kreuzer↩︎ [Checkers Bitboard representation](http://www.talkchess.com/forum/viewtopic.php?t=64487) by Pranav Deshpande, CCC, July 02, 2017↩︎ [Lazar A. Lyusternik](https://en.wikipedia.org/wiki/Lazar_Lyusternik), [Aleksandr A. Abramov](http://www.mathnet.ru/php/person.phtml?personid=30351&option_lang=eng), [Victor I. Shestakov](https://en.wikipedia.org/wiki/Victor_Shestakov), Mikhail R. Shura-Bura (1952). Programming for High-Speed Electronic Computers. (Программирование для электронных счетных машин)↩︎ Andrey Ershov, Mikhail R. Shura-Bura (1980). [The Early Development of Programming in the USSR](http://ershov.iis.nsk.su/archive/eaindex.asp?lang=2&gid=910). in [Nicholas C. Metropolis](https://en.wikipedia.org/wiki/Nicholas_C._Metropolis) (ed.) [A History of Computing in the Twentieth Century](http://dl.acm.org/citation.cfm?id=578384). [Academic Press](https://en.wikipedia.org/wiki/Academic_Press), [preprint pp. 43](http://ershov.iis.nsk.su/archive/eaimage.asp?did=28792&fileid=173670)↩︎ [Early Reference on Bit-Boards](https://groups.google.com/group/rec.games.chess/browse_frm/thread/0e3a93f45ff07d31#) by Tony Warnock, rec.games.chess, October 29, 1994↩︎ Georgy Adelson-Velsky, Vladimir Arlazarov, Alexander Bitman, Alexander Zhivotovsky, Anatoly Uskov (1970). [Programming a Computer to Play Chess](http://iopscience.iop.org/0036-0279/25/2/R07). [Russian Mathematical Surveys, Vol. 25](http://iopscience.iop.org/0036-0279/25/2), pp. 221-262.↩︎ Robert Hyatt (1999). [Rotated Bitmaps, a New Twist on an Old Idea](http://www.craftychess.com/hyatt/bitmaps.html). ICCA Journal, Vol. 22, No. 4↩︎ [Fast(er) bitboard move generator](http://www.open-aurec.com/wbforum/viewtopic.php?t=5015) by Lasse Hansen, Winboard Forum, June 14, 2006↩︎ [List of magics for bitboard move generation](http://www.open-aurec.com/wbforum/viewtopic.php?t=5441) by Pradu Kannan, Winboard Forum, August 23, 2006↩︎ [Efficient Bitboard Implementation on 32-bit Architecture](http://www.stmintz.com/ccc/index.php?id=30562) by Roberto Waldteufel, CCC, October 25, 1998↩︎ [Speedup by bitboards](http://www.open-aurec.com/wbforum/viewtopic.php?f=4&t=6651) by Onno Garms, Winboard Forum, July 13, 2007↩︎ [Bitboard user's information request](http://www.stmintz.com/ccc/index.php?id=71880) by Robert Hyatt, CCC, October 05, 1999↩︎ Robert Hyatt (1999). [Rotated Bitmaps, a New Twist on an Old Idea](http://www.craftychess.com/hyatt/bitmaps.html). ICCA Journal, Vol. 22, No. 4↩︎ [Re: multi-dimensional piece/square tables](http://www.talkchess.com/forum3/viewtopic.php?f=7&t=52861&start=8) by Tony P., CCC, January 28, 2020↩︎ [GitHub - sinandredemption/M42: C++ Library for Bitboard Attack Mask Generation](https://github.com/sinandredemption/M42)↩︎ Flottmann-Hallen in [Herne](https://en.wikipedia.org/wiki/Herne,_North_Rhine-Westphalia), [North Rhine-Westphalia](https://en.wikipedia.org/wiki/North_Rhine-Westphalia), [Germany](https://en.wikipedia.org/wiki/Germany), part of The Industrial Heritage Trail of the [Ruhr area](https://en.wikipedia.org/wiki/Ruhr)↩︎
