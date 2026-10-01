source: https://chessprogramming.org/SIMD_and_SWAR_Techniques

# SIMD and SWAR Techniques

Home * Programming * SIMD and SWAR Techniques

[SIMD](https://en.wikipedia.org/wiki/SIMD) 1

---

1. [Flynn's taxonomy from Wikipedia](https://en.wikipedia.org/wiki/Flynn%27s_taxonomy)↩︎

x86, x86-64, as well as PowerPC and [Power ISA v.2.03](https://en.wikipedia.org/wiki/Power_Architecture#Power_ISA_v.2.03) processors provide Single Instructions on Multiple Data (SIMD), namely on vectors of floats, doubles or various integers, bytes, words, double words or quad words, available through assembly and compiler intrinsics. SIMD-applications related to computer chess cover bitboard computations and fill-algorithms like Dumb7Fill and Kogge-Stone Algorithm, as well as evaluation related stuff, like this SSE2 dot-product of 64 bits by a vector of 64 bytes.

SWAR as acronym for SIMD Within A Register was coined by Hank Dietz and Randell J. Fisher 1 . It is a processing model which applies SIMD parallel processing across sections of a CPU register, often vectors of smaller than byte-entities are processed in parallel prefix manner.

# SIMD Instruction Sets

- MMX on x86 and x86-64
- SSE2, SSE3, SSSE3 and SSE4 on x86 and x86-64
- SSE5 by AMD (proposed but not implemented, replaced by XOP 2)
- AltiVec on PowerPC G4, PowerPC G5 resp. VMX since POWER6
- [VSX](https://en.wikipedia.org/wiki/AltiVec#VSX_(Vector_Scalar_Extension)) since POWER7
- Helium by ARM
- NEON by ARM
- SVE 3 and SVE2 4 by ARM
- AVX by Intel
- AVX2 by Intel
- AVX-512 by Intel
- XOP by AMD
- VIS 5 since SPARC v9
- RISC-V vector-set extension 6

# SWAR Arithmetic

To apply addition and subtraction on vectors of bit-aggregates or [bit-field structures](https://en.wikipedia.org/wiki/Bit_field) within a general purpose register, one has to take care carries and borrows don't wrap around. Thus the need to mask of all most significant bits (H) and add in two steps, one 'add' with MSB clear and one add modulo 2 aka 'xor' for the MSB itself. For bytewise (rankwise) math inside a 64-bit register, H is 0x8080808080808080 and L is 0x0101010101010101.

```
SWAR add z = x + y
    z = ((x &~H) + (y &~H)) ^ ((x ^ y) & H)

SWAR sub z = x - y
    z = ((x | H) - (y &~H)) ^ ((x ^~y) & H)

SWAR average z = (x+y)/2 based on x + y = (x^y) + 2*(x&y)
    z = (x & y) + (((x ^ y) & ~L) >> 1)
```

# Samples

Amazing, how similar these two SWAR- and parallel prefix wise routines are. Mirror horizontally and population count have in common to act on vectors of duos, nibbles and bytes. One swaps bits, duos and nibbles, while the second adds populations of them.

```
U64 mirrorHorizontal (U64 x) {
    const U64 k1 = C64(0x5555555555555555);
    const U64 k2 = C64(0x3333333333333333);
    const U64 k4 = C64(0x0f0f0f0f0f0f0f0f);
    x = ((x & k1) << 1) | ((x >> 1)  & k1);
    x = ((x & k2) << 2) | ((x >> 2)  & k2);
    x = ((x & k4) << 4) | ((x >> 4)  & k4);
    return x;
}

int popCount (U64 x) {
    const U64 k1 = C64(0x5555555555555555);
    const U64 k2 = C64(0x3333333333333333);
    const U64 k4 = C64(0x0f0f0f0f0f0f0f0f);
    x =   x             - ((x >> 1)  & k1);
    x =  (x & k2)       + ((x >> 2)  & k2);
    x = ( x             +  (x >> 4)) & k4 ;
    x = (x * C64(0x0101010101010101))>> 56;
    return (int) x;
}
```

# See also

- GPU
- NNUE
- Parallel Prefix Algorithms

# Publications

## 1987 ...

- Alan H. Bond (1987). Broadcasting Arrays - A Highly Parallel Computer Architecture Suitable For Easy Fabrication. [pdf](http://www.exso.com/bc.pdf)
- Guy E. Blelloch (1990). [Vector Models for Data-Parallel Computing](https://dl.acm.org/citation.cfm?id=91254). [MIT Press](https://en.wikipedia.org/wiki/MIT_Press), [pdf](https://www.cs.cmu.edu/~guyb/papers/Ble90.pdf)
- [Randell J. Fisher](https://dblp.uni-trier.de/pers/f/Fisher:Randall_J=.html), Hank Dietz (1998). [Compiling for SIMD Within a Register](https://link.springer.com/chapter/10.1007/3-540-48319-5_19). [LCPC 1998](https://dblp.uni-trier.de/db/conf/lcpc/lcpc1998.html), [pdf](https://link.springer.com/chapter/10.1007/3-540-48319-5_19)
- [Tom Thompson](https://www.linkedin.com/in/tom-thompson-500bb7b) (1999). [AltiVec Revealed](http://www.mactech.com/articles/mactech/Vol.15/15.07/AltiVecRevealed/index.html). [MacTech](http://www.mactech.com/), Vol. 15, No. 7

## 2000 ...

- [Randell J. Fisher](https://dblp.uni-trier.de/pers/f/Fisher:Randall_J=.html) (2003). [General-Purpose SIMD Within A Register: Parallel Processing on Consumer Microprocessors](https://docs.lib.purdue.edu/dissertations/AAI3108343/). Ph.D. thesis, [Purdue University](https://en.wikipedia.org/wiki/Purdue_University), advisor Hank Dietz, [pdf](http://aggregate.org/SWAR/Dis/dissertation.pdf)
- Daisuke Takahashi (2007). [An Implementation of Parallel 1-D FFT Using SSE3 Instructions on Dual-Core Processors](https://link.springer.com/chapter/10.1007/978-3-540-75755-9_135/). Proc. Workshop on State-of-the-Art in Scientific and Parallel Computing, [Lecture Notes in Computer Science](https://en.wikipedia.org/wiki/Lecture_Notes_in_Computer_Science), No. 4699, [Springer](https://en.wikipedia.org/wiki/Springer_Science%2BBusiness_Media)
- Daisuke Takahashi (2008). Implementation and Evaluation of Parallel FFT Using SIMD Instructions on Multi-Core Processors. Proc. 2007 International Workshop on Innovative Architecture for Future Generation High-Performance Processors and Systems
- [Nicolas Fritz](https://www.researchgate.net/profile/Nicolas_Fritz2) (2009). SIMD Code Generation in Data-Parallel Programming. Ph.D. thesis, [Saarland University](https://en.wikipedia.org/wiki/Saarland_University), [pdf](http://scidok.sulb.uni-saarland.de/volltexte/2009/2563/pdf/Dissertation_9229_Frit_Nico_2009.pdf?q=ibms-cell-processor)

## 2010 ...

- [Georg Hager](https://www.rrze.fau.de/wir-ueber-uns/organigramm/mitarbeiter/index.shtml/georg-hager.shtml) 7, [Jan Treibig](http://dblp.uni-trier.de/pers/hd/t/Treibig:Jan), [Gerhard Wellein](http://dblp.uni-trier.de/pers/hd/w/Wellein:Gerhard) (2013). The Practitioner's Cookbook for Good Parallel Performance on Multi- and Many-Core Systems. [RRZE](https://de.wikipedia.org/wiki/Regionales_Rechenzentrum_Erlangen), [SC13](http://sc13.supercomputing.org/), [slides as pdf](https://blogs.fau.de/hager/files/2013/11/sc13_tutorial_134.pdf)
- [Kaixi Hou](https://scholar.google.com/citations?user=4Ab_NBkAAAAJ&hl=en), Hao Wang, [Wu-chun Feng](http://dblp.uni-trier.de/pers/hd/f/Feng:Wu=chun) (2015). ASPaS: A Framework for Automatic SIMDIZation of Parallel Sorting on x86-based Many-core Processors. [ICS2015](http://dblp.uni-trier.de/db/conf/ics/ics2015.html#HouWF15),

# Manuals

## AMD

- [AMD64 Architecture Volume 4: 128-Bit and 256-Bit Media Instructions](http://developer.amd.com/wordpress/media/2012/10/26568_APM_v41.pdf) (pdf)
- [AMD64 Architecture Volume 5: 64-Bit Media and x87 Floating-Point Instructions](http://support.amd.com/TechDocs/26569_APM_v5.pdf) (pdf)
- [AMD64 Architecture Volume 6: 128-Bit and 256-Bit XOP, FMA4 and CVT16 Instructions](http://support.amd.com/TechDocs/43479.pdf) (pdf)

## NXP Semiconductors

- [AltiVec Technology - Programming Interface Manual](http://www.nxp.com/files/32bit/doc/ref_manual/ALTIVECPIM.pdf) (pdf) 8

## Intel

- [Intel 64 and IA32 Architectures Optimization Reference Manual](http://www.intel.com/design/processor/manuals/248966.pdf) (pdf)

# Forum Posts

## 1999

- [G4 & AltiVec](https://www.stmintz.com/ccc/index.php?id=71754) by Will Singleton, CCC, October 04, 1999 » AltiVec, PowerPC G4

## 2000 ...

- [Superlinear interpolator: a nice novelity ?](http://www.talkchess.com/forum/viewtopic.php?t=23860) by Marco Costalba, CCC, September 20, 2008 » Tapered Eval
- [Re: talk about IPP's evaluation](http://www.talkchess.com/forum/viewtopic.php?p=301746#301746) by Richard Vida, CCC, November 07, 2009 » Ippolit, Tapered Eval

## 2010 ...

- [My experience with Linux/GCC](http://www.talkchess.com/forum/viewtopic.php?t=38523) by Richard Vida, CCC, March 23, 2011 » C, Linux, Tapered Eval
- [Re: Utilizing Architecture Specific Functions from a HL Language](http://www.talkchess.com/forum/viewtopic.php?t=39916&start=1) by Wylie Garvin, CCC, July 31, 2011
- [two values in one integer](http://www.talkchess.com/forum/viewtopic.php?t=42054) by Pierre Bokma, CCC, January 18, 2012
- [Pigeon now using opportunistic SIMD](http://www.talkchess.com/forum/viewtopic.php?t=59820) by Stuart Riffle, CCC, April 11, 2016 » Pigeon
- [couple of questions about stockfish code ?](http://www.talkchess.com/forum/viewtopic.php?t=61850) by Mahmoud Uthman, CCC, October 26, 2016 » Stockfish, Tapered Eval

## 2020 ...

- [SIMD methods in TT probing and replacement](http://www.talkchess.com/forum3/viewtopic.php?f=7&t=73126) by Harm Geert Muller, CCC, February 20, 2020 » Transposition Table
- [CPU Vector Unit, the new jam for NNs...](http://www.talkchess.com/forum3/viewtopic.php?f=2&t=75862) by Srdja Matovic, CCC, November 18, 2020 » NNUE

# External Links

- [SIMD from Wikipedia](https://en.wikipedia.org/wiki/SIMD)
- [SWAR from Wikipedia](https://en.wikipedia.org/wiki/SWAR)
- [The Aggregate: SWAR, SIMD Within A Register](http://www.aggregate.org/SWAR/) by Hank Dietz

## x86/x86-64

- [MMX from Wikipedia](https://en.wikipedia.org/wiki/MMX_%28instruction_set%29)
- [3DNow! from Wikipedia](https://en.wikipedia.org/wiki/3DNow)
- [Streaming SIMD Extensions from Wikipedia](https://en.wikipedia.org/wiki/Streaming_SIMD_Extensions)
- [SSE2 from Wikipedia](https://en.wikipedia.org/wiki/SSE2)
- [SSE3 from Wikipedia](https://en.wikipedia.org/wiki/SSE3)
- [SSSE3 from Wikipedia](https://en.wikipedia.org/wiki/SSSE3)
- [SSE4 from Wikipedia](https://en.wikipedia.org/wiki/SSE4)
- [SSE4a from Wikipedia.de](https://de.wikipedia.org/wiki/SSE4a)
- [SSE5 from Wikipedia](https://en.wikipedia.org/wiki/SSE5)
- [XOP instruction set from Wikipedia](https://en.wikipedia.org/wiki/XOP_instruction_set)
- [Advanced Vector Extensions from Wikipedia](https://en.wikipedia.org/wiki/Advanced_Vector_Extensions)
- [AVX-512 from Wikipedia](https://en.wikipedia.org/wiki/AVX-512)
- [SSEPlus Project](http://developer.amd.com/cpu/Libraries/sseplus/Pages/default.aspx) from [AMD Developer Central](http://developer.amd.com/pages/default.aspx)
- [SSEPlus Project Documentation](http://sseplus.sourceforge.net/index.html)

## Other

- [SIMD ISAs | Neon – Arm Developer](https://developer.arm.com/architectures/instruction-sets/simd-isas/neon)
- [ARM NEON Technology from Wikipedia](https://en.wikipedia.org/wiki/ARM_architecture#Advanced_SIMD_.28NEON.29)
- [SIMD ISAs | Arm Helium technology – Arm Developer](https://developer.arm.com/architectures/instruction-sets/simd-isas/helium)
- [AltiVec from Wikipedia](https://en.wikipedia.org/wiki/AltiVec)

## Misc

- [Explicitly parallel instruction computing (EPIC) from Wikipedia](https://en.wikipedia.org/wiki/Explicitly_parallel_instruction_computing)
- [Instruction-level parallelism from Wikipedia](https://en.wikipedia.org/wiki/Instruction-level_parallelism)
- [MIMD from Wikipedia](https://en.wikipedia.org/wiki/MIMD)
- [Parallel Thread Execution from Wikipedia](https://en.wikipedia.org/wiki/Parallel_Thread_Execution) » GPU, Thread
- [SPMD from Wikipedia](https://en.wikipedia.org/wiki/SPMD)
- [Very long instruction word (VLIW) from Wikipedia](https://en.wikipedia.org/wiki/Very_long_instruction_word)

# References

Up one Level    [The Aggregate: SWAR, SIMD Within A Register](http://www.aggregate.org/SWAR/) by Hank Dietz↩︎ [SSE5 from Wikipedia](https://en.wikipedia.org/wiki/SSE5)↩︎ [SVE from Wikipedia](https://en.wikipedia.org/wiki/AArch64#Scalable_Vector_Extension_(SVE))↩︎ [SVE2 from Wikipedia](https://en.wikipedia.org/wiki/SVE)↩︎ [VIS from Wikipedia](https://en.wikipedia.org/wiki/Visual_Instruction_Set)↩︎ [RISC-V vector-set from Wikipedia](https://en.wikipedia.org/wiki/RISC-V#Vector_set)↩︎ [Georg Hager's Blog | Random thoughts on High Performance Computing](https://blogs.fau.de/hager/)↩︎ On December 7, 2015, [NXP Semiconductors](https://en.wikipedia.org/wiki/NXP_Semiconductors) completed its acquisition of Freescale, [Freescale from Wikipedia](https://en.wikipedia.org/wiki/Freescale_Semiconductor)↩︎
