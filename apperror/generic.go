package apperror

type Error string

func (e Error) Error() string { return string(e) }

const (
	ErrNoData          = Error("no records found")
	ErrMultipleRecords = Error("mulitple records found")
	ErrGuest           = Error("user is guest")
	ErrNotFriend       = Error("user is not in friendlist")
	ErrPrivate         = Error("item is private")
	ErrRecordChanged   = Error("write conflict")
	ErrDenied          = Error("not allowed") // eg. upd/del not allowed
)
