package skyaway

import (
	"time"
	"fmt"
	"strings"
)

func niceDuration(d time.Duration) string {
	if d < 0 {
		return d.String()
	}

	var hours, minutes, seconds int
	seconds = int(d.Seconds())
	hours, seconds = seconds / 3600, seconds % 3600
	minutes, seconds = seconds / 60, seconds % 60

	if hours > 0 {
		if minutes > 0 {
			return fmt.Sprintf("%dh%dm", hours, minutes)
		} else {
			return fmt.Sprintf("%dh", hours)
		}
	} else {
		if minutes > 0 {
			if seconds > 0 {
				return fmt.Sprintf("%dm%ds", minutes, seconds)
			} else {
				return fmt.Sprintf("%dm", minutes)
			}
		} else {
			return fmt.Sprintf("%ds", seconds)
		}
	}
}

func appendField(fields []string, name, format string, args ...interface{}) []string {
	value := fmt.Sprintf(format, args...)
	return append(fields, fmt.Sprintf("*%s*: %s", strings.Title(name), value))
}

func formatEventAsMarkdown(event *Event, public bool) string {
	var fields []string
	fields = appendField(fields, "coins", "%d", event.Coins)
	if event.StartedAt.Valid {
		fields = appendField(fields, "started", "%s (%s ago)",
			event.StartedAt.Time.Format("Jan 2 2006, 15:04:05 -0700"),
			niceDuration(time.Since(event.StartedAt.Time)),
		)
	} else {
		fields = appendField(fields, "will start", "%s (in %s)",
			event.ScheduledAt.Time.Format("Jan 2 2006, 15:04:05 -0700"),
			niceDuration(time.Until(event.ScheduledAt.Time)),
		)
	}

	if event.EndedAt.Valid {
		fields = appendField(fields, "duration", "%s (ended %s ago)",
			niceDuration(event.Duration.Duration),
			niceDuration(time.Since(event.EndedAt.Time)),
		)
	} else {
		var endsAt time.Time
		if event.StartedAt.Valid {
			endsAt = event.StartedAt.Time.Add(event.Duration.Duration)
		} else {
			endsAt = event.ScheduledAt.Time.Add(event.Duration.Duration)
		}
		fields = appendField(fields, "duration", "%s (ends in %s)",
			niceDuration(event.Duration.Duration),
			niceDuration(time.Until(endsAt)),
		)
	}

	if !public {
		fields = appendField(fields, "surprise", "%t", event.Surprise)
	}

	return strings.Join(fields, "\n")
}

