package process

import (
	"deelfietsdashboard-importer/feed"
	"time"
)

type Event struct {
	Bike      feed.Bike
	EventType string
	Timestamp time.Time
}
