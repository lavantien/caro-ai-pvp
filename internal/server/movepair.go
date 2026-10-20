package server

type MovePair struct {
	N          int
	Red        string
	Blue       string
	RedLatest  bool
	BlueLatest bool
}

func movePairs(names []string) []MovePair {
	if len(names) == 0 {
		return nil
	}
	pairs := make([]MovePair, 0, (len(names)+1)/2)
	for i := 0; i < len(names); i += 2 {
		p := MovePair{N: i/2 + 1, Red: names[i]}
		if i+1 < len(names) {
			p.Blue = names[i+1]
			p.BlueLatest = i+1 == len(names)-1
		} else {
			p.RedLatest = true
		}
		pairs = append(pairs, p)
	}
	return pairs
}
