package monitor

import (
	"strings"
	"testing"
)

func TestTruncateMessage(t *testing.T) {
	// Short message should not be truncated
	short := "short message"
	if got := truncateMessage(short); got != short {
		t.Errorf("truncateMessage(short) = %q, want %q", got, short)
	}

	// Exactly max length should not be truncated
	exact := strings.Repeat("a", maxTelegramMessageLength)
	if got := truncateMessage(exact); got != exact {
		t.Errorf("truncateMessage(exact) length = %d, want %d", len(got), maxTelegramMessageLength)
	}

	// Long message should be truncated with suffix
	long := strings.Repeat("b", maxTelegramMessageLength+100)
	got := truncateMessage(long)
	if len(got) > maxTelegramMessageLength {
		t.Errorf("truncateMessage(long) length = %d, want <= %d", len(got), maxTelegramMessageLength)
	}
	if !strings.HasSuffix(got, "...\n\n<i>Message truncated, see logs for full details.</i>") {
		t.Errorf("truncateMessage(long) missing suffix: %q", got)
	}

	// Truncation should respect UTF-8 boundaries (emoji is 4 bytes)
	emojiMsg := strings.Repeat("🙂", maxTelegramMessageLength/4) + strings.Repeat("c", maxTelegramMessageLength)
	got = truncateMessage(emojiMsg)
	if len(got) > maxTelegramMessageLength {
		t.Errorf("truncateMessage(emojiMsg) length = %d, want <= %d", len(got), maxTelegramMessageLength)
	}
	// Verify the result is valid UTF-8
	if !strings.HasSuffix(got, "...\n\n<i>Message truncated, see logs for full details.</i>") {
		t.Errorf("truncateMessage(emojiMsg) missing suffix: %q", got)
	}
}
