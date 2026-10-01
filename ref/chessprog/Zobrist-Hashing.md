source: https://chessprogramming.org/Zobrist_Hashing

# Zobrist Hashing

Home * Search * Transposition Table * Zobrist Hashing

[King Wen sequence](https://en.wikipedia.org/wiki/King_Wen_sequence) 1 2

---

1. [King Wen sequence](https://en.wikipedia.org/wiki/King_Wen_sequence), [I Ching](https://en.wikipedia.org/wiki/I_Ching) [divination](https://en.wikipedia.org/wiki/I_Ching_divination) involves obtaining a [Hexagram](https://en.wikipedia.org/wiki/Hexagram_%28I_Ching%29) by random generation↩︎
2. All of Cage's [music](https://en.wikipedia.org/wiki/Music_of_Changes) since 1951 was composed using [chance](https://en.wikipedia.org/wiki/John_Cage#Chance) procedures, most commonly using the [I Ching](https://en.wikipedia.org/wiki/I_Ching)↩︎

Zobrist Hashing,
 a technique to transform a board position of arbitrary size into a number of a set length, with an equal distribution over all possible numbers, invented by Albert Zobrist 1. In an early Usenet post in 1982, Tom Truscott mentioned Jim Gillogly's n-bit hashing technique 2, who apparently read Zobrist's paper early, and credits Zobrist in a 1997 rgcc post 3. Zobrist Hashing is an instance of [tabulation hashing](https://en.wikipedia.org/wiki/Tabulation_hashing) 4, a method for constructing [universal families of hash functions](https://en.wikipedia.org/wiki/Universal_hashing) by combining [table lookup](https://en.wikipedia.org/wiki/Lookup_table) with exclusive or operations. Zobrist Hashing was rediscovered by J. Lawrence Carter and Mark N. Wegman in 1977 5 and studied in more detail by Mihai Pătrașcu and Mikkel Thorup in 2011 6 7.

The main purpose of Zobrist hash codes in chess programming is to get an almost unique index number for any chess position, with a very important requirement that two similar positions generate entirely different indices. These index numbers are used for faster and more space-efficient Hash tables or databases, e.g. transposition tables and opening books.

# Metamorphosis

M. C. Escher, [Metamorphosis](https://en.wikipedia.org/wiki/Metamorphosis) III, 1967-1968 8

# Initialization

At program initialization, we generate an array of pseudorandom numbers 9 10:

- One number for each piece at each square
- One number to indicate the side to move is black
- Four numbers to indicate the castling rights, though usually 16 (2^4) are used for speed
- Eight numbers to indicate the file of a valid En passant square, if any

This leaves us with an array of 781 (12*64 + 1 + 4 + 8) random numbers. Since pawns don't happen on first and eighth rank, one might be fine with 12*64, though. There are even proposals and implementations to use overlapping keys from unaligned access up to an array of only 12 numbers for every piece and to rotate that number by square 11 12.

Programs usually implement their own Pseudorandom number generator (PRNG), both for better quality random numbers than standard library functions, and also for reproducibility. This means that whatever platform the program is run on, it will use the exact same set of Zobrist keys. This is also useful for things like opening books, where the positions in the book can be stored by hash key and be used portably across machines, considering endianness.

# Runtime

If we now want to get the Zobrist hash code of a certain position, we initialize the hash key by xoring all random numbers linked to the given feature, e.g. the initial position:

```
[Hash for White Rook on a1] xor [Hash for White Knight on b1] xor [Hash for White Bishop on c1] xor ... ( all pieces )
... xor [Hash for White king castling] xor [Hash for White queen castling] xor ... ( all castling rights )
```

The fact that xor-operation is [own inverse](https://en.wikipedia.org/wiki/Involution) and can be undone by using the same xor-operation again, is often used by chess engines. It allows a fast incremental update of the hash key during make or unmake moves. E.g., for a White Knight that jumps from b1 to c3, capturing a Black Bishop, these operations are performed:

```
[Original Hash of position] xor [Hash for White Knight on b1] ... ( removing the knight from b1 )
... xor [Hash for Black Bishop on c3] ( removing the captured bishop from c3 )
... xor [Hash for White Knight on c3] ( placing the knight on the new square )
... xor [Hash for Black to move] ( change sides)
```

# Collisions

Key collisions or type-1 errors are inherent in using Zobrist keys with far fewer bits than required to encode all reachable chess positions.

## Theory

An important issue is the question of what size the hash keys should have. Smaller hash keys are faster and more space-efficient, while larger ones reduce the risk of a hash collision. A collision occurs if two positions map the same key 13 . The dangers of which were well assessed by Robert Hyatt and Anthony Cozzie in their paper Hash Collisions Effect 14. Usually, 64-bit is used as a standard size in modern chess programs.

Hash collisions demonstrate the [birthday "paradox"](https://en.wikipedia.org/wiki/Birthday_problem), which is to say the chance of collisions approaches certainty at around the square root of the number of possible keys, contrary to some people's expectations. You can expect to encounter a collision in a 32-bit hash when you have evaluated sqrt(2 ^ 32) == 2 ^ 16 or around 65 thousand positions. With a 64-bit hash, you can expect a collision after about 2 ^ 32 or 4 billion positions.

## Praxis

Post by Jonathan Schaeffer 15 :

`... I can speak from experience here. In the early versions of my chess program``Phoenix``, I generated my Zobrist hash numbers using my student ID number as a seed, naively thinking the random numbers generated by this seed would be good enough. A few years later I put code in to detect when my 32-bit hash key matched the wrong position. To my surprise, there were``lots``of errors. I changed my seed to another number, and the error rate dropped dramatically. With this better seed, it became very, very rare to see a hash error. All randomly generated numbers are not the same!`

## Lack a True Integer Type

Some languages (such as JavaScript and [Lua](https://en.wikipedia.org/wiki/Lua_%28programming_language%29)) only have a 64-bit floating point "Number" type. In JavaScript, this type breaks down into a 32-bit integer when bitwise operators are used. One way to get a 64-bit hash is to use two 32-bit numbers in parallel, as Garbochess-JS 16 does. Another, which p4wn used at one stage, is to use 47 or 48-bit additive hashes. 64-bit floating point numbers are true integers up to 53 bits, so it is possible to sum at least 32 (and on average close to 64) random 48-bit numbers, which was enough for p4wn's purposes. For additive Zobrist hashing, you add the number when placing a piece and subtract it when removing it, rather than using xor both ways. There is no difference in accuracy or speed, and 48-bit hashes give you collisions at around the 2 ^ 24 or 16 million point.

## Linear Independence

The minimum and average Hamming Distance over all Zobrist keys was often considered as a "quality" measure of the keys. However, maximising the minimal hamming distance leads to very poor Zobrist keys. As long the minimum hamming distance is greater zero, [linear independence](https://en.wikipedia.org/wiki/Linear_independence) (that is a small subset of all keys doesn't xor to zero), is much more important than hamming distance as explained by Sven Reichard 17 :

Assume we associate a bitstring to every piece-square combination. That is what's usually done in chess programs; some codes are added for the side to move, castling rights, e.p. squares, etc. We obtain the code of a position by XOR-ing the codes of all the pieces contained in it.

What we want to avoid is collisions at nodes close to the root. For nodes close to the leaves, the cost of recomputing the score is smaller. Hence, we want to avoid that:

```
x1^x2^...^xm = y1^y2^...^yn
for codes xi, yi and small numbers m and n, and xi not equal to yj
```

To translate that to a language that is more familiar - at least for people of a mathematical background - we consider the [field F2](https://en.wikipedia.org/wiki/Field_%28mathematics%29#Finite_fields) of two elements. The elements are 0 and 1, and we can add and multiply them as usual, with the additional rule that 1 + 1 = 0. This is really a field, just like the [real](https://en.wikipedia.org/wiki/Real_number) or [complex numbers](https://en.wikipedia.org/wiki/Complex_number), and we can do calculations as usual. Note that addition is just the exclusive or.

Now the codes or bitstrings become [vectors](https://en.wikipedia.org/wiki/Vector_%28mathematics_and_physics%29) over the field F2, and the bitwise exclusive or becomes componentwise addition, i.e., usual addition of vectors. All these vectors form the [vector space](https://en.wikipedia.org/wiki/Vector_space) F2^k, where k is the length of the vectors. Typically, k = 64.

So, what we want to avoid is an equation

```
x1 + x2 + ... + xm = y1 + y2 + ... + yn
```

or

```
x1 + x2 + ... + xm + y1 + y2 + ... + yn = 0
```

since in F2, subtraction is the same as addition. Remembering some [linear algebra](https://en.wikipedia.org/wiki/Linear_algebra), this just means that we want the set x1,...,xm,y1,...,yn to be linearly independent.

This leads to the following criterion for picking a set of hash codes: A set of vectors in F2^k is a good set of hash codes if each small subset of non-zero vectors is linearly independent. What is not clear here is the meaning of "small", but we want small to be as big as possible. In other words, we consider sets of size up to a certain size as small, and if we can make that size bigger, it is better, since this leads to unique codes deeper in the tree.

However, what is clear is that this quality criterion does not depend on the base of the vector space. I.e., if we have a good set and multiply each vector by an invertible matrix (in other words, if we rotate the vectors), the obtained set will be just as good, since the rotation does not change the linear independence. The Hamming distance, on the other hand, is highly dependent on the vector space base. Take, for example, the vectors (1,0) and (0,1) in F2^2; they have Hamming distance 2. If we multiply both of them by

```
(1 1)
(0 1)
```

We get (1,1) and (0,1), which have a Hamming distance of 1. Actually, we can change any distance to anything else (except for 0) by an appropriate matrix. Thus, we try to approximate something that is independent of the base (the quality of our hash codes) by something that depends on it (the Hamming distance). Simple logic tells you that this approximation has to be really bad. An example where it doesn't work: It has been said that the Hamming distance shouldn't be too small or too big. So, vectors at a distance that is half the length should be ok, right? Let the length be 8 (I don't want to type too many 0's and 1's), and consider the vectors

```
11110000
11001100
00111100
```

They all have weight 4, their pairwise distance is 4, and yet they add up to 0. Just by looking at Hamming distances, you have no chance of detecting that.

Summarising I can say that I see no connection between the quality of hash codes and their Hamming distance. Using a good RNG like the one provided in GNU's stdlib will yield good hash codes ( you can actually prove that), and so I will take the codes as they are supplied by rand() or random() without messing with them and thereby most likely make them worse.

# See also

- CPW-Engine_transposition
- BCH Hashing

# Publications

- Albert Zobrist (1970). A New Hashing Method with Application for Game Playing. Technical Report #88, Computer Science Department, The University of Wisconsin, Madison, WI, USA. Reprinted (1990) in ICCA Journal, Vol. 13, No. 2, [pdf](http://www.cs.wisc.edu/techreports/1970/TR88.pdf)
- J. Lawrence Carter, Mark N. Wegman (1977). [Universal classes of hash functions](http://dl.acm.org/citation.cfm?id=803400). [STOC '77](http://dl.acm.org/citation.cfm?id=800105)
- Robert Hyatt, Anthony Cozzie (2005). [The Effect of Hash Signature Collisions in a Chess Program](http://www.craftychess.com/hyatt/collisions.html). ICGA Journal, Vol. 28., No. 3
- Borko Bošković, Sašo Greiner, Janez Brest, Viljem Žumer (2005). [The Representation of Chess Game](http://ieeexplore.ieee.org/xpl/freeabs_all.jsp?arnumber=1491153). Proceedings of the 27th International Conference on Information Technology Interfaces
- Mihai Pătrașcu, Mikkel Thorup (2011). The Power of Simple Tabulation Hashing. [arXiv:1011.5200v2](http://arxiv.org/abs/1011.5200)

# Forum Posts

## 1982 ...

- [compact representation of chess positions](http://quux.org:70/Archives/usenet-a-news/NET.chess/82.01.07_duke.1593_net.chess.txt) by Tom Truscott, net.chess, January 7, 1982

## 1990 ...

- [Hash tables - Clash!!! What happens next?](https://groups.google.com/d/msg/rec.games.chess/h9Q2wik_kTg/Jq7rYE0vqtoJ) by Valavan Manohararajah, rgc, March 15, 1994

[Re: Hash tables - Clash!!! What happens next?](https://groups.google.com/d/msg/rec.games.chess/h9Q2wik_kTg/9zrP0flwuzAJ) by Jonathan Schaeffer, March 17, 1994

- [Collision probability](https://groups.google.com/d/msg/rec.games.chess.computer/C53jRusegwA/XzgyZLbzcn8J) by Dennis Breuker, rgcc, April 15, 1996
- [Re: Berliner vs. Botvinnik Some interesting points](https://groups.google.com/d/msg/rec.games.chess.computer/JZ-_0rObjuQ/Mfwy9qBLxoAJ) by Bradley C. Kuszmaul, rgcc, November 6, 1996
- [Re: Hashing function for board positions](https://groups.google.com/d/msg/rec.games.chess.computer/oKgv-7WbfO0/TH-p0KUIo2kJ)by Jim Gillogly, rgcc, May 12, 1997
- [Fast hash algorithm](https://www.stmintz.com/ccc/index.php?id=13810) by John Scalo, CCC, January 08, 1998
- [Fast hash key method - Revisited!](https://www.stmintz.com/ccc/index.php?id=14053) by John Scalo, CCC, January 14, 1998
- [How to create a set of random integers for hashing?](https://www.stmintz.com/ccc/index.php?id=29817) by Ed Schröder, CCC, October 18, 1998

## 2000 ...

- [Why Random Number Needed In HashFunction[piece](https://groups.google.com/group/rec.games.chess.computer/browse_frm/thread/587d039679461fb8)[position]] by Cheok Yan Cheng, rgcc, June 12, 2001
- [About random numbers and hashing](https://www.stmintz.com/ccc/index.php?id=200366) by Severi Salminen, CCC, December 04, 2001
- [Random keys and hamming distance](https://www.stmintz.com/ccc/index.php?id=245775) by James Swafford, CCC, August 16, 2002
- [Hamming distance and lower hash table indexing](https://www.stmintz.com/ccc/index.php?id=313807) by Tom Likens, CCC, September 02, 2003
- [64-Bit random numbers](https://www.stmintz.com/ccc/index.php?id=324223) by Martin Schreiber, CCC, October 28, 2003
- [Is it necessary to include empty fields in the hash key of a position?](https://groups.google.com/group/rec.games.chess.computer/browse_frm/thread/42c6f293450dba50/) by Frank Hablizel, rgcc, December 25, 2003
- [Hashkey collisions (typical numbers)](https://www.stmintz.com/ccc/index.php?id=358836) by Renze Steenhuisen, CCC, April 07, 2004

## 2005 ...

- [Zobrist key random numbers](http://www.talkchess.com/forum/viewtopic.php?t=26152) by Robert Hyatt, CCC, January 21, 2009
- [Incremental Zobrist - slow?](http://www.talkchess.com/forum/viewtopic.php?t=28523) by Vlad Stamate, CCC, June 20, 2009 » Incremental Updates
- [On Zobrist keys](http://talkchess.com/forum/viewtopic.php?t=28545) by Lasse Hansen, CCC, June 21, 2009
- [Overlapped Zobrist keys array](http://www.talkchess.com/forum/viewtopic.php?t=30008) by Stefano Gemma, CCC, October 06, 2009

## 2010 ...

- [Transposition table random numbers](http://www.talkchess.com/forum/viewtopic.php?t=35415) by Justin Madru, CCC, July 13, 2010
- [TT Key Collisions, Workarounds?](http://www.talkchess.com/forum/viewtopic.php?t=40062) by Clemens Pruell, CCC, August 16, 2011
- [Key collision handling](http://www.talkchess.com/forum/viewtopic.php?t=40849) by Jonatan Pettersson, CCC, October 21, 2011
- [Using a Transposition Table with Zobrist Keys](http://www.open-chess.org/viewtopic.php?f=5&t=1872) by Miyagi403, OpenChess Forum, February 21, 2012
- [MT or KISS ?](http://www.talkchess.com/forum/viewtopic.php?t=43910) by Dan Honeycutt, CCC, June 02, 2012 18 19 20
- [Zobrist alternative?](http://www.talkchess.com/forum/viewtopic.php?t=44043) by Harm Geert Muller, CCC, June 12, 2012
- [Zobrist Number Statistics and WHat to Look For](http://www.talkchess.com/forum/viewtopic.php?t=45605) by Andrew Templeton, CCC, October 16, 2012
- [Question about Zobrist code](http://www.open-chess.org/viewtopic.php?f=5&t=2178) by Hamfer, OpenChess Forum, December 19, 2012

## 2015 ...

- [Zobrist keys - measure of quality?](http://www.talkchess.com/forum/viewtopic.php?t=55449) by Martin Sedlak, CCC, February 24, 2015
- [On-the fly hash key generation?](http://www.talkchess.com/forum/viewtopic.php?t=58890) by Evert Glebbeek, CCC, January 12, 2016

[Re: On-the fly hash key generation?](http://www.talkchess.com/forum/viewtopic.php?t=58890&start=13) by Aleks Peshkov, CCC, January 13, 2016

- [Rotated hash](http://www.talkchess.com/forum/viewtopic.php?t=61411) by J. Wesley Cleveland, CCC, September 13, 2016
- [No Zobrist key](http://www.talkchess.com/forum/viewtopic.php?t=61533) by Henk van den Belt, CCC, September 26, 2016
- [Enpass + Castling for Zorbist hashes](http://www.talkchess.com/forum/viewtopic.php?t=62733) by Andrew Grant, CCC, January 06, 2017 » Castling Rights, En passant
- [Zobrist hashing for text](http://www.talkchess.com/forum/viewtopic.php?t=66372) by Alvaro Cardoso, CCC, January 20, 2018

## 2020 ...

- [Zobrist key independence](http://www.talkchess.com/forum3/viewtopic.php?f=7&t=73110) by Harm Geert Muller, CCC, February 17, 2020
- [Best practices for transposition tables](http://www.talkchess.com/forum3/viewtopic.php?f=7&t=76508) by Brian Adkins, CCC, February 06, 2021
- [Zobrist keys](https://talkchess.com/viewtopic.php?t=85156) by Ben Vining, CCC, June 07, 2025

# External Links

- [Zobrist hashing from Wikipedia](https://en.wikipedia.org/wiki/Zobrist_hashing)
- [Tabulation hashing from Wikipedia](https://en.wikipedia.org/wiki/Tabulation_hashing)
- [Zobrist keys](http://web.archive.org/web/20070810003508/www.seanet.com/%7Ebrucemo/topics/zobrist.htm) from Bruce Moreland's [Programming Topics](http://web.archive.org/web/20070811182741/www.seanet.com/%7Ebrucemo/topics/topics.htm)
- [Zobrist keys](https://mediocrechess.blogspot.com/2007/01/guide-zobrist-keys.html) from [Mediocre Chess](https://mediocrechess.blogspot.com/) by Jonatan Pettersson
- [Gödel numbering from Wikipedia](https://en.wikipedia.org/wiki/G%C3%B6del_numbering)
- John Cage - [Music of Changes](https://en.wikipedia.org/wiki/Music_of_Changes), Book 1 (1951), performed by [Vicky Chow](https://www.facebook.com/vickychowpianist), [DiMenna Center](https://www.facebook.com/DiMennaCenter), [NYC](https://en.wikipedia.org/wiki/New_York_City), June 09, 2012, [YouTube](https://en.wikipedia.org/wiki/YouTube) Video

[Watch on YouTube](https://www.youtube.com/watch?v=Y7LD1iTl-lM)

# References

Up one Level    Albert Zobrist (1970). A New Hashing Method with Application for Game Playing. Technical Report #88, Computer Science Department, The University of Wisconsin, Madison, WI, USA. Reprinted (1990) in ICCA Journal, Vol. 13, No. 2, [pdf](http://www.cs.wisc.edu/techreports/1970/TR88.pdf)↩︎ [compact representation of chess positions](http://quux.org:70/Archives/usenet-a-news/NET.chess/82.01.07_duke.1593_net.chess.txt) by Tom Truscott, net.chess, January 7, 1982↩︎ [Re: Hashing function for board positions](https://groups.google.com/d/msg/rec.games.chess.computer/oKgv-7WbfO0/TH-p0KUIo2kJ)by Jim Gillogly, rgcc, May 12, 1997↩︎ [Re: Zobrist keys - measure of quality?](http://www.talkchess.com/forum/viewtopic.php?t=55449&start=4) by Rein Halbersma, CCC, February 24, 2015↩︎ J. Lawrence Carter, Mark N. Wegman (1977). [Universal classes of hash functions](http://dl.acm.org/citation.cfm?id=803400). [STOC '77](http://dl.acm.org/citation.cfm?id=800105)↩︎ Mihai Pătrașcu, Mikkel Thorup (2011). The Power of Simple Tabulation Hashing. [arXiv:1011.5200v2](http://arxiv.org/abs/1011.5200)↩︎ [Tabulation hashing from Wikipedia](https://en.wikipedia.org/wiki/Tabulation_hashing)↩︎ [Picture gallery "Recognition and Success 1955 - 1972"](http://www.mcescher.com/Gallery/gallery-recogn.htm) from [The Official M.C. Escher Website](http://www.mcescher.com/)↩︎ [RANDOM.ORG - Integer Generator](https://www.random.org/integers/?mode=advanced)↩︎ [The Marsaglia Random Number CDROM including the Diehard Battery of Tests](http://www.stat.fsu.edu/pub/diehard/) by George Marsaglia↩︎ [Re: Zobrist key random numbers](http://www.talkchess.com/forum/viewtopic.php?topic_view=threads&p=245932&t=26152) by Zach Wegner, CCC, January 22, 2009↩︎ [Overlapped Zobrist keys array](http://www.talkchess.com/forum/viewtopic.php?t=30008) by Stefano Gemma, CCC, October 06, 2009↩︎ [Hashkey collisions (typical numbers)](https://www.stmintz.com/ccc/index.php?id=358836) by Renze Steenhuisen, CCC, April 07, 2004↩︎ Robert Hyatt, Anthony Cozzie (2005). [The Effect of Hash Signature Collisions in a Chess Program](http://www.craftychess.com/hyatt/collisions.html). ICGA Journal, Vol. 28, No. 3↩︎ [Re: Hash tables - Clash!!! What happens next?](https://groups.google.com/d/msg/rec.games.chess/h9Q2wik_kTg/9zrP0flwuzAJ) by Jonathan Schaeffer, March 17, 1994↩︎ [Garbochess-JS](http://forwardcoding.com/projects/ajaxchess/chess.html)↩︎ [Re: About random numbers and hashing](https://www.stmintz.com/ccc/index.php?id=200622) by Sven Reichard, CCC, December 05, 2001↩︎ [Mersenne twister from Wikipedia](https://en.wikipedia.org/wiki/Mersenne_twister)↩︎ [64-bit KISS RNGs](http://compgroups.net/comp.lang.fortran/64-bit-kiss-rngs/601519) by George Marsaglia, [comp.lang.fortran | Computer Group](http://compgroups.net/comp.lang.fortran/), February 28, 2009↩︎ RKISS by Bob Jenkins↩︎
