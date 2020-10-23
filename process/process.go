package process

import (
	"deelfietsdashboard-importer/feed"
	"log"

	"github.com/go-redis/redis"
	_ "github.com/lib/pq"
    "github.com/jmoiron/sqlx"
)

type ProcessResult struct {
	CurrentBikesInFeed map[string]feed.Bike
	CreatedEvents      []Event
	FeedIsEmpty        bool
}

// DataProcessor struct for eventchannel and redis.
type DataProcessor struct {
	eventChan chan []ProcessResult
	rdb       *redis.Client
	db        *sqlx.DB
}

func InitDataProcessor() DataProcessor {
	db, err := sqlx.Connect("postgres", "dbname=deelfietsdashboard sslmode=disable")
	if err != nil {
		log.Fatal(err)
	}
	return DataProcessor{
		rdb: redis.NewClient(&redis.Options{
			Addr:     "localhost:6379",
			Password: "", // no password set
			DB:       0,  // use default DB
		}),
		eventChan: make(chan []Event),
		db:  db,
	}

}

func (processor DataProcessor) ProcessNewData(strategy string, old map[string]feed.Bike, new []feed.Bike) ProcessResult {
	result := ProcessResult{}
	switch strategy {
	case "clean":
		result = CleanCompare(old, new)
	case "gps":
		result = CleanCompare(old, new)
	}
	processor.eventChan <- result
	return result
}
