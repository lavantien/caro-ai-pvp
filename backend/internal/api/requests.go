package api

type CreateGameRequest struct {
	TimeControl    string `json:"timeControl"`
	GameMode       string `json:"gameMode"`
	Difficulty     *int   `json:"difficulty"`
	RedDifficulty  *int   `json:"redDifficulty"`
	BlueDifficulty *int   `json:"blueDifficulty"`
}

type MoveRequest struct {
	X int `json:"x"`
	Y int `json:"y"`
}

type GameResponse struct {
	Board             []CellResponse     `json:"board"`
	CurrentPlayer     string             `json:"currentPlayer"`
	MoveNumber        int                `json:"moveNumber"`
	IsGameOver        bool               `json:"isGameOver"`
	Winner            string             `json:"winner"`
	WinningLine       []PositionResponse `json:"winningLine"`
	RedTimeRemaining  float64            `json:"redTimeRemaining"`
	BlueTimeRemaining float64            `json:"blueTimeRemaining"`
	TimeControl       string             `json:"timeControl"`
	InitialTime       int                `json:"initialTime"`
	Increment         int                `json:"increment"`
	GameMode          string             `json:"gameMode"`
	RedDifficulty     *int               `json:"redDifficulty"`
	BlueDifficulty    *int               `json:"blueDifficulty"`
}

type CellResponse struct {
	X      int    `json:"x"`
	Y      int    `json:"y"`
	Player string `json:"player"`
}

type PositionResponse struct {
	X int `json:"x"`
	Y int `json:"y"`
}

type ErrorResponse struct {
	Error   string `json:"error"`
	Message string `json:"message"`
}
