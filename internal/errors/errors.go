package errors

import "errors"

var (
	// Отсутствует userID
	ErrXUserID = errors.New("missing X-User-ID header")
	// Невалидный формат файла
	ErrInvalidMime = errors.New("invalid file format")
)
