# Feed Monitor

A monitoring service that tracks feed uptime and sends Telegram notifications when feeds go down or come back up.

## Features

- **Downtime Detection**: Monitors vehicle import feeds and detects when they haven't successfully imported for more than 5 minutes
- **Status Change Notifications**: Alerts when feeds are enabled or disabled in the database
- **Recovery Tracking**: Sends notifications when feeds recover from downtime
- **Persistent History**: All downtime events are stored indefinitely in `feed_downtime` table
- **Minute-by-minute Monitoring**: Checks all feeds every minute
- **Rate Limiting**: Built-in protection against Telegram API 429 (Too Many Requests) errors
- **Retry Logic**: Automatic retry with exponential backoff on API failures
- **Notification Queue**: Buffers notifications to prevent flooding

## How It Works

1. Queries the `feeds` table every minute for all feeds with `import_vehicles = true`
2. Checks `last_time_succesfully_imported` timestamp for each feed
3. If more than 5 minutes have passed since last successful import:
   - Creates a record in `feed_downtime` table
   - Queues Telegram alert with system ID and feed ID
4. Notifications are processed by a background worker that:
   - Sends at most 1 message per second (configurable)
   - Retries on 429 errors with exponential backoff
   - Respects Telegram's `retry_after` header when rate limited
5. When feed recovers (successful import detected):
   - Closes the downtime record
   - Sends recovery notification with total downtime duration
6. Detects when feeds change `is_active` status:
   - Sends notification when feed is enabled (🆕)
   - Sends notification when feed is disabled (🚫)

## Architecture

### Notification Queue System
The monitor uses a buffered channel (queue) to handle notifications:
- **Buffer size**: 100 notifications
- **Processing**: Single background worker processes one notification at a time
- **Flow**: Feed check → Queue notification → Worker sends to Telegram

### Rate Limiting Strategy
Two layers of protection against 429 errors:

1. **Client-side rate limiting**:
   - Enforces minimum 1 second between messages by default
   - Configurable via `TELEGRAM_RATE_LIMIT_MS`
   - Uses mutex to ensure thread-safe timing

2. **Server-side rate limit handling**:
   - Detects HTTP 429 (Too Many Requests) responses
   - Extracts `retry_after` from Telegram's response
   - Waits the specified time before retrying
   - Falls back to exponential backoff if no retry_after provided

### Retry Logic
- **Max retries**: 3 attempts
- **Backoff strategy**: Exponential (1s, 2s, 4s)
- **Retry conditions**: 429 errors, network errors, transient failures
- **Non-retryable errors**: 400 (Bad Request), 401 (Unauthorized), 403 (Forbidden)

## Configuration

### Environment Variables

```bash
# Database Configuration (same as main importer)
PGDATABASE=deelfietsdashboard
PGUSER=deelfietsdashboard
PGHOST=localhost
PGPASSWORD=your_password

# Telegram Bot Configuration
TELEGRAM_BOT_TOKEN=your_bot_token_here
TELEGRAM_CHAT_ID=your_chat_id_here

# Optional: Rate limiting configuration (default: 1000ms = 1 second between messages)
TELEGRAM_RATE_LIMIT_MS=1000
```

### Setting up Telegram Bot

1. Message [@BotFather](https://t.me/botfather) on Telegram
2. Create a new bot with `/newbot`
3. Copy the token provided
4. Get your chat ID by messaging [@userinfobot](https://t.me/userinfobot) or by sending a message to your bot and checking the API

## Installation

### Database Setup

Run the migration to create the `feed_downtime` table:

```bash
psql -d deelfietsdashboard -f sql/migrations/20260402_add_feed_downtime_table.sql
```

### Local Development

```bash
# Set environment variables
export PGDATABASE=deelfietsdashboard
export PGUSER=deelfietsdashboard
export PGHOST=localhost
export PGPASSWORD=your_password
export TELEGRAM_BOT_TOKEN=your_bot_token
export TELEGRAM_CHAT_ID=your_chat_id

# Run the monitor
go run ./cmd/feed_monitor
```

### Docker Deployment

```bash
# Build the image
docker build -f Dockerfile.feed_monitor -t feed-monitor:latest .

# Run the container
docker run -d \
  -e PGDATABASE=deelfietsdashboard \
  -e PGUSER=deelfietsdashboard \
  -e PGHOST=db-host \
  -e PGPASSWORD=your_password \
  -e TELEGRAM_BOT_TOKEN=your_bot_token \
  -e TELEGRAM_CHAT_ID=your_chat_id \
  feed-monitor:latest
```

## Database Schema

### feed_downtime Table

| Column | Type | Description |
|--------|------|-------------|
| downtime_id | SERIAL | Primary key |
| feed_id | INTEGER | Foreign key to feeds table |
| downtime_start | TIMESTAMP | When the downtime began |
| downtime_end | TIMESTAMP | When the downtime ended (NULL if ongoing) |
| reason | TEXT | Description of why feed went down |
| notification_sent | BOOLEAN | Whether initial alert was sent |
| recovery_notification_sent | BOOLEAN | Whether recovery alert was sent |
| created_at | TIMESTAMP | When the record was created |

## Notification Examples

### Downtime Alert
```
⚠️ Feed Down Alert

System: donkey
Feed ID: 8
Status: DOWN
Duration: 7 minutes
```

### Recovery Alert
```
✅ Feed Recovered

System: donkey
Feed ID: 8
Status: UP
Total Downtime: 12 minutes
```

### New Feed Enabled
```
🆕 New Feed Enabled

System: newoperator
Feed ID: 206
Status: ACTIVE
```

### Feed Disabled
```
🚫 Feed Disabled

System: oldoperator
Feed ID: 10
Status: INACTIVE
```

## Monitoring Queries

### View current ongoing downtime
```sql
SELECT f.system_id, d.downtime_start, 
       NOW() - d.downtime_start as duration
FROM feed_downtime d
JOIN feeds f ON d.feed_id = f.feed_id
WHERE d.downtime_end IS NULL;
```

### View downtime history for a feed
```sql
SELECT downtime_start, downtime_end, 
       downtime_end - downtime_start as duration,
       reason
FROM feed_downtime
WHERE feed_id = 8
ORDER BY downtime_start DESC;
```

### View uptime statistics
```sql
SELECT 
    f.system_id,
    COUNT(d.downtime_id) as downtime_count,
    SUM(EXTRACT(EPOCH FROM (COALESCE(d.downtime_end, NOW()) - d.downtime_start)))/3600 as total_hours_down
FROM feeds f
LEFT JOIN feed_downtime d ON f.feed_id = d.feed_id
WHERE f.import_vehicles = true
GROUP BY f.system_id;
```

## File Structure

```
cmd/feed_monitor/
├── main.go                    # Entry point
monitor/
├── telegram.go                # Telegram notification service
├── feed_monitor.go            # Core monitoring logic
sql/migrations/
├── 20260402_add_feed_downtime_table.sql  # Database migration
Dockerfile.feed_monitor        # Docker configuration
```

## Troubleshooting

### No notifications being sent
- Check that `TELEGRAM_BOT_TOKEN` and `TELEGRAM_CHAT_ID` are set correctly
- Verify the bot has permission to send messages to the chat
- Check logs for error messages

### 429 Too Many Requests Errors
The monitor has built-in protection against Telegram rate limits:
- By default, sends max 1 message per second (adjustable via `TELEGRAM_RATE_LIMIT_MS`)
- Automatically retries on 429 errors with exponential backoff
- Respects Telegram's `retry_after` header

If you frequently hit rate limits:
1. Increase `TELEGRAM_RATE_LIMIT_MS` (e.g., `TELEGRAM_RATE_LIMIT_MS=2000` for 2 seconds)
2. Check if multiple services are using the same bot token
3. Consider using separate bot tokens for different environments

### False positives
- The 5-minute threshold can be adjusted in `monitor/feed_monitor.go` by changing the `threshold` variable

### Database connection errors
- Verify PostgreSQL environment variables match your database configuration
- Check that the database is accessible from the monitor service

### Notification queue is full
If you see "Notification queue is full" warnings in logs:
- This means notifications are being generated faster than they can be sent
- The queue has a buffer of 100 notifications
- Consider increasing the rate limit interval if this happens frequently
- Check Telegram API status if notifications are backing up

## License

Same as the main deelfietsdashboard-importer project.
