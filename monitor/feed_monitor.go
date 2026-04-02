package monitor

import (
	"database/sql"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/jmoiron/sqlx"
)

// FeedStatus represents the current status of a feed in the database
type FeedStatus struct {
	FeedID                      int          `db:"feed_id"`
	SystemID                    string       `db:"system_id"`
	IsActive                    bool         `db:"is_active"`
	ImportVehicles              bool         `db:"import_vehicles"`
	LastTimeSuccesfullyImported sql.NullTime `db:"last_time_succesfully_imported"`
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
	Type     string // "downtime", "recovery", "enabled", "disabled"
	SystemID string
	FeedID   int
	Duration string // For downtime/recovery
}

// FeedMonitor tracks feed uptime and manages notifications
type FeedMonitor struct {
	DB               *sqlx.DB
	Notifier         *TelegramNotifier
	LastCheck        map[int]FeedStatus
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
		notificationChan: make(chan Notification, 100), // Buffer up to 100 notifications
		stopChan:         make(chan bool),
	}
}

// MonitorFeeds continuously monitors all vehicle import feeds
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
			err = fm.Notifier.SendDowntimeAlert(notification.SystemID, notification.FeedID, notification.Duration)
		case "recovery":
			err = fm.Notifier.SendRecoveryAlert(notification.SystemID, notification.FeedID, notification.Duration)
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

		// Only check downtime for feeds that should import vehicles
		if feed.ImportVehicles && feed.IsActive {
			fm.checkDowntime(feed, currentTime)
		}

		// Update last check state
		fm.LastCheck[feed.FeedID] = feed
	}
}

// getActiveFeeds retrieves all feeds from the database
func (fm *FeedMonitor) getActiveFeeds() ([]FeedStatus, error) {
	stmt := `SELECT 
		feed_id, 
		system_id, 
		is_active, 
		import_vehicles,
		last_time_succesfully_imported
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
