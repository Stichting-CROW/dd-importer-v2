package feed_status

import (
	"log"

	"github.com/jmoiron/sqlx"
)

func UpdateLastTimeSuccesfullyImported(feedIds []int, db *sqlx.DB) {

	stmt := `UPDATE feeds
		SET last_time_succesfully_imported = NOW(),
		ignore_disruptions_in_feed_until = NULL
		WHERE feed_id IN (?)`
	query, args, err := sqlx.In(stmt, feedIds)
	query = db.Rebind(query)
	if err != nil {
		log.Print(err)
	}

	_, err = db.Exec(query, args...)
	if err != nil {
		log.Print(err)
	}
}

// RecordImportSuccess stores that a feed was requested just now and delivered
// usable data. Used by the importers that run as a scheduled job (trips and
// service areas), so the feed monitor can tell a failing feed from an importer
// that is not running.
func RecordImportSuccess(feedID int, db *sqlx.DB) {
	stmt := `UPDATE feeds
		SET last_time_import_attempted = NOW(),
		last_time_succesfully_imported = NOW(),
		import_failing_since = NULL
		WHERE feed_id = $1`
	if _, err := db.Exec(stmt, feedID); err != nil {
		log.Printf("Could not record import success for feed %d: %v", feedID, err)
	}
}

// RecordImportFailure stores that a feed was requested just now and did not
// deliver usable data. import_failing_since keeps the first failed attempt
// since the last success.
func RecordImportFailure(feedID int, db *sqlx.DB) {
	stmt := `UPDATE feeds
		SET last_time_import_attempted = NOW(),
		import_failing_since = COALESCE(import_failing_since, NOW())
		WHERE feed_id = $1`
	if _, err := db.Exec(stmt, feedID); err != nil {
		log.Printf("Could not record import failure for feed %d: %v", feedID, err)
	}
}
