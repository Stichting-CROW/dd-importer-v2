package main

import (
	"context"
	"log"
	"math"
	"os"
	"sync"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/peterstace/simplefeatures/geom"
	"github.com/tidwall/rtree"
)

func main() {
	dbURL := os.Getenv("DB_URL")
	log.Print(streamLargeQuery(dbURL))
}

type GeometryRTree struct {
	RTree      rtree.RTree
	Geometries map[string]ZoneGeometry
}

type ZoneGeometry struct {
	Area     geom.Geometry
	StatsRef string
}

type ParkEventLocation struct {
	ParkEventID int
	Location    geom.Point
}

func loadZones(conn *pgx.Conn) GeometryRTree {
	stmt := `
	SELECT st_asbinary(ST_makevalid(area)) as area, COALESCE(stats_ref, CONCAT('z:', zone_id)) 
	FROM zones
	LEFT JOIN geographies
	USING(zone_id)
	WHERE zone_type IN ('residential_area', 'municipality', 'custom')
	AND (retire_date IS NULL OR retire_date > NOW());
	`
	rows, err := conn.Query(context.Background(), stmt)
	if err != nil {
		log.Fatal(err)
	}

	start := time.Now()

	tree := GeometryRTree{
		RTree:      rtree.RTree{},
		Geometries: make(map[string]ZoneGeometry),
	}

	for rows.Next() {
		var location ZoneGeometry
		if err := rows.Scan(&location.Area, &location.StatsRef); err != nil {
			log.Fatal(err)
		}
		tree.Geometries[location.StatsRef] = location
		min, max, _ := location.Area.Envelope().MinMaxXYs()

		tree.RTree.Insert([2]float64{min.X, min.Y}, [2]float64{max.X, max.Y}, location.StatsRef)
	}
	log.Printf("Loading zones %d from database took %s", len(tree.Geometries), time.Since(start))

	return tree
}

func fetchLocations(conn *pgx.Conn, cursor int) ([]ParkEventLocation, int, error) {
	rows, err := conn.Query(context.Background(),
		`SELECT park_event_id, ST_AsBinary(location) as location
		FROM park_events
		WHERE park_event_id < $1
		ORDER BY park_event_id DESC
		LIMIT 10000`,
		cursor)

	// reset cursor to 0
	cursor = 0
	if err != nil {
		return []ParkEventLocation{}, 0, err
	}
	defer rows.Close()

	var res []ParkEventLocation

	for rows.Next() {
		var parkEventLocation ParkEventLocation
		if err := rows.Scan(&parkEventLocation.ParkEventID, &parkEventLocation.Location); err != nil {
			return []ParkEventLocation{}, 0, err
		}
		res = append(res, parkEventLocation)
		cursor = parkEventLocation.ParkEventID

	}

	return res, cursor, nil
}

func streamLargeQuery(connString string) error {
	ctx := context.Background()
	conn, err := pgx.Connect(ctx, connString)
	if err != nil {
		return err
	}

	defer conn.Close(ctx)
	tree := loadZones(conn)

	cursor := math.MaxInt32
	var res []ParkEventLocation
	for cursor > 0 {
		start := time.Now()
		log.Printf("Fetching locations with cursor: %d", cursor)
		res, cursor, err = fetchLocations(conn, cursor)
		if err != nil {
			log.Fatal(err)
		}
		log.Printf("Finished fetching %d locations, start checking intersections, took %s", len(res), time.Since(start))

		start = time.Now()
		parkEventsLinkedToLocation := tree.checkIntersections(res)

		log.Printf("Finished checking intersections found %d intersections, took %s", len(parkEventsLinkedToLocation), time.Since(start))
		log.Print("Bulk insert park events linked to zones")
		if err := insertParkEventZones(conn, parkEventsLinkedToLocation); err != nil {
			log.Fatal(err)
		}
		log.Print("Finished bulk insert park events linked to zones")
	}

	//cursor := 0
	return nil
}

func (tree *GeometryRTree) checkIntersections(locations []ParkEventLocation) []ParkEventLocationLinkedToZone {
	const workerCount = 16

	type job struct {
		location ParkEventLocation
	}
	type result struct {
		intersections []ParkEventLocationLinkedToZone
	}

	jobs := make(chan job, len(locations))
	results := make(chan result, len(locations))

	// Start worker goroutines
	var wg sync.WaitGroup
	for i := 0; i < workerCount; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := range jobs {
				intersections := tree.checkIntersection(j.location)
				results <- result{intersections: intersections}
			}
		}()
	}

	// Send jobs
	for _, location := range locations {
		jobs <- job{location: location}
	}
	close(jobs)

	// Wait for all workers to finish
	go func() {
		wg.Wait()
		close(results)
	}()

	// Collect results
	var res []ParkEventLocationLinkedToZone
	for r := range results {
		res = append(res, r.intersections...)
	}

	return res
}

type ParkEventLocationLinkedToZone struct {
	StatRef     string
	ParkEventID int
}

func (tree *GeometryRTree) checkIntersection(location ParkEventLocation) []ParkEventLocationLinkedToZone {
	var res []ParkEventLocationLinkedToZone
	point := location.Location.AsGeometry()
	coordinates, _ := location.Location.Coordinates()
	tree.RTree.Search([2]float64{coordinates.X, coordinates.Y}, [2]float64{coordinates.X, coordinates.Y},
		func(min, max [2]float64, data interface{}) bool {
			statsRef := data.(string)
			area := tree.Geometries[data.(string)]
			match, _ := geom.Contains(area.Area, point)
			if match {
				res = append(res, ParkEventLocationLinkedToZone{
					ParkEventID: location.ParkEventID,
					StatRef:     statsRef,
				})
			}

			return true
		},
	)
	return res
}
