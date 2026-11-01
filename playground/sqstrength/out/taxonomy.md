## square-hard-tc1
verdict: square-passgates: FAILED opp=hard tc=1 wd=0.500 threshold=0.600 n=60 lb=0.373 ub=0.627 cap=true
seat Red: n=30 W17 D4 L9 wd=0.700
seat Blue: n=30 W4 D5 L21 wd=0.300
loss taxonomy over 30 losses: eval crash no mate x1, one-ply mate reversal x3, slow bleed x26
losses as Red: n=9 one-ply=2 first-self-mate=0 flip median=3 early-flips=8 ponder-at-flip=1 kill-tags={'VCF': 9}
losses as Blue: n=21 one-ply=1 first-self-mate=0 flip median=6 early-flips=20 ponder-at-flip=2 kill-tags={'VCF': 21}
openings as Red: H8 H5 J6 J8 x8, H8 H5 K6 F6 x7, H8 H5 F6 F8 x4, H8 K8 I6 K6 x3, H8 H5 K6 E6 x2, H8 H5 F7 F5 x2
openings as Blue: H6 I6 I7 J5 x16, H6 G6 G7 F5 x6, H6 G6 G7 E7 x1, G7 G8 G9 G6 x1, H6 G6 G7 G8 x1, H6 F6 F5 D5 x1
master first-3-move scores as Red result W: mean=3 min=-1000
master first-3-move scores as Red result L: mean=-177 min=-1000
master first-3-move scores as Blue result W: mean=225 min=-1500
master first-3-move scores as Blue result L: mean=11 min=-1500
search parity as Red: master d=8.091 n=15489018 opp d=7.884 n=12525863
search parity as Blue: master d=7.908 n=17127025 opp d=7.223 n=9523707
master book moves total=35 of which as blue=0
master ponder adopt 542/1100
master phase shares as Red: early: ahead 0.547 mating 8 mated 3 over 300; mid: ahead 0.814 mating 36 mated 2 over 274; late: ahead 0.544 mating 59 mated 42 over 778
master phase shares as Blue: early: ahead 0.160 mating 2 mated 5 over 300; mid: ahead 0.029 mating 0 mated 63 over 273; late: ahead 0.382 mating 15 mated 52 over 827
master move timing: overruns=0 early-stops=958
master solver mate claims=79 collapsed-within-2=2 unconverted-in-losses=7
opp solver mate claims=142 collapsed-within-2=1
master as Blue line W-D-L: H6 I6 I7 J5 2-2-12, H6 G6 G7 F5 1-2-3, H6 G6 G7 E7 0-0-1, G7 G8 G9 G6 0-0-1, H6 G6 G7 G8 0-0-1, H6 F6 F5 D5 1-0-0, H6 I6 I7 J7 0-0-1, H6 I6 I7 I8 0-0-1
master as Red line W-D-L: H8 H5 J6 J8 4-1-3, H8 H5 K6 F6 4-1-2, H8 H5 F6 F8 2-1-1, H8 K8 I6 K6 3-0-0, H8 H5 K6 E6 1-0-1, H8 H5 F7 F5 1-0-1, H8 H11 F9 F11 0-1-1, H8 E8 F6 H6 1-0-0
opp as Blue line W-D-L: H6 I6 I7 J5 3-1-4, H6 I6 G6 G7 2-1-4, H6 G6 G7 F5 1-1-2, I7 I8 J7 J6 0-0-2, G7 H7 G6 F8 1-0-1, H6 I6 G6 F6 0-0-1, G9 H9 G10 F8 0-1-0, F8 F7 G7 E6 0-0-1
opp as Red line W-D-L: H8 H5 J6 J8 14-2-2, H8 H5 F6 F8 4-2-1, H8 H5 F6 G5 1-0-0, H8 E8 G5 G11 1-0-0, H8 G4 E6 F7 0-0-1, H8 H5 F6 J6 0-1-0, H8 H5 K6 F6 1-0-0
move tags both engines: {'BOOK': 35, 'PONDER': 1066, 'VCT': 50, 'VCF': 171}
ponder master: 542/1100 adopt, adopted-move own-score delta mean=-356 drops<=-500 124/458
ponder hard: 608/1099 adopt, adopted-move own-score delta mean=-224 drops<=-500 158/514

## square-hard-tc2
verdict: square-passgates: FAILED opp=hard tc=2 wd=0.350 threshold=0.600 n=40 lb=0.202 ub=0.498 cap=false
seat Red: n=20 W7 D1 L12 wd=0.400
seat Blue: n=20 W4 D2 L14 wd=0.300
loss taxonomy over 26 losses: eval crash no mate x1, one-ply mate reversal x4, slow bleed x20, was-mating then collapsed x1
losses as Red: n=12 one-ply=2 first-self-mate=0 flip median=3.0 early-flips=8 ponder-at-flip=1 kill-tags={'VCF': 12}
losses as Blue: n=14 one-ply=2 first-self-mate=0 flip median=6.0 early-flips=14 ponder-at-flip=0 kill-tags={'VCF': 14}
openings as Red: H8 H5 F6 F8 x9, H8 H5 J6 J8 x7, H8 E8 F6 H6 x1, H8 K8 I6 K6 x1, H8 H5 K6 F6 x1, H8 H11 J9 J11 x1
openings as Blue: H6 I6 I7 J5 x9, H6 G6 G7 F5 x7, F8 F7 G7 E6 x2, I7 I8 J7 K7 x1, H6 H5 G4 F4 x1
master first-3-move scores as Red result W: mean=142 min=-1000
master first-3-move scores as Red result L: mean=130 min=-1000
master first-3-move scores as Blue result W: mean=-8 min=-1500
master first-3-move scores as Blue result L: mean=-133 min=-1500
search parity as Red: master d=8.214 n=27174155 opp d=8.006 n=23219794
search parity as Blue: master d=8.375 n=31595508 opp d=8.274 n=17747317
master book moves total=22 of which as blue=0
master ponder adopt 279/574
master phase shares as Red: early: ahead 0.735 mating 10 mated 0 over 200; mid: ahead 0.855 mating 41 mated 2 over 172; late: ahead 0.358 mating 24 mated 31 over 330
master phase shares as Blue: early: ahead 0.090 mating 1 mated 0 over 200; mid: ahead 0.082 mating 5 mated 33 over 194; late: ahead 0.525 mating 30 mated 36 over 343
master move timing: overruns=0 early-stops=478
master solver mate claims=63 collapsed-within-2=4 unconverted-in-losses=10
opp solver mate claims=109 collapsed-within-2=1
master as Blue line W-D-L: H6 I6 I7 J5 1-2-6, H6 G6 G7 F5 2-0-5, F8 F7 G7 E6 0-0-2, I7 I8 J7 K7 0-0-1, H6 H5 G4 F4 1-0-0
master as Red line W-D-L: H8 H5 F6 F8 2-1-6, H8 H5 J6 J8 4-0-3, H8 E8 F6 H6 1-0-0, H8 K8 I6 K6 0-0-1, H8 H5 K6 F6 0-0-1, H8 H11 J9 J11 0-0-1
opp as Blue line W-D-L: H6 I6 I7 J5 3-0-4, H6 G6 G7 F5 4-1-1, H6 G6 G7 F7 2-0-0, F8 F7 G7 E6 0-0-1, H6 G6 G7 G8 0-0-1, I7 I8 J7 H6 1-0-0, H6 I6 G6 G7 1-0-0, I9 H9 I10 J8 1-0-0
opp as Red line W-D-L: H8 H5 J6 J8 6-2-1, H8 H5 F6 F8 5-0-2, H8 E8 F6 H6 2-0-0, H8 K8 I6 K6 1-0-0, H8 F5 H3 F3 0-0-1
move tags both engines: {'BOOK': 22, 'PONDER': 571, 'VCF': 130, 'VCT': 42}
ponder master: 279/574 adopt, adopted-move own-score delta mean=-179 drops<=-500 71/220
ponder hard: 356/626 adopt, adopted-move own-score delta mean=-221 drops<=-500 75/274

## square-medium-tc1
verdict: square-passgates: FAILED opp=medium tc=1 wd=0.450 threshold=0.750 n=20 lb=0.232 ub=0.668 cap=false
seat Red: n=10 W7 D0 L3 wd=0.700
seat Blue: n=10 W1 D1 L8 wd=0.200
loss taxonomy over 11 losses: eval crash no mate x1, one-ply mate reversal x2, slow bleed x8
losses as Red: n=3 one-ply=1 first-self-mate=0 flip median=3 early-flips=3 ponder-at-flip=0 kill-tags={'VCF': 3}
losses as Blue: n=8 one-ply=1 first-self-mate=0 flip median=6.0 early-flips=6 ponder-at-flip=1 kill-tags={'VCF': 8}
openings as Red: H8 H5 F6 F8 x4, H8 H5 K6 F6 x2, H8 H5 J6 F6 x1, H8 H5 J6 J8 x1, H8 H5 J8 I9 x1, H8 H5 K6 E6 x1
openings as Blue: H6 G6 G9 G7 x2, H6 I6 I7 J5 x2, H6 I6 G6 G7 x1, I7 H7 I6 J8 x1, H6 G6 I6 J6 x1, H6 G6 G7 F5 x1
master first-3-move scores as Red result W: mean=38 min=-1000
master first-3-move scores as Red result L: mean=-255 min=-1000
master first-3-move scores as Blue result W: mean=1466 min=300
master first-3-move scores as Blue result L: mean=512 min=-1500
search parity as Red: master d=8.370 n=22585016 opp d=8.335 n=10780937
search parity as Blue: master d=8.376 n=20020073 opp d=7.511 n=8244377
master book moves total=10 of which as blue=0
master ponder adopt 158/281
master phase shares as Red: early: ahead 0.440 mating 3 mated 2 over 100; mid: ahead 0.649 mating 23 mated 6 over 77; late: ahead 0.733 mating 15 mated 7 over 60
master phase shares as Blue: early: ahead 0.390 mating 2 mated 5 over 100; mid: ahead 0.212 mating 0 mated 28 over 85; late: ahead 0.423 mating 4 mated 12 over 194
master move timing: overruns=0 early-stops=178
master solver mate claims=32 collapsed-within-2=2 unconverted-in-losses=3
opp solver mate claims=32 collapsed-within-2=0
move tags both engines: {'BOOK': 10, 'PONDER': 277, 'VCF': 57, 'VCT': 7}
ponder master: 158/281 adopt, adopted-move own-score delta mean=-319 drops<=-500 29/113
ponder medium: 149/292 adopt, adopted-move own-score delta mean=-335 drops<=-500 43/116

## square-medium-tc2
verdict: square-passgates: FAILED opp=medium tc=2 wd=0.617 threshold=0.750 n=60 lb=0.494 ub=0.740 cap=false
seat Red: n=30 W20 D4 L6 wd=0.800
seat Blue: n=30 W10 D3 L17 wd=0.433
loss taxonomy over 23 losses: eval crash no mate x1, slow bleed x20, was-mating then collapsed x2
losses as Red: n=6 one-ply=0 first-self-mate=0 flip median=19.0 early-flips=3 ponder-at-flip=2 kill-tags={'VCF': 6}
losses as Blue: n=17 one-ply=0 first-self-mate=0 flip median=6 early-flips=13 ponder-at-flip=1 kill-tags={'VCF': 17}
openings as Red: H8 H5 J6 J8 x14, H8 H5 F6 F8 x13, H8 H5 K6 E6 x1, H8 H5 K6 F6 x1, H8 H11 J9 J11 x1
openings as Blue: H6 G6 G7 F5 x9, H6 I6 G6 G7 x6, H6 I6 I7 J5 x5, H6 I6 I7 K7 x3, H6 G6 I6 I7 x2, H6 I6 G6 F6 x1
master first-3-move scores as Red result W: mean=73 min=-1000
master first-3-move scores as Red result L: mean=188 min=-1000
master first-3-move scores as Blue result W: mean=466 min=-1600
master first-3-move scores as Blue result L: mean=207 min=-3000
search parity as Red: master d=8.563 n=24564254 opp d=8.300 n=13477630
search parity as Blue: master d=8.413 n=27092490 opp d=7.959 n=12517206
master book moves total=31 of which as blue=0
master ponder adopt 560/1062
master phase shares as Red: early: ahead 0.740 mating 6 mated 0 over 300; mid: ahead 0.890 mating 75 mated 2 over 290; late: ahead 0.386 mating 61 mated 40 over 684
master phase shares as Blue: early: ahead 0.373 mating 16 mated 1 over 300; mid: ahead 0.256 mating 12 mated 48 over 281; late: ahead 0.401 mating 41 mated 35 over 733
master move timing: overruns=0 early-stops=949
master solver mate claims=164 collapsed-within-2=1 unconverted-in-losses=16
opp solver mate claims=92 collapsed-within-2=0
move tags both engines: {'BOOK': 31, 'PONDER': 1019, 'VCF': 221, 'VCT': 35}
ponder master: 560/1062 adopt, adopted-move own-score delta mean=-194 drops<=-500 130/446
ponder medium: 564/1129 adopt, adopted-move own-score delta mean=-293 drops<=-500 156/480

## square-easy-tc1
verdict: square-passgates: DONE
seat Red: n=30 W28 D0 L2 wd=0.933
seat Blue: n=30 W26 D1 L3 wd=0.900
loss taxonomy over 5 losses: one-ply mate reversal x3, slow bleed x2
losses as Red: n=2 one-ply=0 first-self-mate=0 flip median=4.0 early-flips=2 ponder-at-flip=1 kill-tags={}
losses as Blue: n=3 one-ply=3 first-self-mate=0 flip median=4 early-flips=3 ponder-at-flip=3 kill-tags={}
openings as Red: H8 H5 J6 J8 x26, H8 H5 K6 F6 x3, H8 H5 K6 E6 x1
openings as Blue: H6 G6 G7 E7 x8, H6 I6 G7 G8 x6, H6 I6 I9 J6 x4, I9 I8 I7 I6 x2, I7 H7 G7 J7 x2, I7 H7 G7 F7 x1
master first-3-move scores as Red result W: mean=119 min=-1000
master first-3-move scores as Red result L: mean=-550 min=-1000
master first-3-move scores as Blue result W: mean=589 min=-1500
master first-3-move scores as Blue result L: mean=633 min=-900
search parity as Red: master d=7.472 n=20235338 opp d=6.650 n=5488560
search parity as Blue: master d=8.242 n=20700691 opp d=7.031 n=4937182
master book moves total=30 of which as blue=0
master ponder adopt 395/584
master phase shares as Red: early: ahead 0.703 mating 2 mated 0 over 300; mid: ahead 0.900 mating 186 mated 2 over 239; late: ahead 0.848 mating 8 mated 2 over 46
master phase shares as Blue: early: ahead 0.520 mating 19 mated 12 over 300; mid: ahead 0.660 mating 107 mated 2 over 238; late: ahead 0.623 mating 58 mated 1 over 244
master move timing: overruns=0 early-stops=417
master solver mate claims=201 collapsed-within-2=3 unconverted-in-losses=3
opp solver mate claims=0 collapsed-within-2=0
move tags both engines: {'BOOK': 30, 'PONDER': 309, 'VCT': 41, 'VCF': 160}
ponder master: 395/584 adopt, adopted-move own-score delta mean=657 drops<=-500 62/258

## square-easy-tc2
verdict: square-passgates: FAILED opp=easy tc=2 wd=0.850 threshold=0.900 n=60 lb=0.760 ub=0.940 cap=true
seat Red: n=30 W21 D6 L3 wd=0.900
seat Blue: n=30 W24 D0 L6 wd=0.800
loss taxonomy over 9 losses: eval crash no mate x1, one-ply mate reversal x4, slow bleed x4
losses as Red: n=3 one-ply=0 first-self-mate=0 flip median=5 early-flips=2 ponder-at-flip=2 kill-tags={}
losses as Blue: n=6 one-ply=4 first-self-mate=0 flip median=10.0 early-flips=6 ponder-at-flip=5 kill-tags={}
openings as Red: H8 H5 J6 J8 x29, H8 H5 K6 F6 x1
openings as Blue: H6 G6 G7 E7 x10, H6 I6 I9 J6 x7, H6 I6 G7 F5 x3, F8 F7 F9 F6 x2, G7 G8 G9 G10 x2, H6 I6 G7 G8 x1
master first-3-move scores as Red result W: mean=225 min=-1000
master first-3-move scores as Red result L: mean=-22 min=-1000
master first-3-move scores as Blue result W: mean=465 min=-1500
master first-3-move scores as Blue result L: mean=555 min=-1500
search parity as Red: master d=9.371 n=26318916 opp d=7.469 n=6162566
search parity as Blue: master d=7.766 n=35221487 opp d=6.520 n=8204539
master book moves total=30 of which as blue=0
master ponder adopt 577/808
master phase shares as Red: early: ahead 0.777 mating 1 mated 0 over 300; mid: ahead 0.989 mating 65 mated 0 over 277; late: ahead 0.589 mating 70 mated 12 over 922
master phase shares as Blue: early: ahead 0.480 mating 29 mated 16 over 300; mid: ahead 0.502 mating 77 mated 2 over 227; late: ahead 0.803 mating 62 mated 6 over 229
master move timing: overruns=0 early-stops=734
master solver mate claims=186 collapsed-within-2=4 unconverted-in-losses=6
opp solver mate claims=0 collapsed-within-2=0
move tags both engines: {'BOOK': 30, 'PONDER': 522, 'VCT': 49, 'VCF': 137}
ponder master: 577/808 adopt, adopted-move own-score delta mean=-20 drops<=-500 125/473

## hex-hard-tc1
verdict: hex-passgates: DONE
seat Red: n=20 W5 D13 L2 wd=0.900
seat Blue: n=20 W3 D10 L7 wd=0.650
loss taxonomy over 9 losses: one-ply mate reversal x1, slow bleed x8
losses as Red: n=2 one-ply=0 first-self-mate=0 flip median=1.0 early-flips=2 ponder-at-flip=0 kill-tags={'VCF': 2}
losses as Blue: n=7 one-ply=1 first-self-mate=0 flip median=60 early-flips=0 ponder-at-flip=1 kill-tags={'VCF': 7}
openings as Red: L12 K9 J12 K13 x4, L12 M8 N10 M12 x3, L12 K9 N8 H14 x2, L12 M8 J10 P10 x1, L12 P9 N12 L13 x1, L12 H13 J15 J9 x1
openings as Blue: L10 K11 J12 M9 x4, L10 M10 M11 O12 x3, L10 K11 L9 N6 x3, L10 M10 L9 J8 x2, L10 K11 J12 N10 x2, L10 M10 N10 O8 x1
master first-3-move scores as Red result W: mean=-940 min=-1000
master first-3-move scores as Red result L: mean=-1083 min=-1500
master first-3-move scores as Blue result W: mean=1222 min=100
master first-3-move scores as Blue result L: mean=1347 min=100
search parity as Red: master d=5.903 n=10437060 opp d=5.917 n=7484988
search parity as Blue: master d=5.617 n=11683590 opp d=5.759 n=7866055
master book moves total=0 of which as blue=0
master ponder adopt 870/1961
master phase shares as Red: early: ahead 0.030 mating 0 mated 0 over 200; mid: ahead 0.207 mating 0 mated 3 over 198; late: ahead 0.454 mating 16 mated 1 over 2599
master phase shares as Blue: early: ahead 0.960 mating 0 mated 0 over 200; mid: ahead 0.770 mating 2 mated 0 over 200; late: ahead 0.414 mating 13 mated 15 over 2203
master move timing: overruns=0 early-stops=2296
master solver mate claims=26 collapsed-within-2=0 unconverted-in-losses=0
opp solver mate claims=36 collapsed-within-2=1
master as Blue line W-D-L: L10 K11 J12 M9 1-2-1, L10 M10 M11 O12 0-1-2, L10 K11 L9 N6 2-0-1, L10 M10 L9 J8 0-1-1, L10 K11 J12 N10 0-2-0, L10 M10 N10 O8 0-1-0, J12 K11 L11 O9 0-1-0, L10 M9 L8 M8 0-0-1
master as Red line W-D-L: L12 K9 J12 K13 0-4-0, L12 M8 N10 M12 1-2-0, L12 K9 N8 H14 1-1-0, L12 M8 J10 P10 0-1-0, L12 P9 N12 L13 0-1-0, L12 H13 J15 J9 1-0-0, L12 H13 J10 J15 0-1-0, L12 I11 L10 M11 1-0-0
opp as Blue line W-D-L: L10 K11 K12 I15 0-4-0, L10 M10 M11 O12 0-1-1, L10 K11 J12 M9 0-1-1, L10 M10 N10 K10 0-1-0, N10 N11 M12 L15 0-1-0, J12 J13 J11 J14 0-0-1, J12 J13 J14 L10 0-1-0, J12 K11 L11 O9 0-0-1
opp as Red line W-D-L: L12 K9 N8 H14 1-2-1, L12 M8 N10 M12 2-1-0, L12 K9 M9 L8 1-0-2, L12 M8 J10 O10 0-2-0, L12 M8 K10 L8 1-1-0, L12 K9 N8 I13 0-2-0, L12 I11 L10 M11 0-1-0, L12 K9 N8 L9 1-0-0
move tags both engines: {'PONDER': 1775, 'VCT': 10, 'VCF': 52}
ponder master: 870/1961 adopt, adopted-move own-score delta mean=34 drops<=-500 170/859
ponder hard: 924/2052 adopt, adopted-move own-score delta mean=-110 drops<=-500 196/912

## hex-hard-tc2
verdict: hex-passgates: DONE
seat Red: n=10 W2 D8 L0 wd=1.000
seat Blue: n=10 W1 D6 L3 wd=0.700
loss taxonomy over 3 losses: eval crash no mate x1, slow bleed x2
losses as Blue: n=3 one-ply=0 first-self-mate=0 flip median=48 early-flips=0 ponder-at-flip=0 kill-tags={'VCF': 3}
openings as Red: L12 L9 I10 N10 x1, L12 H13 J10 J16 x1, L12 O8 K10 Q10 x1, L12 M8 J10 P10 x1, L12 K9 J12 K13 x1, L12 M8 K10 L8 x1
openings as Blue: L10 K11 J12 N10 x1, N10 N11 M12 L15 x1, L10 K11 K12 I15 x1, L10 K11 J12 M9 x1, L10 M9 L8 M8 x1, N10 N11 O9 P6 x1
master first-3-move scores as Red result W: mean=-1016 min=-1500
master first-3-move scores as Blue result W: mean=2600 min=1000
master first-3-move scores as Blue result L: mean=1288 min=1000
search parity as Red: master d=6.844 n=17755171 opp d=6.914 n=13294283
search parity as Blue: master d=6.064 n=16805402 opp d=6.041 n=12042273
master book moves total=0 of which as blue=0
master ponder adopt 500/1059
master phase shares as Red: early: ahead 0.000 mating 0 mated 0 over 100; mid: ahead 0.430 mating 1 mated 0 over 100; late: ahead 0.479 mating 9 mated 0 over 1482
master phase shares as Blue: early: ahead 0.990 mating 0 mated 0 over 100; mid: ahead 0.690 mating 0 mated 0 over 100; late: ahead 0.382 mating 4 mated 3 over 1381
master move timing: overruns=0 early-stops=1391
master solver mate claims=11 collapsed-within-2=0 unconverted-in-losses=0
opp solver mate claims=10 collapsed-within-2=0
master as Blue line W-D-L: L10 K11 J12 N10 0-0-1, N10 N11 M12 L15 0-1-0, L10 K11 K12 I15 0-1-0, L10 K11 J12 M9 0-1-0, L10 M9 L8 M8 1-0-0, N10 N11 O9 P6 0-1-0, L10 K11 L9 N6 0-1-0, L10 K11 K12 J11 0-0-1
master as Red line W-D-L: L12 L9 I10 N10 0-1-0, L12 H13 J10 J16 0-1-0, L12 O8 K10 Q10 0-1-0, L12 M8 J10 P10 1-0-0, L12 K9 J12 K13 0-1-0, L12 M8 K10 L8 0-1-0, L12 K9 I13 O7 0-1-0, L12 K9 N8 H14 0-1-0
opp as Blue line W-D-L: L10 K10 M10 M11 0-1-0, J12 J13 J14 J11 0-1-0, N10 M10 O10 L10 0-1-0, L10 M10 N10 K10 0-0-1, L10 K11 K12 I15 0-1-0, L10 M10 L9 J8 0-1-0, L10 K11 M9 J12 0-1-0, L10 K11 J12 M9 0-1-0
opp as Red line W-D-L: L12 K9 N8 I13 2-0-0, L12 P9 N12 L13 0-1-0, L12 K9 J12 K13 0-1-0, L12 K9 N8 H14 0-1-0, L12 K9 N8 L9 0-0-1, L12 P9 N9 P8 0-1-0, L12 K9 M9 L8 0-1-0, L12 L9 J12 K10 1-0-0
move tags both engines: {'PONDER': 1012, 'VCT': 8, 'VCF': 13}
ponder master: 500/1059 adopt, adopted-move own-score delta mean=-55 drops<=-500 92/496
ponder hard: 521/1100 adopt, adopted-move own-score delta mean=-143 drops<=-500 118/512

