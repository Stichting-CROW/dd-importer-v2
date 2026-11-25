package monitor

import (
	"database/sql"
	"fmt"
	"log"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/jmoiron/sqlx"
)

// Values of feeds.data_type.
const (
	dataTypeVehicles     = "vehicles"
	dataTypeTrips        = "trips"
	dataTypeServiceAreas = "service_areas"
)

// scheduledImportThresholds holds, for the data types that are imported by an
// hourly job, how long a feed may go without a successful import before it
// counts as down. Two runs plus a margin, so one failed run opens no downtime.
var scheduledImportThresholds = map[string]time.Duration{
	dataTypeTrips:        150 * time.Minute,
	dataTypeServiceAreas: 150 * time.Minute,
}

// alertDataTypesEnv names the data types, besides vehicles, whose downtime and
// recovery are also sent to Telegram (comma separated). Downtime is recorded
// in feed_downtime either way.
const alertDataTypesEnv = "TELEGRAM_ALERT_DATA_TYPES"

// FeedStatus represents the current status of a feed in the database
type FeedStatus struct {
	FeedID                      int          `db:"feed_id"`
	SystemID                    string       `db:"system_id"`
	IsActive                    bool         `db:"is_active"`
	DataType                    string       `db:"data_type"`
	LastTimeSuccesfullyImported sql.NullTime `db:"last_time_succesfully_imported"`
	LastTimeImportAttempted     sql.NullTime `db:"last_time_import_attempted"`
	ImportFailingSince          sql.NullTime `db:"import_failing_since"`
}

// DowntimeRecord represents a downtime entry in the database
type DowntimeRecord struct {
	DowntimeID               int            `db:"downtime_id"`
	FeedID                   int            `db:"feed_id"`
	DowntimeStart            time.Time      `db:"downtime_start"`
	DowntimeEnd              sql.NullTime   `db:"downtime_end"`
	Reason                   sql.NullString `db:"reason"`
	NotificationSent         bool           `db:"notification_sent"`
	RecoveryNotificationSent bool           `db:"recovery_notification_sent"`
}

// Notification represents a notification to be sent
type Notification struct {
	Type     string // "downtime", "recovery", "enabled", "disabled", "importer_stalled"
	SystemID string
	FeedID   int
	DataType string // For importer_stalled, and downtime/recovery of other feeds than vehicles
	Duration string // For downtime/recovery/importer_stalled
}

// FeedMonitor tracks feed uptime and manages notifications
type FeedMonitor struct {
	DB               *sqlx.DB
	Notifier         *TelegramNotifier
	LastCheck        map[int]FeedStatus
	alertDataTypes   map[string]bool
	importerStalled  map[string]bool
	notificationChan chan Notification
	stopChan         chan bool
	wg               sync.WaitGroup
}

// NewFeedMonitor creates a new feed monitor
func NewFeedMonitor(db *sqlx.DB, notifier *TelegramNotifier) *FeedMonitor {
	return &FeedMonitor{
		DB:               db,
		Notifier:         notifier,
		LastCheck:        make(map[int]FeedStatus),
		alertDataTypes:   parseDataTypes(os.Getenv(alertDataTypesEnv)),
		importerStalled:  make(map[string]bool),
		notificationChan: make(chan Notification, 100), // Buffer up to 100 notifications
		stopChan:         make(chan bool),
	}
}

// MonitorFeeds continuously monitors all feeds
func (fm *FeedMonitor) MonitorFeeds() {
	log.Println("Starting feed monitoring loop...")

	// Start notification processor in background
	fm.wg.Add(1)
	go fm.notificationProcessor()

	for {
		select {
		case <-fm.stopChan:
			log.Println("Stopping feed monitor...")
			close(fm.notificationChan)
			fm.wg.Wait()
			return
		default:
			start := time.Now()
			fm.checkFeeds()

			// Sleep for approximately 1 minute, accounting for check duration
			duration := time.Since(start)
			if duration < time.Minute {
				time.Sleep(time.Minute - duration)
			}
		}
	}
}

// notificationProcessor processes notifications from the queue at a controlled rate
func (fm *FeedMonitor) notificationProcessor() {
	defer fm.wg.Done()

	log.Println("Notification processor started")

	for notification := range fm.notificationChan {
		var err error

		switch notification.Type {
		case "downtime":
			err = fm.Notifier.SendDowntimeAlert(notification.SystemID, notification.FeedID, notification.DataType, notification.Duration)
		case "recovery":
			err = fm.Notifier.SendRecoveryAlert(notification.SystemID, notification.FeedID, notification.DataType, notification.Duration)
		case "importer_stalled":
			err = fm.Notifier.SendImporterStalledAlert(notification.DataType, notification.Duration)
		case "enabled":
			err = fm.Notifier.SendNewFeedAlert(notification.SystemID, notification.FeedID)
		case "disabled":
			err = fm.Notifier.SendFeedDisabledAlert(notification.SystemID, notification.FeedID)
		}

		if err != nil {
			log.Printf("Error sending %s notification for feed %d: %v", notification.Type, notification.FeedID, err)
		} else {
			log.Printf("%s notification sent for feed %s (ID: %d)", notification.Type, notification.SystemID, notification.FeedID)
		}
	}

	log.Println("Notification processor stopped")
}

// queueNotification adds a notification to the queue
func (fm *FeedMonitor) queueNotification(notification Notification) {
	select {
	case fm.notificationChan <- notification:
		// Successfully queued
	default:
		// Channel is full, log and drop
		log.Printf("Warning: Notification queue is full, dropping %s notification for feed %d", 
			notification.Type, notification.FeedID)
	}
}

// checkFeeds performs a single check of all feeds
func (fm *FeedMonitor) checkFeeds() {
	feeds, err := fm.getActiveFeeds()
	if err != nil {
		log.Printf("Error fetching feeds: %v", err)
		return
	}

	currentTime := time.Now()

	for _, feed := range feeds {
		// Check for status changes (is_active changes)
		if prevFeed, exists := fm.LastCheck[feed.FeedID]; exists {
			if prevFeed.IsActive != feed.IsActive {
				if feed.IsActive {
					// Feed was enabled
					fm.queueNotification(Notification{
						Type:     "enabled",
						SystemID: feed.SystemID,
						FeedID:   feed.FeedID,
					})
					log.Printf("Feed %s (ID: %d) has been enabled", feed.SystemID, feed.FeedID)
				} else {
					// Feed was disabled
					fm.queueNotification(Notification{
						Type:     "disabled",
						SystemID: feed.SystemID,
						FeedID:   feed.FeedID,
					})
					log.Printf("Feed %s (ID: %d) has been disabled", feed.SystemID, feed.FeedID)

					// If there was an ongoing downtime, close it
					fm.closeOngoingDowntime(feed.FeedID, currentTime, "Feed disabled")
				}
			}
		}

		if feed.IsActive {
			if feed.DataType == dataTypeVehicles {
				fm.checkDowntime(feed, currentTime)
			} else if threshold, ok := scheduledImportThresholds[feed.DataType]; ok {
				fm.checkScheduledImport(feed, currentTime, threshold)
			}
		}

		// Update last check state
		fm.LastCheck[feed.FeedID] = feed
	}

	fm.checkImportersRunning(feeds, currentTime)
}

// importState is what the monitor concludes about a feed that is imported by
// a scheduled job.
type importState int

const (
	// importStateUnknown: no conclusion, downtime records are left as they are.
	importStateUnknown importState = iota
	importStateUp
	importStateDown
)

// scheduledImportState decides whether a trips or service area feed is up or
// down. When it is down, the second result is the start of the downtime: the
// first failed attempt since the last successful import.
//
// The state is unknown when the feed was never attempted, when the last
// attempt is older than the threshold (the importer is not running, which is
// not the feed's fault) and while a failing feed is still within the threshold.
func scheduledImportState(feed FeedStatus, now time.Time, threshold time.Duration) (importState, time.Time) {
	attempted := feed.LastTimeImportAttempted
	if !attempted.Valid || now.Sub(attempted.Time) > threshold {
		return importStateUnknown, time.Time{}
	}

	if !feed.ImportFailingSince.Valid {
		return importStateUp, time.Time{}
	}

	failingSince := feed.ImportFailingSince.Time
	lastGood := failingSince
	if feed.LastTimeSuccesfullyImported.Valid {
		lastGood = feed.LastTimeSuccesfullyImported.Time
	}
	if now.Sub(lastGood) > threshold {
		return importStateDown, failingSince
	}
	return importStateUnknown, time.Time{}
}

// checkScheduledImport manages downtime records for a trips or service area feed
func (fm *FeedMonitor) checkScheduledImport(feed FeedStatus, currentTime time.Time, threshold time.Duration) {
	state, downSince := scheduledImportState(feed, currentTime, threshold)
	if state == importStateUnknown {
		return
	}

	ongoingDowntime, err := fm.getOngoingDowntime(feed.FeedID)
	if err != nil {
		log.Printf("Error checking ongoing downtime for feed %d: %v", feed.FeedID, err)
		return
	}

	switch {
	case state == importStateDown && ongoingDowntime == nil:
		downtimeID, err := fm.createDowntimeRecord(feed.FeedID, downSince,
			fmt.Sprintf("No successful %s import since %s", feed.DataType, downSince.UTC().Format(time.RFC3339)))
		if err != nil {
			log.Printf("Error creating downtime record for feed %d: %v", feed.FeedID, err)
			return
		}
		log.Printf("Downtime recorded for %s feed %s (ID: %d)", feed.DataType, feed.SystemID, feed.FeedID)

		if fm.alertDataTypes[feed.DataType] {
			fm.queueNotification(Notification{
				Type:     "downtime",
				SystemID: feed.SystemID,
				FeedID:   feed.FeedID,
				DataType: feed.DataType,
				Duration: formatDuration(currentTime.Sub(downSince)),
			})
			fm.markNotificationSent(downtimeID)
		}

	case state == importStateUp && ongoingDowntime != nil:
		if err := fm.closeDowntimeRecord(ongoingDowntime.DowntimeID, currentTime); err != nil {
			log.Printf("Error closing downtime record %d: %v", ongoingDowntime.DowntimeID, err)
			return
		}
		downtimeDuration := currentTime.Sub(ongoingDowntime.DowntimeStart)
		log.Printf("Downtime ended for %s feed %s (ID: %d), downtime was %v",
			feed.DataType, feed.SystemID, feed.FeedID, formatDuration(downtimeDuration))

		if fm.alertDataTypes[feed.DataType] {
			fm.queueNotification(Notification{
				Type:     "recovery",
				SystemID: feed.SystemID,
				FeedID:   feed.FeedID,
				DataType: feed.DataType,
				Duration: formatDuration(downtimeDuration),
			})
			fm.markRecoveryNotificationSent(ongoingDowntime.DowntimeID)
		}
	}
}

// checkImportersRunning sends one alert when the scheduled importer of a data
// type stops attempting imports. Its feeds are not marked as down meanwhile.
func (fm *FeedMonitor) checkImportersRunning(feeds []FeedStatus, currentTime time.Time) {
	for dataType, threshold := range scheduledImportThresholds {
		latestAttempt, ok := latestImportAttempt(feeds, dataType)
		if !ok {
			continue
		}

		stalled := currentTime.Sub(latestAttempt) > threshold
		if stalled && !fm.importerStalled[dataType] {
			fm.queueNotification(Notification{
				Type:     "importer_stalled",
				DataType: dataType,
				Duration: formatDuration(currentTime.Sub(latestAttempt)),
			})
			log.Printf("The %s importer made no import attempt since %s",
				dataType, latestAttempt.UTC().Format(time.RFC3339))
		} else if !stalled && fm.importerStalled[dataType] {
			log.Printf("The %s importer is running again", dataType)
		}
		fm.importerStalled[dataType] = stalled
	}
}

// latestImportAttempt returns the most recent import attempt over the active
// feeds of a data type. The second result is false when none was attempted yet.
func latestImportAttempt(feeds []FeedStatus, dataType string) (time.Time, bool) {
	var latest time.Time
	found := false
	for _, feed := range feeds {
		if feed.DataType != dataType || !feed.IsActive || !feed.LastTimeImportAttempted.Valid {
			continue
		}
		if !found || feed.LastTimeImportAttempted.Time.After(latest) {
			latest = feed.LastTimeImportAttempted.Time
			found = true
		}
	}
	return latest, found
}

// parseDataTypes turns a comma separated list of data types into a set
func parseDataTypes(list string) map[string]bool {
	dataTypes := make(map[string]bool)
	for _, dataType := range strings.Split(list, ",") {
		if dataType = strings.TrimSpace(dataType); dataType != "" {
			dataTypes[dataType] = true
		}
	}
	return dataTypes
}

// getActiveFeeds retrieves all feeds from the database
func (fm *FeedMonitor) getActiveFeeds() ([]FeedStatus, error) {
	stmt := `SELECT 
		feed_id, 
		system_id, 
		is_active, 
		data_type,
		last_time_succesfully_imported,
		last_time_import_attempted,
		import_failing_since
		FROM feeds
		ORDER BY feed_id`

	var feeds []FeedStatus
	err := fm.DB.Select(&feeds, stmt)
	if err != nil {
		return nil, fmt.Errorf("failed to query feeds: %w", err)
	}

	return feeds, nil
}

// checkDowntime evaluates if a feed is down and manages downtime records
func (fm *FeedMonitor) checkDowntime(feed FeedStatus, currentTime time.Time) {
	// Calculate how long since last successful import
	var timeSinceLastImport time.Duration
	if feed.LastTimeSuccesfullyImported.Valid {
		timeSinceLastImport = currentTime.Sub(feed.LastTimeSuccesfullyImported.Time)
	} else {
		// No successful import ever recorded - treat as down
		timeSinceLastImport = time.Hour * 24 // Assume 24 hours
	}

	// Check if there's an ongoing downtime record
	ongoingDowntime, err := fm.getOngoingDowntime(feed.FeedID)
	if err != nil {
		log.Printf("Error checking ongoing downtime for feed %d: %v", feed.FeedID, err)
		return
	}

	// 5-minute threshold for considering a feed down
	threshold := 5 * time.Minute

	if timeSinceLastImport > threshold {
		// Feed is down
		if ongoingDowntime == nil {
			// New downtime detected - create record and queue alert
			downtimeID, err := fm.createDowntimeRecord(feed.FeedID, currentTime,
				fmt.Sprintf("No successful import for %v", timeSinceLastImport.Round(time.Second)))
			if err != nil {
				log.Printf("Error creating downtime record for feed %d: %v", feed.FeedID, err)
				return
			}

			// Queue notification
			fm.queueNotification(Notification{
				Type:     "downtime",
				SystemID: feed.SystemID,
				FeedID:   feed.FeedID,
				Duration: formatDuration(timeSinceLastImport),
			})
			
			// Mark notification as queued (will be updated to sent after actual send)
			fm.markNotificationSent(downtimeID)
			log.Printf("Downtime alert queued for feed %s (ID: %d)", feed.SystemID, feed.FeedID)
		}
	} else {
		// Feed is up
		if ongoingDowntime != nil {
			// Downtime has ended - update record and queue recovery notification
			downtimeDuration := currentTime.Sub(ongoingDowntime.DowntimeStart)

			err := fm.closeDowntimeRecord(ongoingDowntime.DowntimeID, currentTime)
			if err != nil {
				log.Printf("Error closing downtime record %d: %v", ongoingDowntime.DowntimeID, err)
				return
			}

			// Queue recovery notification
			fm.queueNotification(Notification{
				Type:     "recovery",
				SystemID: feed.SystemID,
				FeedID:   feed.FeedID,
				Duration: formatDuration(downtimeDuration),
			})
			
			fm.markRecoveryNotificationSent(ongoingDowntime.DowntimeID)
			log.Printf("Recovery alert queued for feed %s (ID: %d), downtime was %v",
				feed.SystemID, feed.FeedID, formatDuration(downtimeDuration))
		}
	}
}

// getOngoingDowntime retrieves an active (unended) downtime record for a feed
func (fm *FeedMonitor) getOngoingDowntime(feedID int) (*DowntimeRecord, error) {
	stmt := `SELECT 
		downtime_id, feed_id, downtime_start, downtime_end, 
		reason, notification_sent, recovery_notification_sent
		FROM feed_downtime 
		WHERE feed_id = $1 
		AND downtime_end IS NULL 
		ORDER BY downtime_start DESC 
		LIMIT 1`

	var record DowntimeRecord
	err := fm.DB.Get(&record, stmt, feedID)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &record, nil
}

// createDowntimeRecord creates a new downtime record
func (fm *FeedMonitor) createDowntimeRecord(feedID int, startTime time.Time, reason string) (int, error) {
	stmt := `INSERT INTO feed_downtime 
		(feed_id, downtime_start, reason, notification_sent, recovery_notification_sent)
		VALUES ($1, $2, $3, false, false)
		RETURNING downtime_id`

	var downtimeID int
	err := fm.DB.QueryRow(stmt, feedID, startTime, reason).Scan(&downtimeID)
	if err != nil {
		return 0, err
	}
	return downtimeID, nil
}

// closeDowntimeRecord marks a downtime record as ended
func (fm *FeedMonitor) closeDowntimeRecord(downtimeID int, endTime time.Time) error {
	stmt := `UPDATE feed_downtime 
		SET downtime_end = $1 
		WHERE downtime_id = $2`

	_, err := fm.DB.Exec(stmt, endTime, downtimeID)
	return err
}

// closeOngoingDowntime closes any ongoing downtime for a feed with a specific reason
func (fm *FeedMonitor) closeOngoingDowntime(feedID int, endTime time.Time, reason string) error {
	stmt := `UPDATE feed_downtime 
		SET downtime_end = $1, reason = COALESCE(reason, '') || ' | ' || $2
		WHERE feed_id = $3 
		AND downtime_end IS NULL`

	_, err := fm.DB.Exec(stmt, endTime, reason, feedID)
	return err
}

// markNotificationSent marks the initial downtime notification as sent
func (fm *FeedMonitor) markNotificationSent(downtimeID int) error {
	stmt := `UPDATE feed_downtime SET notification_sent = true WHERE downtime_id = $1`
	_, err := fm.DB.Exec(stmt, downtimeID)
	return err
}

// markRecoveryNotificationSent marks the recovery notification as sent
func (fm *FeedMonitor) markRecoveryNotificationSent(downtimeID int) error {
	stmt := `UPDATE feed_downtime SET recovery_notification_sent = true WHERE downtime_id = $1`
	_, err := fm.DB.DB.Exec(stmt, downtimeID)
	return err
}

// formatDuration formats a duration in a human-readable way
func formatDuration(d time.Duration) string {
	if d < time.Minute {
		return fmt.Sprintf("%d seconds", int(d.Seconds()))
	} else if d < time.Hour {
		return fmt.Sprintf("%d minutes", int(d.Minutes()))
	} else {
		hours := int(d.Hours())
		minutes := int(d.Minutes()) % 60
		if minutes > 0 {
			return fmt.Sprintf("%dh %dm", hours, minutes)
		}
		return fmt.Sprintf("%d hours", hours)
	}
}
