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
	window := newAggregationWindow(runStart, runEnd, selected)

	if !window.runDaily && !window.runMonthly {
		log.Print("Nothing to calculate for the selected indicators and date range.")
		updateMaterializedViewsAndIndexes(pgConn)
		return nil
	}

	if window.runDaily {
		fmt.Printf("Daily range: %s -> %s\n",
			runStart.Format("2006-01-02"),
			runEnd.Format("2006-01-02"),
		)
	}
	if window.runMonthly {
		fmt.Printf("Monthly range: %s -> %s\n",
			window.monthStart.Format("2006-01"),
			window.monthEnd.Format("2006-01"),
		)
	}

	aggregateAndStoreData(dConn, window, selected)

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
		requestedStart = getNewestDateInMomentStatistics(pgConn).Local().AddDate(0, 0, 1)
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

type aggregationWindow struct {
	dailyStart time.Time
	dailyEnd   time.Time
	monthStart time.Time
	monthEnd   time.Time
	runDaily   bool
	runMonthly bool
}

func startOfMonth(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), 1, 0, 0, 0, 0, t.Location())
}

func newAggregationWindow(runStart time.Time, runEnd time.Time, selected []indicators.Indicator) aggregationWindow {
	loc := runEnd.Location()
	runStart = runStart.In(loc)
	runEnd = runEnd.In(loc)

	runDaily := indicators.HasDailyIndicators(selected) && !runStart.After(runEnd)
	runMonthly := indicators.HasMonthlyIndicators(selected)

	window := aggregationWindow{
		dailyStart: runStart,
		dailyEnd:   runEnd,
		runDaily:   runDaily,
		runMonthly: runMonthly,
	}

	if runMonthly {
		monthlyAnchor := runStart
		if monthlyAnchor.After(runEnd) {
			monthlyAnchor = runEnd
		}
		window.monthStart = startOfMonth(monthlyAnchor)
		window.monthEnd = startOfMonth(runEnd)
		if window.monthStart.After(window.monthEnd) {
			log.Printf("Invalid aggregation window: monthStart %s is after monthEnd %s",
				window.monthStart.Format("2006-01-02"),
				window.monthEnd.Format("2006-01-02"),
			)
		}
	} else if runDaily {
		window.monthStart = startOfMonth(runStart)
		window.monthEnd = startOfMonth(runEnd)
	}

	return window
}

func hasAnyIndicator(selected []indicators.Indicator, textIDs ...string) bool {
	for _, textID := range textIDs {
		if indicators.HasIndicator(selected, textID) {
			return true
		}
	}
	return false
}

func aggregateAndStoreData(dConn *sql.DB, window aggregationWindow, selected []indicators.Indicator) {
	if window.monthStart.After(window.monthEnd) {
		log.Printf("Skipping aggregation: monthStart %s is after monthEnd %s",
			window.monthStart.Format("2006-01-02"),
			window.monthEnd.Format("2006-01-02"),
		)
		return
	}

	const flushEveryMonths = 5
	processedMonths := 0
	flushMonths := []time.Time{}

	for monthStart := window.monthStart; !monthStart.After(window.monthEnd); monthStart = monthStart.AddDate(0, 1, 0) {
		monthEnd := monthStart.AddDate(0, 1, -1)
		monthLoadEnd := monthEnd
		if monthLoadEnd.After(window.dailyEnd) {
			monthLoadEnd = window.dailyEnd
		}

		dayStart := monthStart
		if dayStart.Before(window.dailyStart) {
			dayStart = window.dailyStart
		}
		dayEnd := monthEnd
		if dayEnd.After(window.dailyEnd) {
			dayEnd = window.dailyEnd
		}
		runDailyThisMonth := window.runDaily && !dayStart.After(dayEnd)

		log.Printf("Processing month %s", monthStart.Format("2006-01"))
		analyzeMonth(dConn, monthStart, monthLoadEnd, dayStart, dayEnd, runDailyThisMonth, window.runMonthly, selected)

		if window.runMonthly {
			flushMonths = append(flushMonths, monthStart)
		}

		processedMonths += 1
		if processedMonths%flushEveryMonths == 0 {
			flushStatistics(dConn, selected, flushMonths)
			flushMonths = nil
		}
	}

	flushStatistics(dConn, selected, flushMonths)
}

func selectedMonthlyIndicatorIDs(selected []indicators.Indicator) []int32 {
	ids := []int32{}
	for _, indicator := range selected {
		if indicators.IsMonthlyIndicator(indicator) {
			ids = append(ids, int32(indicator.ID))
		}
	}
	return ids
}

func flushStatistics(dConn *sql.DB, selected []indicators.Indicator, months []time.Time) {
	if len(months) > 0 {
		indicatorIDs := selectedMonthlyIndicatorIDs(selected)
		_, err := dConn.Exec("TRUNCATE TABLE monthly_flush_context;")
		if err != nil {
			log.Fatal(err)
		}
		for _, month := range months {
			for _, indicatorID := range indicatorIDs {
				_, err = dConn.Exec("INSERT INTO monthly_flush_context (month, indicator) VALUES ($1, $2);",
					month.Format("2006-01-02"), indicatorID)
				if err != nil {
					log.Fatal(err)
				}
			}
		}
	}

	analyze.AggregateVehiclesInPublicSpacePerDay(dConn, selected)
	analyze.ComputeTripsPerVehiclePerDay(dConn, selected)
	analyze.AggregateAvailableVehiclesPerDay(dConn, selected)
	writeToPostgres(dConn)
	cleanupTmpTables(dConn)
}

func analyzeMonth(dConn *sql.DB, monthStart time.Time, monthLoadEnd time.Time, dayStart time.Time, dayEnd time.Time, runDaily bool, runMonthly bool, selected []indicators.Indicator) {
	wronglyParkedSelected := runDaily && indicators.HasIndicator(selected, "count_wrongly_parked_vehicles")

	needsParkEvents := runDaily && (wronglyParkedSelected || hasAnyIndicator(selected,
		"count_vehicles_in_public_space",
		"count_vehicles_in_public_space_longer_then_1_days",
		"count_vehicles_in_public_space_longer_then_3_days",
		"count_vehicles_in_public_space_longer_then_7_days",
		"count_vehicles_in_public_space_longer_then_14_days",
		"count_vehicles_non_operational_longer_then_24_hours",
		"count_vehicles_non_operational_longer_then_7_days",
		"rentals_per_vehicle_per_day",
		"available_vehicles_in_public_space",
	))

	needsNonOperationalEvents := runDaily && hasAnyIndicator(selected,
		"count_vehicles_non_operational_longer_then_24_hours",
		"count_vehicles_non_operational_longer_then_7_days",
		"available_vehicles_in_public_space",
	)

	needsTrips := runMonthly || (runDaily && hasAnyIndicator(selected,
		"count_rentals_per_day",
		"rentals_per_vehicle_per_day",
		"available_vehicles_in_public_space",
		"rented_out_vehicles_available_same_day",
	))

	if needsParkEvents {
		loadParkEventInBetween(dConn, dayStart, monthLoadEnd)
	}
	if needsNonOperationalEvents {
		loadNonOperationalEventsInBetween(dConn, dayStart, monthLoadEnd)
	}
	if needsTrips {
		tripLoadStart := monthStart
		if !runMonthly {
			tripLoadStart = dayStart
		}
		loadTripsInBetween(dConn, tripLoadStart, monthLoadEnd)
	}
	if needsParkEvents {
		analyze.FindIntersectionsWithZones(dConn)
	}
	if needsTrips {
		analyze.FindTripIntersectionsWithZones(dConn)
	}

	if runDaily {
		for d := dayStart; !d.After(dayEnd); d = d.AddDate(0, 0, 1) {
			log.Printf("Analyzing date %s", d.Format("2006-01-02"))
			analyzeDay(dConn, d, selected)
		}
		if wronglyParkedSelected {
			analyze.CountWronglyParkedVehicles(dConn, dayStart, dayEnd, selected)
			analyze.AggregateWronglyParkedVehiclesPerDay(dConn, dayStart, dayEnd, selected)
		}
	}

	if runMonthly {
		computeMonthlyStatisticsForMonth(dConn, monthStart, selected)
	}
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
	analyze.CountRentedOutVehiclesAvailableSameDay(dConn, date, selected)
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

	var flushContextCount int
	err = db.QueryRow("SELECT COUNT(*) FROM monthly_flush_context;").Scan(&flushContextCount)
	if err != nil {
		log.Fatal(err)
	}

	if flushContextCount > 0 {
		stmt = `
		DELETE FROM postgres_db.month_statistics
		WHERE indicator IN (SELECT DISTINCT indicator FROM monthly_flush_context)
			AND month IN (SELECT DISTINCT month FROM monthly_flush_context);
		`
		_, err = db.Exec(stmt)
		if err != nil {
			log.Fatal(err)
		}

		stmt = `
		INSERT INTO postgres_db.month_statistics
			(month, indicator, geometry_ref, system_id, vehicle_type, trip_source, category, time_period, value)
		SELECT
			month,
			indicator,
			geometry_ref,
			system_id,
			vehicle_type,
			trip_source,
			category,
			time_period,
			value
		FROM month_statistics;
		`

		_, err = db.Exec(stmt)
		if err != nil {
			log.Fatal(err)
		}
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

	stmt3 := `
		TRUNCATE TABLE month_statistics;
	`
	_, err = db.Exec(stmt3)
	if err != nil {
		log.Fatal(err)
	}

	stmt4 := `
		TRUNCATE TABLE monthly_flush_context;
	`
	_, err = db.Exec(stmt4)
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
