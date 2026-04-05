package hub

import "time"

const (
	writeWait      = 20 * time.Second
	pongWait       = 60 * time.Second
	pingPeriod     = pongWait * 2 / 3
	maxMessageSize = 1024 * 1024
)
