package domain

import "time"

type MessageType string

const (
	PublicMessage  MessageType = "public"
	PrivateMessage MessageType = "private"
	SystemMessage  MessageType = "system"
	ErrorMessage   MessageType = "error"
)

type Message struct {
	Type       MessageType `json:"type"`
	Sender     string      `json:"sender"`
	Reciptient string      `json:"reciptient,omitempty"`
	Content    string      `json:"content"`
	Timestamp  time.Time   `json:"timestamp"`
}
