package process

import (
	"deelfietsdashboard-importer/feed"
	"fmt"
	"log"
	"os"

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
	DB        *sqlx.DB
}

// InitDataProcessor sets up all dataprocessing.
func InitDataProcessor() DataProcessor {
	connStr := ""
	if os.Getenv("DEV") == "true" {
		connStr = "dbname=deelfietsdashboard sslmode=disable"
	} else {
		connStr = fmt.Sprintf("dbname=%s user=%s host=%s password=%s sslmode=disable",
			os.Getenv("DB_NAME"), os.Getenv("DB_USER"), os.Getenv("DB_HOST"), os.Getenv("DB_PASSWORD"))
	}

	db, err := sqlx.Connect("postgres", connStr)
	if err != nil {
		log.Fatal(err)
	}

	redisAddress := "localhost:6379"
	if os.Getenv("DEV") != "true" {
		redisAddress = os.Getenv("REDIS_HOST")
	}

	return DataProcessor{
		rdb: redis.NewClient(&redis.Options{
			Addr:     redisAddress,
			Password: "", // no password set
			DB:       0,  // use default DB
		}),
		eventChan: make(chan []Event),
		DB:        db,
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
