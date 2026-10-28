# gateforensic report

generated from playground/gateforensic/out datasets, wald z=1.96,
phases early<=20 mid<=40,
win-sign consistency 1.000, provisional arms ['square-passgates-easy-tc2']

## per arm W+D with Wald 95 percent intervals

| arm | geometry | opp | tc_idx | n | wins | draws | losses | wd | lb | ub | verdict_status | verdict_wd | threshold | verdict_n | verdict_lb | verdict_ub | provisional |
|---|---|---|---|---|---|---|---|---|---|---|---|---|---|---|---|---|---|
| hex-passgates-easy-tc1 | hex | easy | 1 | 60 | 20 | 33 | 7 | 0.883 | 0.802 | 0.965 | FAILED | 0.883 | 0.9 | 60 | 0.802 | 0.965 | False |
| hex-passgates-easy-tc1.attempt1 | hex | easy | 1 | 20 | 6 | 11 | 3 | 0.85 | 0.694 | 1 | FAILED | 0.85 | 0.9 |  |  |  | False |
| hex-passgates-easy-tc2 | hex | easy | 2 | 20 | 6 | 14 | 0 | 1 | 1 | 1 | DONE |  |  |  |  |  | False |
| hex-passgates-hard-tc1 | hex | hard | 1 | 20 | 5 | 12 | 3 | 0.85 | 0.694 | 1 | DONE |  |  |  |  |  | False |
| hex-passgates-hard-tc2 | hex | hard | 2 | 20 | 4 | 15 | 1 | 0.95 | 0.854 | 1 | DONE |  |  |  |  |  | False |
| hex-passgates-medium-tc1 | hex | medium | 1 | 20 | 7 | 9 | 4 | 0.8 | 0.625 | 0.975 | DONE |  |  |  |  |  | False |
| hex-passgates-medium-tc2 | hex | medium | 2 | 20 | 4 | 13 | 3 | 0.85 | 0.694 | 1 | DONE |  |  |  |  |  | False |
| square-passgates-easy-tc1 | square | easy | 1 | 60 | 54 | 2 | 4 | 0.933 | 0.87 | 0.996 | DONE |  |  |  |  |  | False |
| square-passgates-easy-tc2 | square | easy | 2 | 23 | 17 | 2 | 4 | 0.826 | 0.671 | 0.981 |  |  |  |  |  |  | True |
| square-passgates-hard-tc1 | square | hard | 1 | 60 | 25 | 4 | 31 | 0.483 | 0.357 | 0.61 | FAILED | 0.483 | 0.6 | 60 | 0.357 | 0.61 | False |
| square-passgates-hard-tc2 | square | hard | 2 | 60 | 26 | 3 | 31 | 0.483 | 0.357 | 0.61 | FAILED | 0.483 | 0.6 | 60 | 0.357 | 0.61 | False |
| square-passgates-medium-tc1 | square | medium | 1 | 40 | 18 | 3 | 19 | 0.525 | 0.37 | 0.68 | FAILED | 0.525 | 0.75 | 40 | 0.37 | 0.68 | False |
| square-passgates-medium-tc2 | square | medium | 2 | 20 | 7 | 0 | 13 | 0.35 | 0.141 | 0.559 | FAILED | 0.35 | 0.75 | 20 | 0.141 | 0.559 | False |

## W+D by master seat color

| arm | geometry | opp | tc_idx | master_seat | n | wins | draws | losses | wd | lb | ub |
|---|---|---|---|---|---|---|---|---|---|---|---|
| hex-passgates-easy-tc1 | hex | easy | 1 | blue | 30 | 6 | 18 | 6 | 0.8 | 0.657 | 0.943 |
| hex-passgates-easy-tc1.attempt1 | hex | easy | 1 | blue | 10 | 1 | 6 | 3 | 0.7 | 0.416 | 0.984 |
| hex-passgates-easy-tc1 | hex | easy | 1 | red | 30 | 14 | 15 | 1 | 0.967 | 0.902 | 1.031 |
| hex-passgates-easy-tc1.attempt1 | hex | easy | 1 | red | 10 | 5 | 5 | 0 | 1 | 1 | 1 |
| hex-passgates-easy-tc2 | hex | easy | 2 | blue | 10 | 2 | 8 | 0 | 1 | 1 | 1 |
| hex-passgates-easy-tc2 | hex | easy | 2 | red | 10 | 4 | 6 | 0 | 1 | 1 | 1 |
| hex-passgates-hard-tc1 | hex | hard | 1 | blue | 10 | 1 | 6 | 3 | 0.7 | 0.416 | 0.984 |
| hex-passgates-hard-tc1 | hex | hard | 1 | red | 10 | 4 | 6 | 0 | 1 | 1 | 1 |
| hex-passgates-hard-tc2 | hex | hard | 2 | blue | 10 | 0 | 9 | 1 | 0.9 | 0.714 | 1.086 |
| hex-passgates-hard-tc2 | hex | hard | 2 | red | 10 | 4 | 6 | 0 | 1 | 1 | 1 |
| hex-passgates-medium-tc1 | hex | medium | 1 | blue | 10 | 2 | 6 | 2 | 0.8 | 0.552 | 1.048 |
| hex-passgates-medium-tc1 | hex | medium | 1 | red | 10 | 5 | 3 | 2 | 0.8 | 0.552 | 1.048 |
| hex-passgates-medium-tc2 | hex | medium | 2 | blue | 10 | 3 | 6 | 1 | 0.9 | 0.714 | 1.086 |
| hex-passgates-medium-tc2 | hex | medium | 2 | red | 10 | 1 | 7 | 2 | 0.8 | 0.552 | 1.048 |
| square-passgates-easy-tc1 | square | easy | 1 | blue | 30 | 27 | 2 | 1 | 0.967 | 0.902 | 1.031 |
| square-passgates-easy-tc1 | square | easy | 1 | red | 30 | 27 | 0 | 3 | 0.9 | 0.793 | 1.007 |
| square-passgates-easy-tc2 | square | easy | 2 | blue | 11 | 9 | 0 | 2 | 0.818 | 0.59 | 1.046 |
| square-passgates-easy-tc2 | square | easy | 2 | red | 12 | 8 | 2 | 2 | 0.833 | 0.622 | 1.044 |
| square-passgates-hard-tc1 | square | hard | 1 | blue | 30 | 7 | 2 | 21 | 0.3 | 0.136 | 0.464 |
| square-passgates-hard-tc1 | square | hard | 1 | red | 30 | 18 | 2 | 10 | 0.667 | 0.498 | 0.835 |
| square-passgates-hard-tc2 | square | hard | 2 | blue | 30 | 12 | 2 | 16 | 0.467 | 0.288 | 0.645 |
| square-passgates-hard-tc2 | square | hard | 2 | red | 30 | 14 | 1 | 15 | 0.5 | 0.321 | 0.679 |
| square-passgates-medium-tc1 | square | medium | 1 | blue | 20 | 4 | 2 | 14 | 0.3 | 0.099 | 0.501 |
| square-passgates-medium-tc1 | square | medium | 1 | red | 20 | 14 | 1 | 5 | 0.75 | 0.56 | 0.94 |
| square-passgates-medium-tc2 | square | medium | 2 | blue | 10 | 1 | 0 | 9 | 0.1 | -0.086 | 0.286 |
| square-passgates-medium-tc2 | square | medium | 2 | red | 10 | 6 | 0 | 4 | 0.6 | 0.296 | 0.904 |

## per seat pair game outcomes

| arm | geometry | opp | tc_idx | master_seat | master_wins | draws | master_losses | unfinished |
|---|---|---|---|---|---|---|---|---|
| hex-passgates-easy-tc1 | hex | easy | 1 | blue | 6 | 18 | 6 | 0 |
| hex-passgates-easy-tc1.attempt1 | hex | easy | 1 | blue | 1 | 6 | 3 | 0 |
| hex-passgates-easy-tc1 | hex | easy | 1 | red | 14 | 15 | 1 | 0 |
| hex-passgates-easy-tc1.attempt1 | hex | easy | 1 | red | 5 | 5 | 0 | 0 |
| hex-passgates-easy-tc2 | hex | easy | 2 | blue | 2 | 8 | 0 | 0 |
| hex-passgates-easy-tc2 | hex | easy | 2 | red | 4 | 6 | 0 | 0 |
| hex-passgates-hard-tc1 | hex | hard | 1 | blue | 1 | 6 | 3 | 0 |
| hex-passgates-hard-tc1 | hex | hard | 1 | red | 4 | 6 | 0 | 0 |
| hex-passgates-hard-tc2 | hex | hard | 2 | blue | 0 | 9 | 1 | 0 |
| hex-passgates-hard-tc2 | hex | hard | 2 | red | 4 | 6 | 0 | 0 |
| hex-passgates-medium-tc1 | hex | medium | 1 | blue | 2 | 6 | 2 | 0 |
| hex-passgates-medium-tc1 | hex | medium | 1 | red | 5 | 3 | 2 | 0 |
| hex-passgates-medium-tc2 | hex | medium | 2 | blue | 3 | 6 | 1 | 0 |
| hex-passgates-medium-tc2 | hex | medium | 2 | red | 1 | 7 | 2 | 0 |
| square-passgates-easy-tc1 | square | easy | 1 | blue | 27 | 2 | 1 | 0 |
| square-passgates-easy-tc1 | square | easy | 1 | red | 27 | 0 | 3 | 0 |
| square-passgates-easy-tc2 | square | easy | 2 | blue | 9 | 0 | 2 | 1 |
| square-passgates-easy-tc2 | square | easy | 2 | red | 8 | 2 | 2 | 0 |
| square-passgates-hard-tc1 | square | hard | 1 | blue | 7 | 2 | 21 | 0 |
| square-passgates-hard-tc1 | square | hard | 1 | red | 18 | 2 | 10 | 0 |
| square-passgates-hard-tc2 | square | hard | 2 | blue | 12 | 2 | 16 | 0 |
| square-passgates-hard-tc2 | square | hard | 2 | red | 14 | 1 | 15 | 0 |
| square-passgates-medium-tc1 | square | medium | 1 | blue | 4 | 2 | 14 | 0 |
| square-passgates-medium-tc1 | square | medium | 1 | red | 14 | 1 | 5 | 0 |
| square-passgates-medium-tc2 | square | medium | 2 | blue | 1 | 0 | 9 | 0 |
| square-passgates-medium-tc2 | square | medium | 2 | red | 6 | 0 | 4 | 0 |

## search parity master versus opponent, untagged moves only

| arm | geometry | opp | tc_idx | is_master | search_moves | depth_mean | depth_median | nodes_m_mean | nps_m_mean | tt_mean | hf_mean | fh1_mean | t_mean | alloc_mean | t_over_alloc |
|---|---|---|---|---|---|---|---|---|---|---|---|---|---|---|---|
| hex-passgates-easy-tc1 | hex | easy | 1 | False | 8007 | 6.03 | 5 | 2.17 | 1.25 | 0 | 0 | 97.9 | 1.83 | 1.92 | 0.962 |
| hex-passgates-easy-tc1.attempt1 | hex | easy | 1 | False | 2531 | 5.92 | 5 | 2.37 | 1.32 | 0 | 0 | 97.8 | 1.88 | 1.98 | 0.962 |
| hex-passgates-easy-tc1 | hex | easy | 1 | True | 6492 | 7.05 | 6 | 10.28 | 6.99 | 26.4 | 5.5 | 98.2 | 1.6 | 2.33 | 0.71 |
| hex-passgates-easy-tc1.attempt1 | hex | easy | 1 | True | 2105 | 6.68 | 5 | 10.9 | 7.41 | 26.8 | 6 | 98.2 | 1.66 | 2.3 | 0.732 |
| hex-passgates-easy-tc2 | hex | easy | 2 | False | 3141 | 5.96 | 4 | 4.55 | 1.56 | 0 | 0 | 98.1 | 3.05 | 3.24 | 0.963 |
| hex-passgates-easy-tc2 | hex | easy | 2 | True | 2693 | 6.94 | 5 | 20.7 | 8.77 | 25.1 | 9.3 | 98.5 | 2.54 | 3.79 | 0.713 |
| hex-passgates-hard-tc1 | hex | hard | 1 | False | 2333 | 6.26 | 5 | 7.09 | 4.88 | 24.4 | 12 | 98 | 1.55 | 2.25 | 0.7 |
| hex-passgates-hard-tc1 | hex | hard | 1 | True | 2345 | 6.15 | 5 | 9.53 | 6.78 | 26.7 | 5.2 | 98 | 1.52 | 2.2 | 0.708 |
| hex-passgates-hard-tc2 | hex | hard | 2 | False | 2652 | 6.55 | 5 | 11.25 | 4.58 | 25.6 | 16 | 98.2 | 2.58 | 3.81 | 0.7 |
| hex-passgates-hard-tc2 | hex | hard | 2 | True | 2678 | 6.29 | 5 | 16.29 | 6.58 | 27.3 | 7.1 | 98.3 | 2.63 | 3.78 | 0.725 |
| hex-passgates-medium-tc1 | hex | medium | 1 | False | 2022 | 6.24 | 5 | 6.2 | 2.9 | 18.1 | 35 | 97.8 | 2.36 | 2.47 | 0.967 |
| hex-passgates-medium-tc1 | hex | medium | 1 | True | 2060 | 6.06 | 5 | 11.98 | 7.96 | 23.3 | 6.9 | 98.1 | 1.64 | 2.39 | 0.696 |
| hex-passgates-medium-tc2 | hex | medium | 2 | False | 2375 | 6.83 | 5 | 10.8 | 3.03 | 20 | 46.2 | 98.1 | 3.86 | 4.11 | 0.96 |
| hex-passgates-medium-tc2 | hex | medium | 2 | True | 2403 | 6.71 | 5 | 22.09 | 8.57 | 24.5 | 9.8 | 98.1 | 2.78 | 4.03 | 0.714 |
| square-passgates-easy-tc1 | square | easy | 1 | False | 1619 | 6.47 | 6 | 4.7 | 1.26 | 0 | 0 | 96.8 | 3.86 | 3.92 | 0.979 |
| square-passgates-easy-tc1 | square | easy | 1 | True | 1057 | 8.31 | 8 | 22.84 | 6.09 | 19 | 19 | 95.9 | 4.02 | 4.69 | 0.83 |
| square-passgates-easy-tc2 | square | easy | 2 | False | 877 | 7.42 | 7 | 7.26 | 1.38 | 0 | 0 | 97.3 | 5.57 | 5.77 | 0.965 |
| square-passgates-easy-tc2 | square | easy | 2 | True | 597 | 9.7 | 9 | 37.71 | 7.22 | 19.2 | 26.8 | 96.5 | 5.81 | 7.37 | 0.801 |
| square-passgates-hard-tc1 | square | hard | 1 | False | 1799 | 7.76 | 8 | 12.94 | 4.47 | 15.7 | 35.1 | 95.6 | 3.15 | 4.07 | 0.746 |
| square-passgates-hard-tc1 | square | hard | 1 | True | 1792 | 7.74 | 8 | 18.55 | 6.33 | 17.4 | 15.4 | 95.9 | 3.21 | 4.12 | 0.754 |
| square-passgates-hard-tc2 | square | hard | 2 | False | 1524 | 8.39 | 8 | 22.03 | 4.05 | 15.9 | 50.5 | 95.7 | 5.76 | 7.45 | 0.763 |
| square-passgates-hard-tc2 | square | hard | 2 | True | 1451 | 8.17 | 8 | 32.02 | 5.63 | 17.2 | 24.8 | 96.1 | 6.06 | 7.71 | 0.774 |
| square-passgates-medium-tc1 | square | medium | 1 | False | 1100 | 8.33 | 8 | 8.51 | 2.12 | 15.3 | 62.4 | 95.5 | 4.38 | 4.49 | 0.97 |
| square-passgates-medium-tc1 | square | medium | 1 | True | 1047 | 8.61 | 8 | 18.73 | 5.53 | 18.8 | 15.7 | 95.7 | 3.74 | 4.62 | 0.8 |
| square-passgates-medium-tc2 | square | medium | 2 | False | 450 | 7.85 | 8 | 17.37 | 2.3 | 11.6 | 83.7 | 95.5 | 7.77 | 7.86 | 0.988 |
| square-passgates-medium-tc2 | square | medium | 2 | True | 457 | 7.89 | 8 | 36.18 | 6.04 | 14.6 | 28.5 | 96 | 6.27 | 7.89 | 0.769 |

## game length distributions

| arm | geometry | opp | tc_idx | n | moves_mean | moves_median | moves_min | moves_max | draw_mean | decided_mean | decided_min | decided_max |
|---|---|---|---|---|---|---|---|---|---|---|---|---|
| hex-passgates-easy-tc1 | hex | easy | 1 | 60 | 267 | 397 | 28 | 397 | 397 | 108.1 | 28 | 285 |
| hex-passgates-easy-tc1.attempt1 | hex | easy | 1 | 20 | 253.2 | 397 | 41 | 397 | 397 | 77.3 | 41 | 121 |
| hex-passgates-easy-tc2 | hex | easy | 2 | 20 | 314.2 | 397 | 28 | 397 | 397 | 121 | 28 | 250 |
| hex-passgates-hard-tc1 | hex | hard | 1 | 20 | 282.9 | 397 | 29 | 397 | 397 | 111.6 | 29 | 223 |
| hex-passgates-hard-tc2 | hex | hard | 2 | 20 | 319.3 | 397 | 37 | 397 | 397 | 86.2 | 37 | 165 |
| hex-passgates-medium-tc1 | hex | medium | 1 | 20 | 248.1 | 182.5 | 86 | 397 | 397 | 126.3 | 86 | 189 |
| hex-passgates-medium-tc2 | hex | medium | 2 | 20 | 291.1 | 397 | 65 | 397 | 397 | 94.3 | 65 | 122 |
| square-passgates-easy-tc1 | square | easy | 1 | 60 | 54.4 | 35 | 31 | 256 | 256 | 47.4 | 31 | 148 |
| square-passgates-easy-tc2 | square | easy | 2 | 23 | 74.8 | 47 | 25 | 256 | 256 | 57.5 | 25 | 140 |
| square-passgates-hard-tc1 | square | hard | 1 | 60 | 78.3 | 49.5 | 29 | 256 | 256 | 65.6 | 29 | 170 |
| square-passgates-hard-tc2 | square | hard | 2 | 60 | 66.3 | 59.5 | 22 | 256 | 256 | 56.3 | 22 | 151 |
| square-passgates-medium-tc1 | square | medium | 1 | 40 | 72.3 | 53 | 23 | 256 | 256 | 57.4 | 23 | 111 |
| square-passgates-medium-tc2 | square | medium | 2 | 20 | 62 | 51 | 21 | 165 |  | 62 | 21 | 165 |

## draw profile per geometry

| geometry | games | draws | draw_share | draw_moves_min | draw_moves_max |
|---|---|---|---|---|---|
| hex | 180 | 107 | 0.594 | 397 | 397 |
| square | 263 | 14 | 0.053 | 256 | 256 |

## turning points of master losses

| arm | game | flip_move | phase | max_pro_before | cp_at_flip | max_pro_kind | after_move | cp_after | tag_after |
|---|---|---|---|---|---|---|---|---|---|
| hex-passgates-easy-tc1 | 4 | 91 | late | 1073741744 | 0 | mate | 92 | -1073741792 |  |
| hex-passgates-easy-tc1 | 6 | 68 | late | 7900 | 900 | cp | 69 | -2000 |  |
| hex-passgates-easy-tc1 | 20 | 60 | late | 6000 | 1500 | cp | 61 | -4500 |  |
| hex-passgates-easy-tc1 | 22 | 147 | late | 19200 | 0 | cp | 148 | -1073741792 |  |
| hex-passgates-easy-tc1 | 24 | 65 | late | 6000 | 1000 | cp | 66 | -1100 |  |
| hex-passgates-easy-tc1 | 33 | 101 | late | 7400 | 0 | cp | 102 | -6300 |  |
| hex-passgates-easy-tc1 | 60 | 30 | mid | 6000 | 600 | cp | 31 | -1000 |  |
| hex-passgates-easy-tc1.attempt1 | 10 | 98 | late | 6400 | 0 | cp | 99 | -1073741808 |  |
| hex-passgates-easy-tc1.attempt1 | 12 | 110 | late | 1073741712 | 0 | mate | 111 | -13400 |  |
| hex-passgates-easy-tc1.attempt1 | 14 | 46 | late | 6000 | 400 | cp | 47 | -1800 |  |
| hex-passgates-hard-tc1 | 12 | 24 | mid | 5600 | 600 | cp | 25 | -200 |  |
| hex-passgates-hard-tc1 | 14 | 111 | late | 1073741712 | 500 | mate | 112 | -2000 |  |
| hex-passgates-hard-tc1 | 18 | 61 | late | 5400 | 0 | cp | 62 | -8800 |  |
| hex-passgates-hard-tc2 | 12 | 23 | mid | 4400 | 500 | cp | 24 | -1300 |  |
| hex-passgates-medium-tc1 | 3 | 91 | late | 1073741744 | 1073741744 | mate | 92 | -1073741776 | VCF |
| hex-passgates-medium-tc1 | 9 | 72 | late | 11700 | 600 | cp | 73 | -1000 |  |
| hex-passgates-medium-tc1 | 10 | 22 | mid | 5400 | 1000 | cp | 23 | -500 |  |
| hex-passgates-medium-tc1 | 12 | 159 | late | 1073741712 | 0 | mate | 160 | -20000 |  |
| hex-passgates-medium-tc2 | 11 | 43 | late | 900 | 0 | cp | 44 | -1000 |  |
| hex-passgates-medium-tc2 | 16 | 60 | late | 4900 | 400 | cp | 61 | -1100 |  |
| hex-passgates-medium-tc2 | 19 | 78 | late | 0 | 0 | cp | 79 | -7500 |  |
| square-passgates-easy-tc1 | 9 | 54 | late | 1073741712 | 1800 | mate | 55 | -9000 |  |
| square-passgates-easy-tc1 | 11 | 110 | late | 11000 | 0 | cp | 111 | -1073741632 |  |
| square-passgates-easy-tc1 | 25 | 38 | mid | 1300 | 0 | cp | 39 | -1073741760 |  |
| square-passgates-easy-tc1 | 30 | 89 | late | 9400 | 0 | cp | 90 | -1073741760 |  |
| square-passgates-easy-tc2 | 4 | 61 | late | 1073741712 | 1100 | mate | 62 | -1600 |  |
| square-passgates-easy-tc2 | 9 | 112 | late | 9300 | 400 | cp | 113 | -1600 | PONDER |
| square-passgates-easy-tc2 | 10 | 16 | early | 1073741712 | 1073741712 | mate | 17 | -1073741680 |  |
| square-passgates-easy-tc2 | 11 | 53 | late | 9000 | 0 | cp | 54 | -600 |  |
| square-passgates-hard-tc1 | 2 | 16 | early | 1073741712 | 1073741712 | mate | 17 | -1300 |  |
| square-passgates-hard-tc1 | 4 | 4 | early | 1000 | 1000 | cp | 5 | -900 |  |
| square-passgates-hard-tc1 | 5 | 83 | late | 1073741712 | 200 | mate | 84 | -5400 |  |
| square-passgates-hard-tc1 | 6 | 4 | early | 1000 | 1000 | cp | 5 | -900 |  |
| square-passgates-hard-tc1 | 8 | 12 | early | 400 | 400 | cp | 13 | -500 |  |
| square-passgates-hard-tc1 | 10 | 13 | early | 1000 | 400 | cp | 14 | -500 | PONDER |
| square-passgates-hard-tc1 | 11 | 53 | late | 1073741712 | 900 | mate | 54 | -5100 |  |
| square-passgates-hard-tc1 | 12 | 13 | early | 1000 | 400 | cp | 14 | -500 | PONDER |
| square-passgates-hard-tc1 | 13 | 75 | late | 7500 | 300 | cp | 76 | -4100 |  |
| square-passgates-hard-tc1 | 14 | 51 | late | 1073741712 | 0 | mate | 52 | -1000 |  |
| square-passgates-hard-tc1 | 17 | 103 | late | 8500 | 0 | cp | 104 | -1073741776 | VCF |
| square-passgates-hard-tc1 | 22 | 4 | early | 1000 | 1000 | cp | 5 | -1500 |  |
| square-passgates-hard-tc1 | 24 | 4 | early | 1000 | 1000 | cp | 5 | -900 |  |
| square-passgates-hard-tc1 | 25 | 63 | late | 9800 | 3500 | cp | 64 | -4300 |  |
| square-passgates-hard-tc1 | 26 | 13 | early | 1000 | 400 | cp | 14 | -500 | PONDER |
| square-passgates-hard-tc1 | 32 | 4 | early | 1000 | 1000 | cp | 5 | -1500 |  |
| square-passgates-hard-tc1 | 34 | 4 | early | 1000 | 1000 | cp | 5 | -900 |  |
| square-passgates-hard-tc1 | 36 | 13 | early | 1000 | 400 | cp | 14 | -500 |  |
| square-passgates-hard-tc1 | 38 | 4 | early | 300 | 300 | cp | 5 | -1500 |  |
| square-passgates-hard-tc1 | 39 | 31 | mid | 1073741712 | 1073741712 | mate | 32 | -400 |  |
| square-passgates-hard-tc1 | 43 | 49 | late | 6200 | 1500 | cp | 50 | -600 |  |
| square-passgates-hard-tc1 | 44 | 158 | late | 19600 | 19100 | cp | 159 | -1073741776 | VCF |
| square-passgates-hard-tc1 | 46 | 36 | mid | 1100 | 900 | cp | 37 | -300 | PONDER |
| square-passgates-hard-tc1 | 48 | 82 | late | 12700 | 7600 | cp | 83 | -1073741776 | VCF |
| square-passgates-hard-tc1 | 50 | 4 | early | 1000 | 1000 | cp | 5 | -1500 | PONDER |
| square-passgates-hard-tc1 | 51 | 33 | mid | 1073741712 | 1073741712 | mate | 34 | -1000 |  |
| square-passgates-hard-tc1 | 54 | 13 | early | 1000 | 400 | cp | 14 | -500 | PONDER |
| square-passgates-hard-tc1 | 56 | 13 | early | 1000 | 400 | cp | 14 | -500 | PONDER |
| square-passgates-hard-tc1 | 57 | 151 | late | 1073741712 | 0 | mate | 152 | -3900 |  |
| square-passgates-hard-tc1 | 58 | 120 | late | 32300 | 18900 | cp | 121 | -1073741776 | VCF |
| square-passgates-hard-tc1 | 59 | 33 | mid | 1073741712 | 1073741712 | mate | 34 | -1400 |  |
| square-passgates-hard-tc2 | 5 | 17 | early | 1073741712 | 1073741712 | mate | 18 | -1073741712 |  |
| square-passgates-hard-tc2 | 10 | 14 | early | 1000 | 400 | cp | 15 | -1000 | PONDER |
| square-passgates-hard-tc2 | 11 | 33 | mid | 8500 | 4000 | cp | 34 | -3900 |  |
| square-passgates-hard-tc2 | 13 | 65 | late | 1073741712 | 0 | mate | 66 | -10000 |  |
| square-passgates-hard-tc2 | 14 | 4 | early | 1000 | 1000 | cp | 5 | -900 |  |
| square-passgates-hard-tc2 | 15 | 71 | late | 1073741712 | 0 | mate | 72 | -1073741808 | VCF |
| square-passgates-hard-tc2 | 18 | 4 | early | 1000 | 1000 | cp | 5 | -1500 |  |
| square-passgates-hard-tc2 | 20 | 4 | early | 1000 | 1000 | cp | 5 | -900 |  |
| square-passgates-hard-tc2 | 21 | 13 | early | 1800 | 1000 | cp | 14 | -300 |  |
| square-passgates-hard-tc2 | 22 | 26 | mid | 4900 | 1500 | cp | 27 | -400 |  |
| square-passgates-hard-tc2 | 25 | 61 | late | 1073741712 | 4300 | mate | 62 | -1073741776 | VCF |
| square-passgates-hard-tc2 | 27 | 35 | mid | 1073741616 | 1073741616 | mate | 36 | -1073741776 | VCF |
| square-passgates-hard-tc2 | 30 | 60 | late | 8600 | 500 | cp | 61 | -1800 |  |
| square-passgates-hard-tc2 | 32 | 4 | early | 1000 | 1000 | cp | 5 | -1500 | PONDER |
| square-passgates-hard-tc2 | 36 | 66 | late | 1073741680 | 1073741680 | mate | 67 | -1073741776 | VCF |
| square-passgates-hard-tc2 | 38 | 14 | early | 1073741712 | 1073741712 | mate | 15 | -1073741680 |  |
| square-passgates-hard-tc2 | 41 | 41 | late | 6000 | 700 | cp | 42 | -2500 |  |
| square-passgates-hard-tc2 | 42 | 78 | late | 1073741744 | 1073741744 | mate | 79 | -1073741776 | VCF |
| square-passgates-hard-tc2 | 43 | 17 | early | 1073741712 | 1073741712 | mate | 18 | -10600 |  |
| square-passgates-hard-tc2 | 45 | 17 | early | 1073741712 | 1073741712 | mate | 18 | -10200 |  |
| square-passgates-hard-tc2 | 46 | 4 | early | 1000 | 1000 | cp | 5 | -1500 |  |
| square-passgates-hard-tc2 | 47 | 34 | mid | 1073741712 | 600 | mate | 35 | -800 |  |
| square-passgates-hard-tc2 | 48 | 4 | early | 1000 | 1000 | cp | 5 | -1500 |  |
| square-passgates-hard-tc2 | 49 | 57 | late | 10400 | 1600 | cp | 58 | -900 |  |
| square-passgates-hard-tc2 | 50 | 64 | late | 24800 | 14700 | cp | 65 | -1073741744 | VCF |
| square-passgates-hard-tc2 | 51 | 17 | early | 1073741680 | 1073741680 | mate | 18 | -10600 |  |
| square-passgates-hard-tc2 | 52 | 4 | early | 1000 | 1000 | cp | 5 | -1600 |  |
| square-passgates-hard-tc2 | 53 | 32 | mid | 4700 | 1400 | cp | 33 | -1000 | PONDER |
| square-passgates-hard-tc2 | 54 | 44 | late | 1073741744 | 1073741744 | mate | 45 | -1073741776 | VCF |
| square-passgates-hard-tc2 | 55 | 17 | early | 1073741680 | 1073741680 | mate | 18 | -9900 |  |
| square-passgates-hard-tc2 | 60 | 4 | early | 1000 | 1000 | cp | 5 | -900 |  |
| square-passgates-medium-tc1 | 2 | 13 | early | 1000 | 400 | cp | 14 | -500 | PONDER |
| square-passgates-medium-tc1 | 3 | 37 | mid | 800 | 800 | cp | 38 | -1100 |  |
| square-passgates-medium-tc1 | 6 | 14 | early | 1073741712 | 1073741712 | mate | 15 | -1073741680 |  |
| square-passgates-medium-tc1 | 7 | 42 | late | 2000 | 800 | cp | 43 | -2500 |  |
| square-passgates-medium-tc1 | 10 | 20 | early | 4900 | 2500 | cp | 21 | -400 |  |
| square-passgates-medium-tc1 | 12 | 4 | early | 1000 | 1000 | cp | 5 | -900 |  |
| square-passgates-medium-tc1 | 16 | 51 | late | 1000 | 400 | cp | 52 | -500 |  |
| square-passgates-medium-tc1 | 17 | 22 | mid | 1073741712 | 100 | mate | 23 | -1000 |  |
| square-passgates-medium-tc1 | 20 | 69 | late | 1073741712 | 1500 | mate | 70 | -1100 |  |
| square-passgates-medium-tc1 | 22 | 25 | mid | 1073741712 | 0 | mate | 26 | -1100 |  |
| square-passgates-medium-tc1 | 26 | 4 | early | 1000 | 1000 | cp | 5 | -1500 |  |
| square-passgates-medium-tc1 | 28 | 60 | late | 1073741712 | 600 | mate | 61 | -600 |  |
| square-passgates-medium-tc1 | 30 | 13 | early | 1000 | 400 | cp | 14 | -500 | PONDER |
| square-passgates-medium-tc1 | 31 | 69 | late | 1073741744 | 1073741744 | mate | 70 | -1073741776 | VCF |
| square-passgates-medium-tc1 | 32 | 4 | early | 1000 | 1000 | cp | 5 | -900 | PONDER |
| square-passgates-medium-tc1 | 34 | 56 | late | 8200 | 2300 | cp | 57 | -1000 |  |
| square-passgates-medium-tc1 | 35 | 90 | late | 9600 | 0 | cp | 91 | -9200 |  |
| square-passgates-medium-tc1 | 36 | 43 | late | 3600 | 2100 | cp | 44 | -100 |  |
| square-passgates-medium-tc1 | 40 | 4 | early | 1000 | 1000 | cp | 5 | -1600 |  |
| square-passgates-medium-tc2 | 2 | 98 | late | 18200 | 18200 | cp | 99 | -1073741776 | VCF |
| square-passgates-medium-tc2 | 4 | 14 | early | 1073741712 | 1073741712 | mate | 15 | -5400 |  |
| square-passgates-medium-tc2 | 8 | 160 | late | 1073741712 | 1073741712 | mate | 161 | -1073741744 | VCF |
| square-passgates-medium-tc2 | 10 | 12 | early | 1073741712 | 1073741712 | mate | 13 | -1073741680 |  |
| square-passgates-medium-tc2 | 12 | 14 | early | 1073741712 | 1073741712 | mate | 15 | -5400 |  |
| square-passgates-medium-tc2 | 13 | 65 | late | 1073741712 | 1500 | mate | 66 | -1500 | PONDER |
| square-passgates-medium-tc2 | 14 | 4 | early | 1000 | 1000 | cp | 5 | -1500 | PONDER |
| square-passgates-medium-tc2 | 15 | 39 | mid | 1073741712 | 1200 | mate | 40 | -1600 |  |
| square-passgates-medium-tc2 | 16 | 12 | early | 1000 | 400 | cp | 13 | -500 |  |
| square-passgates-medium-tc2 | 17 | 81 | late | 1073741712 | 1073741712 | mate | 82 | -1073741744 |  |
| square-passgates-medium-tc2 | 18 | 4 | early | 1000 | 1000 | cp | 5 | -900 |  |
| square-passgates-medium-tc2 | 19 | 39 | mid | 8500 | 800 | cp | 40 | -2700 |  |
| square-passgates-medium-tc2 | 20 | 4 | early | 1000 | 1000 | cp | 5 | -1600 |  |

## turning point summary per arm

| arm | losses | never_ahead | flip_median | flip_mean | flipped_early | flipped_mid | flipped_late |
|---|---|---|---|---|---|---|---|
| hex-passgates-easy-tc1 | 7 | 0 | 68 | 80.3 | 0 | 1 | 6 |
| hex-passgates-easy-tc1.attempt1 | 3 | 0 | 98 | 84.7 | 0 | 0 | 3 |
| hex-passgates-hard-tc1 | 3 | 0 | 61 | 65.3 | 0 | 1 | 2 |
| hex-passgates-hard-tc2 | 1 | 0 | 23 | 23 | 0 | 1 | 0 |
| hex-passgates-medium-tc1 | 4 | 0 | 81.5 | 86 | 0 | 1 | 3 |
| hex-passgates-medium-tc2 | 3 | 0 | 60 | 60.3 | 0 | 0 | 3 |
| square-passgates-easy-tc1 | 4 | 0 | 71.5 | 72.8 | 0 | 1 | 3 |
| square-passgates-easy-tc2 | 4 | 0 | 57 | 60.5 | 1 | 0 | 3 |
| square-passgates-hard-tc1 | 31 | 0 | 16 | 40.6 | 16 | 4 | 11 |
| square-passgates-hard-tc2 | 31 | 0 | 17 | 29.8 | 16 | 5 | 10 |
| square-passgates-medium-tc1 | 19 | 0 | 25 | 33.7 | 8 | 3 | 8 |
| square-passgates-medium-tc2 | 13 | 0 | 14 | 42 | 7 | 2 | 4 |

## ponder verdict counts

| arm | ponder_side | verdict | n |
|---|---|---|---|
| hex-passgates-easy-tc1 | master | adopt | 1466 |
| hex-passgates-easy-tc1 | master | miss | 2380 |
| hex-passgates-easy-tc1 | master | reject | 1032 |
| hex-passgates-easy-tc1.attempt1 | master | adopt | 407 |
| hex-passgates-easy-tc1.attempt1 | master | miss | 752 |
| hex-passgates-easy-tc1.attempt1 | master | reject | 349 |
| hex-passgates-easy-tc2 | master | adopt | 434 |
| hex-passgates-easy-tc2 | master | miss | 986 |
| hex-passgates-easy-tc2 | master | reject | 424 |
| hex-passgates-hard-tc1 | hard | adopt | 490 |
| hex-passgates-hard-tc1 | hard | miss | 617 |
| hex-passgates-hard-tc1 | hard | reject | 548 |
| hex-passgates-hard-tc1 | master | adopt | 469 |
| hex-passgates-hard-tc1 | master | miss | 565 |
| hex-passgates-hard-tc1 | master | reject | 588 |
| hex-passgates-hard-tc2 | hard | adopt | 534 |
| hex-passgates-hard-tc2 | hard | miss | 764 |
| hex-passgates-hard-tc2 | hard | reject | 609 |
| hex-passgates-hard-tc2 | master | adopt | 503 |
| hex-passgates-hard-tc2 | master | miss | 701 |
| hex-passgates-hard-tc2 | master | reject | 629 |
| hex-passgates-medium-tc1 | master | adopt | 407 |
| hex-passgates-medium-tc1 | master | miss | 495 |
| hex-passgates-medium-tc1 | master | reject | 487 |
| hex-passgates-medium-tc1 | medium | adopt | 445 |
| hex-passgates-medium-tc1 | medium | miss | 546 |
| hex-passgates-medium-tc1 | medium | reject | 496 |
| hex-passgates-medium-tc2 | master | adopt | 496 |
| hex-passgates-medium-tc2 | master | miss | 655 |
| hex-passgates-medium-tc2 | master | reject | 544 |
| hex-passgates-medium-tc2 | medium | adopt | 530 |
| hex-passgates-medium-tc2 | medium | miss | 708 |
| hex-passgates-medium-tc2 | medium | reject | 618 |
| square-passgates-easy-tc1 | master | adopt | 343 |
| square-passgates-easy-tc1 | master | miss | 388 |
| square-passgates-easy-tc1 | master | reject | 161 |
| square-passgates-easy-tc2 | master | adopt | 199 |
| square-passgates-easy-tc2 | master | miss | 222 |
| square-passgates-easy-tc2 | master | reject | 75 |
| square-passgates-hard-tc1 | hard | adopt | 459 |
| square-passgates-hard-tc1 | hard | miss | 399 |
| square-passgates-hard-tc1 | hard | reject | 442 |
| square-passgates-hard-tc1 | master | adopt | 392 |
| square-passgates-hard-tc1 | master | miss | 367 |
| square-passgates-hard-tc1 | master | reject | 431 |
| square-passgates-hard-tc2 | hard | adopt | 376 |
| square-passgates-hard-tc2 | hard | miss | 353 |
| square-passgates-hard-tc2 | hard | reject | 408 |
| square-passgates-hard-tc2 | master | adopt | 355 |
| square-passgates-hard-tc2 | master | miss | 395 |
| square-passgates-hard-tc2 | master | reject | 326 |
| square-passgates-medium-tc1 | master | adopt | 285 |
| square-passgates-medium-tc1 | master | miss | 239 |
| square-passgates-medium-tc1 | master | reject | 249 |
| square-passgates-medium-tc1 | medium | adopt | 306 |
| square-passgates-medium-tc1 | medium | miss | 235 |
| square-passgates-medium-tc1 | medium | reject | 291 |
| square-passgates-medium-tc2 | master | adopt | 112 |
| square-passgates-medium-tc2 | master | miss | 83 |
| square-passgates-medium-tc2 | master | reject | 126 |
| square-passgates-medium-tc2 | medium | adopt | 147 |
| square-passgates-medium-tc2 | medium | miss | 103 |
| square-passgates-medium-tc2 | medium | reject | 124 |

## phase folds of the master edge

| arm | geometry | opp | tc_idx | phase | moves | ahead_share | behind_share | mating_share | getting_mated_share | clamped_cp_mean |
|---|---|---|---|---|---|---|---|---|---|---|
| hex-passgates-easy-tc1 | hex | easy | 1 | early | 1200 | 0.513 | 0.486 | 0.002 | 0 | 265 |
| hex-passgates-easy-tc1 | hex | easy | 1 | late | 13640 | 0.492 | 0.399 | 0.01 | 0.003 | 372 |
| hex-passgates-easy-tc1 | hex | easy | 1 | mid | 1179 | 0.64 | 0.354 | 0.036 | 0 | 2408 |
| hex-passgates-easy-tc1.attempt1 | hex | easy | 1 | early | 400 | 0.553 | 0.445 | 0 | 0 | 323 |
| hex-passgates-easy-tc1.attempt1 | hex | easy | 1 | late | 4263 | 0.473 | 0.4 | 0.012 | 0.003 | 1378 |
| hex-passgates-easy-tc1.attempt1 | hex | easy | 1 | mid | 400 | 0.81 | 0.19 | 0.055 | 0 | 4011 |
| hex-passgates-easy-tc2 | hex | easy | 2 | early | 400 | 0.563 | 0.433 | 0.008 | 0 | 775 |
| hex-passgates-easy-tc2 | hex | easy | 2 | late | 5496 | 0.452 | 0.416 | 0.006 | 0 | 1097 |
| hex-passgates-easy-tc2 | hex | easy | 2 | mid | 388 | 0.559 | 0.438 | 0.013 | 0 | 402 |
| hex-passgates-hard-tc1 | hex | hard | 1 | early | 400 | 0.515 | 0.485 | 0.003 | 0 | 128 |
| hex-passgates-hard-tc1 | hex | hard | 1 | late | 4868 | 0.446 | 0.435 | 0.007 | 0.004 | -729 |
| hex-passgates-hard-tc1 | hex | hard | 1 | mid | 389 | 0.542 | 0.427 | 0.026 | 0.003 | 847 |
| hex-passgates-hard-tc2 | hex | hard | 2 | early | 400 | 0.538 | 0.458 | 0 | 0.005 | 248 |
| hex-passgates-hard-tc2 | hex | hard | 2 | late | 5589 | 0.421 | 0.429 | 0.003 | 0.001 | -769 |
| hex-passgates-hard-tc2 | hex | hard | 2 | mid | 397 | 0.418 | 0.567 | 0.03 | 0.028 | -509 |
| hex-passgates-medium-tc1 | hex | medium | 1 | early | 400 | 0.513 | 0.483 | 0.003 | 0 | 107 |
| hex-passgates-medium-tc1 | hex | medium | 1 | late | 4162 | 0.37 | 0.517 | 0.012 | 0.007 | -955 |
| hex-passgates-medium-tc1 | hex | medium | 1 | mid | 400 | 0.395 | 0.605 | 0.003 | 0 | -1123 |
| hex-passgates-medium-tc2 | hex | medium | 2 | early | 400 | 0.518 | 0.483 | 0.003 | 0 | 307 |
| hex-passgates-medium-tc2 | hex | medium | 2 | late | 5021 | 0.401 | 0.503 | 0.004 | 0.004 | -321 |
| hex-passgates-medium-tc2 | hex | medium | 2 | mid | 400 | 0.438 | 0.55 | 0 | 0 | -496 |
| square-passgates-easy-tc1 | square | easy | 1 | early | 1200 | 0.69 | 0.227 | 0.039 | 0 | 3633 |
| square-passgates-easy-tc1 | square | easy | 1 | late | 1066 | 0.585 | 0.331 | 0.072 | 0.038 | 3726 |
| square-passgates-easy-tc1 | square | easy | 1 | mid | 998 | 0.774 | 0.222 | 0.508 | 0.004 | 17140 |
| square-passgates-easy-tc2 | square | easy | 2 | early | 460 | 0.759 | 0.165 | 0.041 | 0.009 | 3651 |
| square-passgates-easy-tc2 | square | easy | 2 | late | 876 | 0.588 | 0.368 | 0.067 | 0.042 | 1581 |
| square-passgates-easy-tc2 | square | easy | 2 | mid | 384 | 0.875 | 0.125 | 0.328 | 0.013 | 12921 |
| square-passgates-hard-tc1 | square | hard | 1 | early | 1200 | 0.443 | 0.484 | 0.013 | 0.008 | 352 |
| square-passgates-hard-tc1 | square | hard | 1 | late | 2396 | 0.429 | 0.477 | 0.038 | 0.073 | -1264 |
| square-passgates-hard-tc1 | square | hard | 1 | mid | 1103 | 0.461 | 0.529 | 0.127 | 0.14 | -984 |
| square-passgates-hard-tc2 | square | hard | 2 | early | 1200 | 0.478 | 0.449 | 0.022 | 0.03 | 162 |
| square-passgates-hard-tc2 | square | hard | 2 | late | 1772 | 0.444 | 0.464 | 0.093 | 0.067 | 616 |
| square-passgates-hard-tc2 | square | hard | 2 | mid | 1004 | 0.45 | 0.545 | 0.108 | 0.142 | -2134 |
| square-passgates-medium-tc1 | square | medium | 1 | early | 800 | 0.451 | 0.475 | 0.01 | 0.008 | 196 |
| square-passgates-medium-tc1 | square | medium | 1 | late | 1347 | 0.417 | 0.474 | 0.07 | 0.111 | -1305 |
| square-passgates-medium-tc1 | square | medium | 1 | mid | 743 | 0.494 | 0.495 | 0.168 | 0.073 | 2589 |
| square-passgates-medium-tc2 | square | medium | 2 | early | 400 | 0.51 | 0.415 | 0.023 | 0.023 | 1001 |
| square-passgates-medium-tc2 | square | medium | 2 | late | 506 | 0.508 | 0.49 | 0.081 | 0.095 | 648 |
| square-passgates-medium-tc2 | square | medium | 2 | mid | 333 | 0.505 | 0.495 | 0.111 | 0.123 | -362 |

## loss mechanism classification

| arm | losses | was_mating | vcf_after | vct_after | ponder_after | plain_after |
|---|---|---|---|---|---|---|
| hex-passgates-easy-tc1 | 7 | 1 | 0 | 0 | 0 | 7 |
| hex-passgates-easy-tc1.attempt1 | 3 | 1 | 0 | 0 | 0 | 3 |
| hex-passgates-hard-tc1 | 3 | 1 | 0 | 0 | 0 | 3 |
| hex-passgates-hard-tc2 | 1 | 0 | 0 | 0 | 0 | 1 |
| hex-passgates-medium-tc1 | 4 | 2 | 1 | 0 | 0 | 3 |
| hex-passgates-medium-tc2 | 3 | 0 | 0 | 0 | 0 | 3 |
| square-passgates-easy-tc1 | 4 | 1 | 0 | 0 | 0 | 4 |
| square-passgates-easy-tc2 | 4 | 2 | 0 | 0 | 1 | 3 |
| square-passgates-hard-tc1 | 31 | 8 | 4 | 0 | 7 | 20 |
| square-passgates-hard-tc2 | 31 | 14 | 7 | 0 | 3 | 21 |
| square-passgates-medium-tc1 | 19 | 6 | 1 | 0 | 3 | 15 |
| square-passgates-medium-tc2 | 13 | 7 | 2 | 0 | 2 | 9 |

## flip ownership and immediate mate after the flip

| arm | losses | was_mating | mated_immediately_after | after_by_master | after_by_opponent | opp_reply_mated |
|---|---|---|---|---|---|---|
| hex-passgates-easy-tc1 | 7 | 1 | 2 | 3 | 4 | 0 |
| hex-passgates-easy-tc1.attempt1 | 3 | 1 | 1 | 0 | 3 | 1 |
| hex-passgates-hard-tc1 | 3 | 1 | 0 | 2 | 1 | 0 |
| hex-passgates-hard-tc2 | 1 | 0 | 0 | 1 | 0 | 0 |
| hex-passgates-medium-tc1 | 4 | 2 | 1 | 2 | 2 | 1 |
| hex-passgates-medium-tc2 | 3 | 0 | 0 | 1 | 2 | 0 |
| square-passgates-easy-tc1 | 4 | 1 | 3 | 4 | 0 | 0 |
| square-passgates-easy-tc2 | 4 | 2 | 1 | 2 | 2 | 1 |
| square-passgates-hard-tc1 | 31 | 8 | 4 | 7 | 24 | 4 |
| square-passgates-hard-tc2 | 31 | 14 | 9 | 2 | 29 | 9 |
| square-passgates-medium-tc1 | 19 | 6 | 2 | 9 | 10 | 2 |
| square-passgates-medium-tc2 | 13 | 7 | 4 | 0 | 13 | 4 |
