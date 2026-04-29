package domain

type Player int

const (
	PlayerNone Player = iota
	PlayerRed
	PlayerBlue
)

func (p Player) Opponent() Player {
	switch p {
	case PlayerRed:
		return PlayerBlue
	case PlayerBlue:
		return PlayerRed
	default:
		return PlayerNone
	}
}

func (p Player) IsValid() bool {
	return p == PlayerRed || p == PlayerBlue
}

func (p Player) String() string {
	switch p {
	case PlayerRed:
		return "red"
	case PlayerBlue:
		return "blue"
	default:
		return "none"
	}
}

func ParsePlayer(s string) (Player, bool) {
	switch s {
	case "red":
		return PlayerRed, true
	case "blue":
		return PlayerBlue, true
	case "none":
		return PlayerNone, true
	default:
		return PlayerNone, false
	}
}
