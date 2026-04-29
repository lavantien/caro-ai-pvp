package api

type CreateGameRequest struct {
	TimeControl    string `json:"time_control"`
	GameMode       string `json:"game_mode"`
	Difficulty     *int   `json:"difficulty"`
	RedDifficulty  *int   `json:"red_difficulty"`
	BlueDifficulty *int   `json:"blue_difficulty"`
}

type MoveRequest struct {
	X int `json:"x"`
	Y int `json:"y"`
}

type GameResponse struct {
	Board             []CellResponse     `json:"board"`
	CurrentPlayer     string             `json:"current_player"`
	MoveNumber        int                `json:"move_number"`
	IsGameOver        bool               `json:"is_game_over"`
	Winner            string             `json:"winner"`
	WinningLine       []PositionResponse `json:"winning_line"`
	RedTimeRemaining  float64            `json:"red_time_remaining"`
	BlueTimeRemaining float64            `json:"blue_time_remaining"`
	TimeControl       string             `json:"time_control"`
	InitialTime       int                `json:"initial_time"`
	Increment         int                `json:"increment"`
	GameMode          string             `json:"game_mode"`
	RedDifficulty     *int               `json:"red_difficulty"`
	BlueDifficulty    *int               `json:"blue_difficulty"`
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
