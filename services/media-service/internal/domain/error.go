package domain

var (
	ErrNotFound           = errSentinel("not found")
	ErrAlreadyExists      = errSentinel("already exists")
	ErrInvalid            = errSentinel("invalid argument")
	ErrAssetNotFound      = errSentinel("asset not found")
	ErrAssetAlreadyExists = errSentinel("asset already exists")
	ErrInvalidMimeType    = errSentinel("invalid mime type")
	ErrInvalidFormat      = errSentinel("invalid format")
	ErrPinFailed          = errSentinel("pin failed")
	ErrStorageFailed      = errSentinel("storage failed")
	ErrInvalidInput       = errSentinel("invalid input")
)

type errSentinel string

func (e errSentinel) Error() string { return string(e) }
