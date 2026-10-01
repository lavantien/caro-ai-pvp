source: https://chessprogramming.org/Killer_Move

# Killer Move

Home * Chess * Moves * Killer Move

The Killer Move is a Move Ordering related issue.

The Killer Move is a quiet move which caused a beta-cutoff in a sibling Cut-node, or any other earlier branch in the tree with the same ply distance to the root. The [rule of thumb](https://en.wikipedia.org/wiki/Rule_of_thumb) is to try that move early direct after a possibly available hash move from the transposition table and considering apparently winning captures. This simple but efficient move ordering heuristic is called the Killer Heuristic. Similar to moves from the transposition table , killers may actually save the generation of quiet moves at all if it fails high, but require a legality test.

# See also

- Hash Move
- Killer Heuristic
- Mate Killers
- Move Ordering
- Pseudo-Legal Move
- PV-Move
- Refutation Move
- Threat Move from null move refutations

# Forum Posts

- [Killer moves](https://groups.google.com/group/gnu.chess/browse_frm/thread/fb62cff6dea1bf09) by Chua Kong Sian, gnu.chess, March 21, 1995
- [killer moves?](https://www.stmintz.com/ccc/index.php?id=325602) by Daniel Shawul, CCC, November 04, 2003
- [Killer moves?](http://www.talkchess.com/forum/viewtopic.php?t=40423) by Mike Robinson, CCC, September 16, 2011
- [Killer and move encoding](http://www.talkchess.com/forum/viewtopic.php?t=53289) by Fabio Gobbato, CCC, August 14, 2014 » Encoding Moves
- [Effectiveness of killer moves](http://www.talkchess.com/forum/viewtopic.php?t=53317) by Alex Ferguson, CCC, August 17, 2014
- [TTMove legality checking ? & Killers Move Format?](http://www.talkchess.com/forum/viewtopic.php?t=63090) by Mahmoud Uthman, CCC, February 08, 2017 » Hash Move
- [How much ELO should I expect to gain from killer moves?](http://www.talkchess.com/forum3/viewtopic.php?f=7&t=77734) by Christian Dean, CCC, July 16, 2021 » Playing Strength

Up one Level
