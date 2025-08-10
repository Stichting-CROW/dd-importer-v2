package main

import (
	"context"
	"fmt"
	"log"

	"github.com/jackc/pgx/v5"
)

func insertParkEventZones(conn *pgx.Conn, records []ParkEventLocationLinkedToZone) error {
	rows := make([][]any, len(records))
	for i, r := range records {
		rows[i] = []any{r.ParkEventID, r.StatRef}
	}

	copyCount, err := conn.CopyFrom(
		context.Background(),
		pgx.Identifier{"park_event_zone"},
		[]string{"park_event_id", "zone_stats_ref"},
		pgx.CopyFromRows(rows),
	)

	if err != nil {
		return fmt.Errorf("copy from failed: %w", err)
	}

	log.Printf("Inserted %d rows", copyCount)
	return nil
}
