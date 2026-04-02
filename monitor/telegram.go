package monitor

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strconv"
	"sync"
	"time"
)

// Notifier interface for sending notifications
type Notifier interface {
	SendAlert(message string) error
}

// TelegramNotifier sends notifications via Telegram Bot API with rate limiting
type TelegramNotifier struct {
	BotToken     string
	ChatID       string
	Client       *http.Client
	rateLimiter  *time.Ticker
	mu           sync.Mutex
	lastSendTime time.Time
	minInterval  time.Duration
}

// NewTelegramNotifier creates a new Telegram notifier from environment variables
func NewTelegramNotifier() (*TelegramNotifier, error) {
	botToken := os.Getenv("TELEGRAM_BOT_TOKEN")
	chatID := os.Getenv("TELEGRAM_CHAT_ID")

	if botToken == "" {
		return nil, fmt.Errorf("TELEGRAM_BOT_TOKEN environment variable not set")
	}
	if chatID == "" {
		return nil, fmt.Errorf("TELEGRAM_CHAT_ID environment variable not set")
	}

	// Telegram rate limit: ~30 messages per second to the same chat
	// We'll be conservative and use 1 message per second
	minInterval := 1 * time.Second
	if interval := os.Getenv("TELEGRAM_RATE_LIMIT_MS"); interval != "" {
		if ms, err := strconv.Atoi(interval); err == nil && ms > 0 {
			minInterval = time.Duration(ms) * time.Millisecond
		}
	}

	return &TelegramNotifier{
		BotToken: botToken,
		ChatID:   chatID,
		Client: &http.Client{
			Timeout: 30 * time.Second,
		},
		lastSendTime: time.Time{},
		minInterval:  minInterval,
	}, nil
}

// SendAlert sends a message to the configured Telegram chat with retry logic
func (t *TelegramNotifier) SendAlert(message string) error {
	// Rate limiting: ensure we don't send too fast
	t.mu.Lock()
	timeSinceLastSend := time.Since(t.lastSendTime)
	if timeSinceLastSend < t.minInterval {
		sleepDuration := t.minInterval - timeSinceLastSend
		t.mu.Unlock()
		log.Printf("Rate limiting: sleeping for %v before sending Telegram message", sleepDuration)
		time.Sleep(sleepDuration)
		t.mu.Lock()
	}
	t.lastSendTime = time.Now()
	t.mu.Unlock()

	url := fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage", t.BotToken)

	payload := map[string]string{
		"chat_id":    t.ChatID,
		"text":       message,
		"parse_mode": "HTML",
	}

	jsonPayload, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal telegram payload: %w", err)
	}

	// Retry logic with exponential backoff
	maxRetries := 3
	baseDelay := 1 * time.Second

	for attempt := 0; attempt < maxRetries; attempt++ {
		if attempt > 0 {
			delay := baseDelay * time.Duration(1<<uint(attempt-1))
			log.Printf("Retrying Telegram send (attempt %d/%d) after %v", attempt+1, maxRetries, delay)
			time.Sleep(delay)
		}

		resp, err := t.Client.Post(url, "application/json", bytes.NewBuffer(jsonPayload))
		if err != nil {
			if attempt == maxRetries-1 {
				return fmt.Errorf("failed to send telegram message after %d attempts: %w", maxRetries, err)
			}
			log.Printf("Telegram send error (attempt %d/%d): %v", attempt+1, maxRetries, err)
			continue
		}

		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()

		// Check for rate limiting (429)
		if resp.StatusCode == http.StatusTooManyRequests {
			// Try to parse retry_after from response
			retryAfter := t.extractRetryAfter(body)
			if retryAfter > 0 {
				log.Printf("Telegram rate limit hit (429), waiting %d seconds before retry", retryAfter)
				time.Sleep(time.Duration(retryAfter) * time.Second)
			} else {
				// If no retry_after provided, use exponential backoff
				delay := baseDelay * time.Duration(1<<uint(attempt))
				log.Printf("Telegram rate limit hit (429), waiting %v before retry", delay)
				time.Sleep(delay)
			}
			continue
		}

		// Check for other errors
		if resp.StatusCode != http.StatusOK {
			// Log the error response for debugging
			log.Printf("Telegram API error (status %d): %s", resp.StatusCode, string(body))
			
			// Some errors shouldn't be retried
			if resp.StatusCode == http.StatusBadRequest || 
			   resp.StatusCode == http.StatusUnauthorized || 
			   resp.StatusCode == http.StatusForbidden {
				return fmt.Errorf("telegram API error (status %d): %s", resp.StatusCode, string(body))
			}
			
			if attempt == maxRetries-1 {
				return fmt.Errorf("telegram API returned non-OK status after %d attempts: %d (body: %s)", 
					maxRetries, resp.StatusCode, string(body))
			}
			continue
		}

		// Success!
		return nil
	}

	return fmt.Errorf("exhausted all retries sending telegram message")
}

// extractRetryAfter attempts to extract the retry_after value from Telegram's 429 response
func (t *TelegramNotifier) extractRetryAfter(body []byte) int {
	var response struct {
		OK          bool   `json:"ok"`
		ErrorCode   int    `json:"error_code"`
		Description string `json:"description"`
		Parameters  struct {
			RetryAfter int `json:"retry_after"`
		} `json:"parameters"`
	}

	if err := json.Unmarshal(body, &response); err != nil {
		return 0
	}

	if response.Parameters.RetryAfter > 0 {
		return response.Parameters.RetryAfter
	}

	return 0
}

// SendDowntimeAlert sends a formatted downtime alert
func (t *TelegramNotifier) SendDowntimeAlert(systemID string, feedID int, duration string) error {
	message := fmt.Sprintf(
		"⚠️ <b>Feed Down Alert</b>\n\n"+
			"<b>System:</b> %s\n"+
			"<b>Feed ID:</b> %d\n"+
			"<b>Status:</b> DOWN\n"+
			"<b>Duration:</b> %s",
		systemID, feedID, duration,
	)
	return t.SendAlert(message)
}

// SendRecoveryAlert sends a formatted recovery alert
func (t *TelegramNotifier) SendRecoveryAlert(systemID string, feedID int, downtime string) error {
	message := fmt.Sprintf(
		"✅ <b>Feed Recovered</b>\n\n"+
			"<b>System:</b> %s\n"+
			"<b>Feed ID:</b> %d\n"+
			"<b>Status:</b> UP\n"+
			"<b>Total Downtime:</b> %s",
		systemID, feedID, downtime,
	)
	return t.SendAlert(message)
}

// SendNewFeedAlert sends a formatted new feed enabled alert
func (t *TelegramNotifier) SendNewFeedAlert(systemID string, feedID int) error {
	message := fmt.Sprintf(
		"🆕 <b>New Feed Enabled</b>\n\n"+
			"<b>System:</b> %s\n"+
			"<b>Feed ID:</b> %d\n"+
			"<b>Status:</b> ACTIVE",
		systemID, feedID,
	)
	return t.SendAlert(message)
}

// SendFeedDisabledAlert sends a formatted feed disabled alert
func (t *TelegramNotifier) SendFeedDisabledAlert(systemID string, feedID int) error {
	message := fmt.Sprintf(
		"🚫 <b>Feed Disabled</b>\n\n"+
			"<b>System:</b> %s\n"+
			"<b>Feed ID:</b> %d\n"+
			"<b>Status:</b> INACTIVE",
		systemID, feedID,
	)
	return t.SendAlert(message)
}
