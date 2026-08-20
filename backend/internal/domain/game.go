package domain

type GameState struct {
	Board            Board
	CurrentPlayer    Player
	MoveNumber       int
	IsGameOver       bool
	Winner           Player
	WinningLine      []Position
	EndReason        string
	BoardHistory     []Board
	MoveHistory      []Position
	TimeControl      string
	InitialTimeMs    int64
	IncrementSeconds int
	GameMode         GameMode
}

func NewGameState(mode GameMode, timeControl string, initialTimeMs int64, incrementSeconds int) GameState {
	return GameState{
		Board:            NewBoard(),
		CurrentPlayer:    PlayerRed,
		TimeControl:      timeControl,
		InitialTimeMs:    initialTimeMs,
		IncrementSeconds: incrementSeconds,
		GameMode:         mode,
	}
}

func (g GameState) WithMove(x, y int) (GameState, error) {
	if g.IsGameOver {
		return g, ErrGameOver
	}
	if g.CurrentPlayer == PlayerRed && !IsValidSecondMove(g.Board, x, y) {
		return g, ErrOpenRule
	}
	newBoard, err := g.Board.PlaceStoneChecked(x, y, g.CurrentPlayer)
	if err != nil {
		return g, err
	}

	history := make([]Board, len(g.BoardHistory)+1)
	history[0] = g.Board
	copy(history[1:], g.BoardHistory)

	moveHistory := make([]Position, len(g.MoveHistory)+1)
	copy(moveHistory, g.MoveHistory)
	moveHistory[len(g.MoveHistory)] = Position{X: x, Y: y}

	return GameState{
		Board:            newBoard,
		CurrentPlayer:    g.CurrentPlayer.Opponent(),
		MoveNumber:       g.MoveNumber + 1,
		BoardHistory:     history,
		MoveHistory:      moveHistory,
		TimeControl:      g.TimeControl,
		InitialTimeMs:    g.InitialTimeMs,
		IncrementSeconds: g.IncrementSeconds,
		GameMode:         g.GameMode,
	}, nil
}

func (g GameState) UndoMove() (GameState, error) {
	if g.IsGameOver {
		return g, ErrGameOver
	}
	if len(g.BoardHistory) == 0 {
		return g, ErrNoMoves
	}

	previousBoard := g.BoardHistory[0]
	newHistory := g.BoardHistory[1:]
	newMoveHistory := g.MoveHistory[:len(g.MoveHistory)-1]

	newPlayer := g.CurrentPlayer.Opponent()
	if g.MoveNumber-1 == 0 {
		newPlayer = PlayerRed
	}

	return GameState{
		Board:            previousBoard,
		CurrentPlayer:    newPlayer,
		MoveNumber:       g.MoveNumber - 1,
		BoardHistory:     newHistory,
		MoveHistory:      newMoveHistory,
		TimeControl:      g.TimeControl,
		InitialTimeMs:    g.InitialTimeMs,
		IncrementSeconds: g.IncrementSeconds,
		GameMode:         g.GameMode,
	}, nil
}

func (g GameState) CanUndo() bool {
	return len(g.BoardHistory) > 0 && !g.IsGameOver
}

func (g GameState) WithGameOver(winner Player, line []Position) GameState {
	g.CurrentPlayer = PlayerNone
	g.IsGameOver = true
	g.Winner = winner
	g.WinningLine = line
	g.EndReason = "win"
	return g
}

// WithTimeout ends the game because player ran out of clock time.
func (g GameState) WithTimeout(winner Player) GameState {
	g.CurrentPlayer = PlayerNone
	g.IsGameOver = true
	g.Winner = winner
	g.EndReason = "timeout"
	return g
}

// WithDraw ends the game without a winner (board full).
func (g GameState) WithDraw() GameState {
	g.CurrentPlayer = PlayerNone
	g.IsGameOver = true
	g.Winner = PlayerNone
	g.EndReason = "draw"
	return g
}
