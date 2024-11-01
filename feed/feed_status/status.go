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
