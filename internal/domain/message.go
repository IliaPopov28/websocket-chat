package domain

import "time"

type MessageType string

const (
	PublicMessage   MessageType = "public"
	PrivateMessage  MessageType = "private"
	SystemMessage   MessageType = "system"
	ErrorMessage    MessageType = "error"
	UserListMessage MessageType = "user_list"
)

type Message struct {
	Type      MessageType `json:"type"`
	Sender    string      `json:"sender"`
	Recipient string      `json:"recipient,omitempty"`
	Content   string      `json:"content"`
	Users     []string    `json:"users,omitempty"`
	Timestamp time.Time   `json:"timestamp"`
}
