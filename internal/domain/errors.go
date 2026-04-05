package domain

import "errors"

var (
	ErrEmptyNickname     = errors.New("nickname cannot be empty")
	ErrUserNotFound      = errors.New("user not found")
	ErrUserAlreadyExists = errors.New("user with this nickname already exists")
)
