source: https://gomocup.org/detail-information/

# Detail Information

## Introduction

Gomocup is a competition in gomoku (five in row). The competitors are not humans, but computer artificial intelligences (AI, brain). The participant can be every programmer knowing how to create such a program. It's quite easy. If you aren't one of them, try to download several interesting programs or choose links to see several interesting addresses. In the other case we challenge you to join in the competition. The AIs compete in tournaments with each other using manager - a Win32/Win64 compatible application (see chapter Compatibility), that you can download here. The AIs communicate with the manager either via text files or via standard input/output, so that you can create it in any programming language generating executable Win32/Win64 file. The matches are logged into a file so that you can download this file and replay the match. There are no entry fees, neither any prizes for winners, the only reward is the publicity at this site and your own feeling of your success.

## Gomoku Rules

If you don't know the rules of gomoku, read this paragraph up, because this game is an invaluable companion in boring lessons or even more boring lectures. Gomoku is the game designed for two players playing on the square paper or on the go-board or other similar board. The players are alternatively putting their own signs (cross and circle) on the paper as far as one of them manages to create the line of five or more stones (freestyle gomoku rules) in the vertical, horizontal or diagonal direction and due to becomes the winner.

## Joining in

If you have created your own brain program, you can join the tournaments for free, please see how to join the Gomocup. If you want to join in, create a Win32/Win64 compatible (see chapter Compatibility) program (the brain) that can communicate with the manager. You can use any programming language.

There are two ways how to communicate. The first one is [via text files](https://plastovicka.github.io/protocl1en.htm). The AI program being executed in every move reads the text file with position, computes the best move, writes the best coordination into the text file and terminates itself. This protocol is considered obsolete. The second one is [via standard input/output](https://plastovicka.github.io/protocl2en.htm). The program(AI) is executed only at the beginning of the game reads/writes information from/to standard input/output and while the opponent is moving, it is only paused. Important: The brain must flush each output to prevent the data from being buffered. This can be performed by flush function in Pascal or fflush(stdout) in C. The main advantage is the possibility to keep all computations in memory during the whole game. You can download a [base of such brain in C++](https://plastovicka.github.io/download/skel_cpp.zip), a [base of brain in C#](https://plastovicka.github.io/download/skel_cs.zip), a [base of brain in Delphi](https://plastovicka.github.io/download/skel_pas.zip), or a [base of brain in Java](https://plastovicka.github.io/download/skel_java.zip).

## Tournament rules

The tournaments are played every year at spring. Every correct and registered brain may join. One person may submit only one brain to each tournament as the first author. The tournament system is set up to according to the number of attendants. Usually, AIs are divided into groups on the basis of recent results. The brains in the same group fight each other for even number of matches (one duel), a draw is also possible. In order to reduce the first player's advantage, we use balanced openings from a set.

The freestyle, standard, renju, and caro rules are played. For the freestyle rule, the winner is the AI forming a continuous line of 5 or more stones. Before 2023 (inclusive), the board has 20x20 squares. Starting from 2024, we have both 15x15 and 20x20 freestyle tournaments. For the standard rule, the board has 15x15 squares, and the winner is the AI forming a continuous line of 5 (more than 5 is not a win). The renju rule used in the tournament is based on the [international rules of renju](http://www.renju.net/study/rifrules.php) with the following modifications: the openings for the renju tournament are prepared by renju experts, and are not limited to the 26 patterns; players (i.e. the AIs) are not allowed to pass; any game in renju tournament will be automatic draw after 200 moves. For the caro rule, we adopt the standard caro rule where the winner must have an unbroken row of exactly five stones that is not blocked at both ends (i.e., neither oxxxxxo nor xxxxxx is a win for x) on a 15x15 board.

[Three points for a win](https://en.wikipedia.org/wiki/Three_points_for_a_win) is used in tournaments before 2016 (inclusive). Concretely, AI obtains 3 points for winning a duel, 1 point for a draw or no point for a loss. The AI obtaining the most points is the winner. If there are more AIs with the same number of points, the score (games won/games loss) then match between then time per move determines. Matches between finalists from the same groups are played again. Starting from 2017, [Elo rating system](https://en.wikipedia.org/wiki/Elo_rating_system) is used to evaluate AIs' strength, and the AI with the highest Elo rating is the winner. The winner of the tournament is the winner of the final. Exact rules, time limits, memory limits etc. are published in advance and usually reflect the last year's rules.

Before launching the tournament, AI may not take more than 256 megabytes of space (zipped together with all necessary files). AI is allowed to save its information into files, that will last during the tournament. The AI creates a directory with its name for storing it. The size of all files belonging to one AI must not go over 20 megabytes at any time during the tournament.

The brain must obey the time limits for one turn and for one match, that are announced before each tournament. The limits won't be stricter than 30 seconds for a turn and 3 minutes for a match. The brain determines how much time it spends for the turn according to the remaining time info, provided by the manager. The speed of the PC is not exactly known, so the brain must be able to adapt. The brain must never occupy more RAM than allowed by a memory limit, which is also announced before the tournament and is not lesser than 70MB. The brain must not display anything at the screen. The brain may not communicate via a network.

Although multi-thread programs are allowed in the Gomocup tournaments, one brain is restricted to use only one CPU core by setting CPU affinities to the program (see the document of Piskvork about how to set CPU affinities), so the brains could not gain speed by multi-thread thinking. Furthermore, pondering is not allowed in the tournament, since the program would be suspended by the manager while the other brain is thinking.

## Compatibility

Before 2017 (inclusive), AI must be Win32 compatible program, which supports Windows 7, Windows 8, Windows 10 on both platforms x86 and x64. Starting from 2018, we no longer require AI to be Win32 compatible. In 2021, only compatibility with Windows 10 is mandatory. Starting from 2022, compatibility with Windows 10 and Windows 11 (including the corresponding server versions such as Windows Server 2016, Windows Server 2019, and Windows Server 2022) is required. In addition, hybrid (x86 and x64) programs are allowed. If you want to gain from speed of x64 and be compatible with x86, you can pack your x86 and x64 executable into ZIP archive. The x64 file name must contain 64 substring. For example, pbrain-yixin.exe and pbrain-yixin64.exe in Yixin.zip.

DOS programs are not allowed anymore.

## How to join in?

If you have created your own brain program, you can join the tournament for free, please see how to join the Gomocup.

## Credits

Kai Sun (sunkaicngmail.com) - organizer of the 17th to the present tournaments, author of GomocupJudge, webmaster
 Tianyi Hao (haotianyi0126.com) - organizer of the 17th to the present tournaments, author of GomocupJudge, webmaster
 Tomas Kubes (Tomas_Kubesseznam.cz) - organizer of the 7th to 16th tournaments, English translation of the old Gomocup website, webmaster of the old Gomocup website
 Vojta Havranek (havranekwerewolf.wz.cz) - organizer of first six tournaments, webmaster of the old Gomocup website
 Petr Lastovicka (omnibotseznam.cz) - author of Piskvork manager and brain bases code, maintenance of the protocol
 Jiri Fontan (jiri.fontansofis.cz) - Gomotur manager author
 Martin Petricek (bilbocentrum.cz) - founder of the new protocol
 Jan Dolezal (--) - author of first DOS manager
 additional thanks to Vaclav Synacek, Petr Zizka and Jan Curn for their contribution

### News

- Gomocup 2026 Announcement
 Apr 6, 2026
- Call for Sponsors
 Apr 6, 2026
- Gomocup 2025 Announcement
 Mar 7, 2025
- Gomocup 2024 Announcement
 Feb 19, 2024
- Gomocup 2023 Announcement
 Mar 19, 2023

Copyright © Gomocup. Theme based on [GenericBlue](http://templated.co/genericblue) designed by [TEMPLATED](http://templated.co).