package fixture

func Both(a, b bool) bool {
	return a && b
}

func Either(a, b bool) bool {
	return a || b
}

func Mask(a, b byte) byte {
	return a&b | a ^ b
}

func Shift(a uint16) uint16 {
	return a<<2 | a>>3
}

func Lit(a bool) bool {
	if a {
		return true
	}
	return false
}
