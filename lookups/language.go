package lookups

// Symbols of legal values
const (
	LANGen = iota
	LANGde
)

// Language returns a "generic" string for a given value
func Language(value int) string {

	var str = ""

	switch {
	case value == LANGen:
		str = "en"
	case value == LANGde:
		str = "de"
	}

	return str
}
