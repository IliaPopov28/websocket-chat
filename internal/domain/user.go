package domain

import "time"

type User struct {
	Nickname string
	JoinedAt time.Time
}

func NewUser(nickname string) *User {
	return &User{
		Nickname: nickname,
		JoinedAt: time.Now(),
	}
}
