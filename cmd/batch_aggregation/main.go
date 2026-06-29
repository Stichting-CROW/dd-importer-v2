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
		deleteIndicatorData(pgConn, selected)
	}

	loadZones(dConn)

	runStart, runEnd := determineDateRange(recalculate, selected, from, to, pgConn)
	if runStart.After(runEnd) {
		log.Print("Nothing to calculate for the selected indicators and date range.")
		return nil
	}

	fmt.Printf("Date range: %s -> %s\n",
		runStart.Format("2006-01-02"),
		runEnd.Format("2006-01-02"),
	)

	aggregateAndStoreData(dConn, runStart, runEnd, selected)

	log.Printf("Done analyzing data, took %s", time.Since(startTime))
	return nil
}

func determineDateRange(recalculate bool, selected []indicators.Indicator, from time.Time, to time.Time, pgConn *pgx.Conn) (time.Time, time.Time) {
	var requestedStart time.Time
	if recalculate {
		requestedStart = from
	} else {
		requestedStart = getNewestDateInMomentStatistics(pgConn).AddDate(0, 0, 1)
	}

	runStart := to
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
			writeToPostgres(dConn)
			cleanupTmpTables(dConn)
		}
	}
	analyze.AggregateVehiclesInPublicSpacePerDay(dConn, selected)
	writeToPostgres(dConn)
	cleanupTmpTables(dConn)
}

func analyzeChunk(dConn *sql.DB, startDate time.Time, endDate time.Time, selected []indicators.Indicator) {
	loadParkEventInBetween(dConn, startDate, endDate)
	//loadParkEventOnDate(dConn, date)
	analyze.FindIntersectionsWithZones(dConn)

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
	analyze.CountVehiclesInPublicSpaceOnDate(dConn, measurementMoment, selected)
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

func deleteIndicatorData(pgConn *pgx.Conn, selected []indicators.Indicator) {
	ids := make([]int32, len(selected))
	idNames := make([]string, len(selected))
	for i, indicator := range selected {
		ids[i] = int32(indicator.ID)
		idNames[i] = indicator.TextID
	}

	log.Printf("Deleting existing data for indicators: %s", strings.Join(idNames, ", "))

	_, err := pgConn.Exec(context.Background(),
		"DELETE FROM moment_statistics WHERE indicator = ANY($1);",
		ids,
	)
	if err != nil {
		log.Fatalf("Failed to delete moment_statistics: %v", err)
	}

	_, err = pgConn.Exec(context.Background(),
		"DELETE FROM day_statistics WHERE indicator = ANY($1);",
		ids,
	)
	if err != nil {
		log.Fatalf("Failed to delete day_statistics: %v", err)
	}
}

func writeToPostgres(db *sql.DB) {
	log.Print("Writing results to Postgres...")
	stmt := `
	INSERT INTO postgres_db.moment_statistics
	SELECT
		date,
		measurement_moment,
		indicator,
		geometry_ref,
		system_id,
		vehicle_type,
		value
	FROM moment_statistics;
	`

	_, err := db.Exec(stmt)
	if err != nil {
		log.Fatal(err)
	}

	stmt = `
	INSERT INTO postgres_db.day_statistics
	SELECT
		date,
		indicator,
		geometry_ref,
		system_id,
		vehicle_type,
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
	var newestDate time.Time
	log.Print("Getting newest date in moment_statistics...")
	err := db.QueryRow(context.Background(), `
		SELECT COALESCE(MAX(date), '2019-12-31'::DATE)
		FROM moment_statistics;
	`).Scan(&newestDate)
	if err != nil {
		log.Fatal(err)
	}
	return newestDate
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
