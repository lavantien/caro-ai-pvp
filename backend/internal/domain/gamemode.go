package domain

type GameMode int

const (
	GameModePvP GameMode = iota
	GameModePvAI
	GameModeAivAI
)

func (m GameMode) String() string {
	switch m {
	case GameModePvP:
		return "pvp"
	case GameModePvAI:
		return "pvai"
	case GameModeAivAI:
		return "aivai"
	default:
		return "pvp"
	}
}

func ParseGameMode(s string) GameMode {
	switch s {
	case "pvai":
		return GameModePvAI
	case "aivai":
		return GameModeAivAI
	default:
		return GameModePvP
	}
}
