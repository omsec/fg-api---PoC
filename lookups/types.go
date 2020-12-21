package lookups

// there's no real good solution in GO :-/
// https://www.reddit.com/r/golang/comments/kh305t/restrict_allowed_values_for_strings/

// Registry of Lookup/Code Types
const (
	LTuserRole = iota
	LTlang
	LTgame
)

// LookupType returns names of the available code types
func LookupType(lt int) string {

	var str = ""

	switch {
	case lt == LTuserRole:
		str = "user role"
	case lt == LTlang:
		str = "user language"
	case lt == LTgame:
		str = "game"
	}

	return str
}
