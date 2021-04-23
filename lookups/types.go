package lookups

// since there are no joins in MongoDB, text descriptions of code values are fetched by the API

// there's no real good solution in GO :-/
// https://www.reddit.com/r/golang/comments/kh305t/restrict_allowed_values_for_strings/

// Registry of Lookup/Code Types
const (
	LTuserRole = iota
	LTlang
	LTgame
	LTprivacy
	LTvisibility
	LTcommentStatus
	LTcourseType
	LTcourseStyle
	LTseries
	LTcarClass
)

// LookupType returns names of the available code types
func LookupType(lt int) string {

	// Alternative:
	// string-const-array -> dann aber bounds checken!

	var str = ""

	switch {
	case lt == LTuserRole:
		str = "user role"
	case lt == LTlang:
		str = "user language"
	case lt == LTgame:
		str = "game"
	case lt == LTprivacy:
		str = "user privacy"
	case lt == LTvisibility:
		str = "visibility"
	case lt == LTcommentStatus:
		str = "comment status"
	case lt == LTcourseType:
		str = "course type"
	case lt == LTcourseStyle:
		str = "course style"
	case lt == LTseries:
		str = "series"
	case lt == LTcarClass:
		str = "car class"
	}

	return str
}
