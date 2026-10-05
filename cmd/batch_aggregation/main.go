package main

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"strings"
	"time"

	"deelfietsdashboard-importer/cmd/batch_aggregation/analyze"
	"deelfietsdashboard-importer/cmd/batch_aggregation/indicators"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/spf13/cobra"
)

var (
	indicatorsFlag string
	allFlag        bool
	fromFlag       string
	toFlag         string
)

func main() {
	if err := rootCmd.Execute(); err != nil {
		log.Fatal(err)
	}
}

var rootCmd = &cobra.Command{
	Use:   "batch-aggregation",
	Short: "Aggregate shared mobility statistics",
	Long: `batch-aggregation calculates KPIs from park events and writes them
to the moment_statistics and day_statistics tables.`,
	RunE: runDefault,
}

var recalculateCmd = &cobra.Command{
	Use:   "recalculate",
	Short: "Recalculate one or more indicators",
	Long: `Removes all existing values for the selected indicators and recalculates
them for the requested date range. Use either --indicators or --all.`,
	RunE: runRecalculate,
}

func init() {
	recalculateCmd.Flags().StringVar(&indicatorsFlag, "indicators", "", "Comma-separated list of indicator text IDs")
	recalculateCmd.Flags().BoolVar(&allFlag, "all", false, "Recalculate all indicators")
	recalculateCmd.Flags().StringVar(&fromFlag, "from", "", "Start date (YYYY-MM-DD); defaults to per-indicator first day")
	recalculateCmd.Flags().StringVar(&toFlag, "to", "", "End date (YYYY-MM-DD); defaults to yesterday")

	recalculateCmd.MarkFlagsOneRequired("indicators", "all")

	rootCmd.AddCommand(recalculateCmd)
}

func runDefault(cmd *cobra.Command, args []string) error {
	return executeRun(false, indicators.All, time.Time{}, time.Time{})
}

func runRecalculate(cmd *cobra.Command, args []string) error {
	selected, err := resolveSelectedIndicators()
	if err != nil {
		return err
	}

	from, to, err := parseDateRange(fromFlag, toFlag)
	if err != nil {
		return err
	}

	return executeRun(true, selected, from, to)
}

func resolveSelectedIndicators() ([]indicators.Indicator, error) {
	if indicatorsFlag != "" && allFlag {
		return nil, fmt.Errorf("use either --indicators or --all, not both")
	}

	if indicatorsFlag != "" {
		return indicators.Resolve(indicatorsFlag)
	}

	if allFlag {
		return indicators.All, nil
	}

	return nil, fmt.Errorf("use either --indicators or --all")
}

func parseDateRange(fromFlag, toFlag string) (time.Time, time.Time, error) {
	to := time.Now().Local().AddDate(0, 0, -1)
	var err error
	if toFlag != "" {
		to, err = time.ParseInLocation("2006-01-02", toFlag, time.Local)
		if err != nil {
			return time.Time{}, time.Time{}, fmt.Errorf("invalid --to date %q: %w", toFlag, err)
		}
	}

	from := indicators.DefaultFirstDay
	if fromFlag != "" {
		from, err = time.ParseInLocation("2006-01-02", fromFlag, time.Local)
		if err != nil {
			return time.Time{}, time.Time{}, fmt.Errorf("invalid --from date %q: %w", fromFlag, err)
		}
	}

	if from.After(to) {
		return time.Time{}, time.Time{}, fmt.Errorf("--from (%s) is after --to (%s)", from.Format("2006-01-02"), to.Format("2006-01-02"))
	}

	return from, to, nil
}

func executeRun(recalculate bool, selected []indicators.Indicator, from time.Time, to time.Time) error {
	pgConn := initPostgresDB()
	dConn := initDuckDB()
	startTime := time.Now()

	syncIndicatorsToPostgres(pgConn)

	if recalculate {
		deleteIndicatorData(pgConn, selected, from, to)
	}

	loadZones(dConn)

	runStart, runEnd := determineDateRange(recalculate, selected, from, to, pgConn)
	if runStart.After(runEnd) {
		log.Print("Nothing to calculate for the selected indicators and date range.")
		updateMaterializedViewsAndIndexes(pgConn)
		return nil
	}

	fmt.Printf("Date range: %s -> %s\n",
		runStart.Format("2006-01-02"),
		runEnd.Format("2006-01-02"),
	)

	aggregateAndStoreData(dConn, runStart, runEnd, selected)

	updateMaterializedViewsAndIndexes(pgConn)

	log.Printf("Done analyzing data, took %s", time.Since(startTime))
	return nil
}

func updateMaterializedViewsAndIndexes(pgConn *pgx.Conn) {
	ctx := context.Background()

	log.Print("Start refresh materialized view park_event_on_date")
	if _, err := pgConn.Exec(ctx, "REFRESH MATERIALIZED VIEW park_event_on_date"); err != nil {
		log.Fatalf("Failed to refresh materialized view park_event_on_date: %v", err)
	}
	log.Print("Finished refresh materialized view park_event_on_date")

	log.Print("Start refresh materialized view trip_on_date")
	if _, err := pgConn.Exec(ctx, "REFRESH MATERIALIZED VIEW trip_on_date"); err != nil {
		log.Fatalf("Failed to refresh materialized view trip_on_date: %v", err)
	}
	log.Print("Finished refresh materialized view trip_on_date")

	log.Print("DROP INDEX CONCURRENTLY IF EXISTS idx_park_events_location_recent_gist")
	if _, err := pgConn.Exec(ctx, "DROP INDEX CONCURRENTLY IF EXISTS idx_park_events_location_recent_gist;"); err != nil {
		log.Fatalf("Failed to drop index idx_park_events_location_recent_gist: %v", err)
	}
	log.Print("Finished DROP INDEX idx_park_events_location_recent_gist")

	log.Print("DROP INDEX CONCURRENTLY IF EXISTS park_events_ended_less_than_three_days_ago")
	if _, err := pgConn.Exec(ctx, "DROP INDEX CONCURRENTLY IF EXISTS park_events_ended_less_than_three_days_ago;"); err != nil {
		log.Fatalf("Failed to drop index park_events_ended_less_than_three_days_ago: %v", err)
	}
	log.Print("Finished DROP INDEX park_events_ended_less_than_three_days_ago")

	dateThreeDaysAgo := time.Now().UTC().AddDate(0, 0, -3).Format("2006-01-02")
	log.Printf("Create new index on park_events starting from_date: %s", dateThreeDaysAgo)

	stmt := fmt.Sprintf(`
		CREATE INDEX CONCURRENTLY park_events_ended_less_than_three_days_ago
		ON park_events (end_time)
		WHERE end_time >= '%s' OR end_time IS NULL;
	`, dateThreeDaysAgo)
	if _, err := pgConn.Exec(ctx, stmt); err != nil {
		log.Fatalf("Failed to create index park_events_ended_less_than_three_days_ago: %v", err)
	}

	stmt = fmt.Sprintf(`
		CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_park_events_location_recent_gist
		ON park_events
		USING gist (location)
		WHERE end_time >= '%s' OR end_time IS NULL;
	`, dateThreeDaysAgo)
	if _, err := pgConn.Exec(ctx, stmt); err != nil {
		log.Fatalf("Failed to create index idx_park_events_location_recent_gist: %v", err)
	}

	tripIndexes := []string{
		"idx_trips_bike_time_recent",
		"trips_ended_less_than_three_days_ago",
		"idx_trips_recent_start_gist",
		"idx_trips_recent_end_gist",
	}
	for _, index := range tripIndexes {
		log.Printf("DROP INDEX CONCURRENTLY IF EXISTS %s", index)
		if _, err := pgConn.Exec(ctx, "DROP INDEX CONCURRENTLY IF EXISTS "+index+";"); err != nil {
			log.Fatalf("Failed to drop index %s: %v", index, err)
		}
		log.Printf("Finished DROP INDEX %s", index)
	}

	log.Printf("Create new indexes on trips starting from_date: %s", dateThreeDaysAgo)

	stmt = fmt.Sprintf(`
		CREATE INDEX CONCURRENTLY trips_ended_less_than_three_days_ago
		ON trips (end_time)
		WHERE end_time >= '%s' OR end_time IS NULL;
	`, dateThreeDaysAgo)
	if _, err := pgConn.Exec(ctx, stmt); err != nil {
		log.Fatalf("Failed to create index trips_ended_less_than_three_days_ago: %v", err)
	}

	stmt = fmt.Sprintf(`
		CREATE INDEX CONCURRENTLY idx_trips_recent_start_gist
		ON trips
		USING gist (start_location)
		WHERE end_time >= '%s' OR end_time IS NULL;
	`, dateThreeDaysAgo)
	if _, err := pgConn.Exec(ctx, stmt); err != nil {
		log.Fatalf("Failed to create index idx_trips_recent_start_gist: %v", err)
	}

	stmt = fmt.Sprintf(`
		CREATE INDEX CONCURRENTLY idx_trips_recent_end_gist
		ON trips
		USING gist (end_location)
		WHERE end_time >= '%s' OR end_time IS NULL;
	`, dateThreeDaysAgo)
	if _, err := pgConn.Exec(ctx, stmt); err != nil {
		log.Fatalf("Failed to create index idx_trips_recent_end_gist: %v", err)
	}

	stmt = fmt.Sprintf(`
		CREATE INDEX CONCURRENTLY idx_trips_bike_time_recent
		ON trips (bike_id, start_time)
		WHERE start_time >= '%s';
	`, dateThreeDaysAgo)
	if _, err := pgConn.Exec(ctx, stmt); err != nil {
		log.Fatalf("Failed to create index idx_trips_bike_time_recent: %v", err)
	}

	log.Print("Finished creating indexes")
}

func determineDateRange(recalculate bool, selected []indicators.Indicator, from time.Time, to time.Time, pgConn *pgx.Conn) (time.Time, time.Time) {
	var requestedStart time.Time
	if recalculate {
		requestedStart = from
	} else {
		requestedStart = getNewestDateInMomentStatistics(pgConn).AddDate(0, 0, 1)
	}

	if to.Year() <= 1 {
		to = time.Now().Local().AddDate(0, 0, -1)
	}
	runStart := requestedStart

	for _, indicator := range selected {
		effectiveStart := indicators.EffectiveStartDate(indicator, requestedStart)
		if effectiveStart.Before(runStart) {
			runStart = effectiveStart
		}
	}

	return runStart, to
}

func aggregateAndStoreData(dConn *sql.DB, startDate time.Time, endDate time.Time, selected []indicators.Indicator) {
	const chunkSize = 30
	processedBatches := 0

	for start := startDate; !start.After(endDate); {
		end := start.AddDate(0, 0, chunkSize)
		if end.After(endDate) {
			end = endDate
		}

		fmt.Printf("Chunk: %s -> %s\n",
			start.Format("2006-01-02"),
			end.Format("2006-01-02"),
		)

		analyzeChunk(dConn, start, end, selected)
		start = end.AddDate(0, 0, 1)

		processedBatches += 1
		// Cleanup and write to Postgres every 5 batches
		if processedBatches%5 == 0 {
			analyze.AggregateVehiclesInPublicSpacePerDay(dConn, selected)
			analyze.ComputeTripsPerVehiclePerDay(dConn, selected)
			analyze.AggregateAvailableVehiclesPerDay(dConn, selected)
			writeToPostgres(dConn)
			cleanupTmpTables(dConn)
		}
	}
	analyze.AggregateVehiclesInPublicSpacePerDay(dConn, selected)
	analyze.ComputeTripsPerVehiclePerDay(dConn, selected)
	analyze.AggregateAvailableVehiclesPerDay(dConn, selected)
	writeToPostgres(dConn)
	cleanupTmpTables(dConn)
}

func analyzeChunk(dConn *sql.DB, startDate time.Time, endDate time.Time, selected []indicators.Indicator) {
	loadParkEventInBetween(dConn, startDate, endDate)
	loadNonOperationalEventsInBetween(dConn, startDate, endDate)
	loadTripsInBetween(dConn, startDate, endDate)
	analyze.FindIntersectionsWithZones(dConn)
	analyze.FindTripIntersectionsWithZones(dConn)

	for d := startDate; !d.After(endDate); d = d.AddDate(0, 0, 1) {
		log.Printf("Analyzing date %s", d.Format("2006-01-02"))
		analyzeDay(dConn, d, selected)
	}
	analyze.CountWronglyParkedVehicles(dConn, startDate, endDate, selected)
	analyze.AggregateWronglyParkedVehiclesPerDay(dConn, startDate, endDate, selected)
}

func analyzeDay(dConn *sql.DB, date time.Time, selected []indicators.Indicator) {
	// Set measurement moment to 03:30 on the given date
	measurementMoment := time.Date(
		date.Year(), date.Month(), date.Day(),
		03, 30, 0, 0,
		time.Local,
	)
	analyze.CountVehiclesInPublicSpaceForLongerThenXDays(dConn, measurementMoment, 1, selected)
	analyze.CountVehiclesInPublicSpaceForLongerThenXDays(dConn, measurementMoment, 3, selected)
	analyze.CountVehiclesInPublicSpaceForLongerThenXDays(dConn, measurementMoment, 7, selected)
	analyze.CountVehiclesInPublicSpaceForLongerThenXDays(dConn, measurementMoment, 14, selected)
	analyze.CountNonOperationalVehiclesLongerThen24Hours(dConn, measurementMoment, selected)
	analyze.CountNonOperationalVehiclesLongerThen7Days(dConn, measurementMoment, selected)
	analyze.CountVehiclesInPublicSpaceOnDate(dConn, measurementMoment, selected)
	analyze.CountTripsPerDay(dConn, date, selected)
	analyze.CountAvailableVehiclesInPublicSpace(dConn, date, selected)
}

func syncIndicatorsToPostgres(pgConn *pgx.Conn) {
	log.Print("Syncing indicators to Postgres...")
	stmt := `
		INSERT INTO indicators (id, text_id, description, first_day, updated_at)
		VALUES ($1, $2, $3, $4, NOW())
		ON CONFLICT (id) DO UPDATE SET
			text_id = EXCLUDED.text_id,
			description = EXCLUDED.description,
			first_day = EXCLUDED.first_day,
			updated_at = NOW();
	`
	for _, indicator := range indicators.All {
		_, err := pgConn.Exec(context.Background(), stmt, indicator.ID, indicator.TextID, indicator.Description, indicator.FirstDay)
		if err != nil {
			log.Fatalf("Failed to sync indicator %s: %v", indicator.TextID, err)
		}
	}
}

func deleteIndicatorData(pgConn *pgx.Conn, selected []indicators.Indicator, from time.Time, to time.Time) {
	ids := make([]int32, len(selected))
	idNames := make([]string, len(selected))
	for i, indicator := range selected {
		ids[i] = int32(indicator.ID)
		idNames[i] = indicator.TextID
	}

	log.Printf("Deleting existing data for indicators %s from %s to %s",
		strings.Join(idNames, ", "),
		from.Format("2006-01-02"),
		to.Format("2006-01-02"),
	)

	_, err := pgConn.Exec(context.Background(),
		"DELETE FROM moment_statistics WHERE indicator = ANY($1) AND date >= $2 AND date <= $3;",
		ids,
		from.Format("2006-01-02"),
		to.Format("2006-01-02"),
	)
	if err != nil {
		log.Fatalf("Failed to delete moment_statistics: %v", err)
	}

	_, err = pgConn.Exec(context.Background(),
		"DELETE FROM day_statistics WHERE indicator = ANY($1) AND date >= $2 AND date <= $3;",
		ids,
		from.Format("2006-01-02"),
		to.Format("2006-01-02"),
	)
	if err != nil {
		log.Fatalf("Failed to delete day_statistics: %v", err)
	}
}

func writeToPostgres(db *sql.DB) {
	log.Print("Writing results to Postgres...")
	stmt := `
	INSERT INTO postgres_db.moment_statistics
		(date, measurement_moment, indicator, geometry_ref, system_id, vehicle_type, trip_source, value)
	SELECT
		date,
		measurement_moment,
		indicator,
		geometry_ref,
		system_id,
		vehicle_type,
		trip_source,
		value
	FROM moment_statistics;
	`

	_, err := db.Exec(stmt)
	if err != nil {
		log.Fatal(err)
	}

	stmt = `
	INSERT INTO postgres_db.day_statistics
		(date, indicator, geometry_ref, system_id, vehicle_type, trip_source, value)
	SELECT
		date,
		indicator,
		geometry_ref,
		system_id,
		vehicle_type,
		trip_source,
		value
	FROM day_statistics;
	`

	_, err = db.Exec(stmt)
	if err != nil {
		log.Fatal(err)
	}
}

func cleanupTmpTables(db *sql.DB) {
	stmt := `
		TRUNCATE TABLE moment_statistics;
	`
	_, err := db.Exec(stmt)
	if err != nil {
		log.Fatal(err)
	}

	stmt2 := `
		TRUNCATE TABLE day_statistics;
	`
	_, err = db.Exec(stmt2)
	if err != nil {
		log.Fatal(err)
	}
}

func getNewestDateInMomentStatistics(db *pgx.Conn) time.Time {
	var newestDate pgtype.Date

	log.Print("Getting newest date in moment_statistics...")

	err := db.QueryRow(context.Background(), `
    SELECT COALESCE(MAX(date), DATE '2019-12-31')
    FROM moment_statistics;
`).Scan(&newestDate)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("%+v\n", newestDate)
	fmt.Println("Valid:", newestDate.Valid)
	fmt.Println("Time:", newestDate.Time)

	return newestDate.Time
}

func writeTmpTableToCSV(db *sql.DB) {
	_, err := db.Exec(`
		COPY moment_statistics TO 'moment_stats.csv'
		(HEADER, DELIMITER ',');
	`)
	if err != nil {
		log.Fatal(err)
	}

	_, err = db.Exec(`
		COPY day_statistics TO 'day_stats.csv'
		(HEADER, DELIMITER ',');
	`)
	if err != nil {
		log.Fatal(err)
	}
}
