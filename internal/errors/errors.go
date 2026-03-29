package errors

import "errors"

var (
	// ErrXUserID indicates that X-User-ID header is missing.
	ErrXUserID = errors.New("missing X-User-ID header")
	// ErrInvalidMime Невалидный формат файла
	ErrInvalidMime = errors.New("invalid file format")
	// ErrAvatarDeletionAlreadyQueued ошибка при которой должна вернукться не ошибка а уведомление что аватарка была удалена
	ErrAvatarDeletionAlreadyQueued = errors.New("avatar deletion already queued")
)
