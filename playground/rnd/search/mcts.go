package main

import (
	"math"
	"math/rand"

	"github.com/lavantien/caro-ai-pvp/internal/config"
	"github.com/lavantien/caro-ai-pvp/internal/rules"
)

type mctsNode struct {
	parent   *mctsNode
	move     rules.Cell
	mover    rules.Color
	wins     int
	visits   int
	untried  []rules.Cell
	children []*mctsNode
}

type mctsEngine struct {
	board *rules.Board
	rng   *rand.Rand
	root  *mctsNode
	pool  []mctsNode
	used  int
	path  []*mctsNode
}

func newMcts(board *rules.Board, seed int64) *mctsEngine {
	e := &mctsEngine{
		board: board,
		rng:   rand.New(rand.NewSource(seed)),
		pool:  make([]mctsNode, 1<<18),
		path:  make([]*mctsNode, 0, config.BoardCells),
	}
	e.root = e.alloc(rules.Cell(config.BoardCells), board.Side.Opponent(), nil)
	e.root.untried = e.candidates()
	return e
}

func (e *mctsEngine) alloc(cell rules.Cell, mover rules.Color, parent *mctsNode) *mctsNode {
	n := &e.pool[e.used]
	e.used++
	n.parent = parent
	n.move = cell
	n.mover = mover
	n.wins, n.visits = 0, 0
	n.untried = n.untried[:0]
	n.children = n.children[:0]
	return n
}

func (e *mctsEngine) candidates() []rules.Cell {
	return freeInRegion(e.board, dilateBB(bb(e.board.Full), candRadiusSearch))
}

func (e *mctsEngine) playout() rules.Color {
	e.path = e.path[:0]
	node := e.root
	for len(node.untried) == 0 && len(node.children) > 0 {
		node = e.bestChild(node)
		e.board.Make(node.move)
		e.path = append(e.path, node)
	}
	if len(node.untried) > 0 {
		i := e.rng.Intn(len(node.untried))
		cell := node.untried[i]
		node.untried[i] = node.untried[len(node.untried)-1]
		node.untried = node.untried[:len(node.untried)-1]
		child := e.alloc(cell, e.board.Side, node)
		node.children = append(node.children, child)
		e.board.Make(cell)
		child.untried = e.candidates()
		e.path = append(e.path, child)
		node = child
	}
	winner := e.simulate()
	for range e.path {
		e.board.Unmake()
	}
	e.root.visits++
	e.backprop(winner)
	return winner
}

func (e *mctsEngine) bestChild(node *mctsNode) *mctsNode {
	best := node.children[0]
	bestVal := -1.0
	ln := math.Log(float64(node.visits) + 1)
	for _, c := range node.children {
		var val float64
		if c.visits == 0 {
			val = 1e9
		} else {
			val = float64(c.wins)/float64(c.visits) + math.Sqrt2*math.Sqrt(ln/float64(c.visits))
		}
		if val > bestVal {
			bestVal = val
			best = c
		}
	}
	return best
}

func (e *mctsEngine) simulate() rules.Color {
	mover := e.board.Side
	made := 0
	winner := rules.Empty
	for range mctsRollover {
		cands := e.candidates()
		if len(cands) == 0 {
			break
		}
		var cell rules.Cell
		if e.rng.Intn(2) == 0 {
			cell = greedyCell(e.board, cands, mover, e.rng)
		} else {
			cell = cands[e.rng.Intn(len(cands))]
		}
		e.board.Make(cell)
		made++
		if e.board.FastLastMoveWin(mover, cell) {
			winner = mover
			break
		}
		mover = mover.Opponent()
	}
	for range made {
		e.board.Unmake()
	}
	return winner
}

func (e *mctsEngine) backprop(winner rules.Color) {
	for i := len(e.path) - 1; i >= 0; i-- {
		n := e.path[i]
		n.visits++
		if winner == rules.Empty || winner == n.mover {
			n.wins++
		}
	}
}
