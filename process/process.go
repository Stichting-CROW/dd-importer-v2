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
	EventChan           chan []Event
	VehicleChan         chan []feed.Bike
	rdb                 *redis.Client
	DB                  *sqlx.DB
	tile38              *redis.Client
	NumberOfFeedsActive *int
}

var numberOfFeedsActive int

// InitDataProcessor sets up all dataprocessing.
func InitDataProcessor() DataProcessor {
	connStr := fmt.Sprintf("dbname=%s user=%s host=%s password=%s sslmode=disable",
		os.Getenv("PGDATABASE"), os.Getenv("PGUSER"), os.Getenv("PGHOST"), os.Getenv("PGPASSWORD"))

	db, err := sqlx.Connect("postgres", connStr+" binary_parameters=yes")
	if err != nil {
		log.Fatal(err)
	}

	redisAddress := "localhost:6379"
	if os.Getenv("DEV") != "true" {
		redisAddress = os.Getenv("REDIS_HOST")
	}

	tile38Address := "localhost:9851"
	if os.Getenv("DEV") != "true" {
		tile38Address = os.Getenv("TILE38_HOST")
	}

	numberOfFeedsActive = 0
	return DataProcessor{
		rdb: redis.NewClient(&redis.Options{
			Addr:     redisAddress,
			Password: "", // no password set
			DB:       0,  // use default DB
		}),
		EventChan:   make(chan []Event, 100),
		VehicleChan: make(chan []feed.Bike, 100),
		DB:          db,
		tile38: redis.NewClient(&redis.Options{
			Addr:     tile38Address,
			Password: "", // no password set
			DB:       0,  // use default DB
		}),
		NumberOfFeedsActive: &numberOfFeedsActive,
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
	processor.VehicleChan <- new
	processor.EventChan <- result.CreatedEvents
	return result
}
