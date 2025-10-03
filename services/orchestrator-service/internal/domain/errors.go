package domain


var (
	ErrNotFound       = Error("not_found")
	ErrInvalidInput   = Error("invalid_input")
	ErrDuplicateTx    = Error("duplicate_tx")
	ErrUnsupportedStd = Error("unsupported_standard")
	ErrUnauthenticated = Error("unauthenticated")
	ErrSessionTimeout  = Error("session_timeout")
)

type Error string

func (e Error) Error() string { return string(e) }

