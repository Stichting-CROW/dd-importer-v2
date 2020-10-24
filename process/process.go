package process

import (
	"deelfietsdashboard-importer/feed"
	"log"

	"github.com/go-redis/redis"
	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq" // postgres
)

// Result is a container for new data.
type Result struct {
	CurrentBikesInFeed map[string]feed.Bike
	CreatedEvents      []Event
	FeedIsEmpty        bool
}

// DataProcessor struct for eventchannel and redis.
type DataProcessor struct {
	eventChan chan []Event
	rdb       *redis.Client
	db        *sqlx.DB
}

// InitDataProcessor sets up all dataprocessing.
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
		db:        db,
	}

}

// ProcessNewData call this function with new data from a datafeed.
func (processor DataProcessor) ProcessNewData(strategy string, old map[string]feed.Bike, new []feed.Bike) Result {
	result := Result{}
	switch strategy {
	case "clean":
		result = CleanCompare(old, new)
	case "gps":
		result = CleanCompare(old, new)
	}
	processor.eventChan <- result.CreatedEvents
	return result
}
