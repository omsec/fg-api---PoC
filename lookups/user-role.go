package lookups

// Symbols of legal values
const (
	URguest = iota
	URmember
	URadmin
)

// UserRole returns a "generic" string for a given value
func UserRole(value int) string {

	var str = ""

	switch {
	case value == URguest:
		str = "guest"
	case value == URmember:
		str = "member"
	case value == URadmin:
		str = "admin"
	}

	return str
}
