package usage

import (
	"fmt"
	"strings"
	"time"
)

// MaskAccount obscures account identity (emails or usernames) for privacy in logs and terminal outputs.
func MaskAccount(account string) string {
	account = strings.TrimSpace(account)
	if account == "" {
		return ""
	}
	parts := strings.Split(account, "@")
	if len(parts) == 2 {
		user := parts[0]
		domain := parts[1]
		if len(user) <= 2 {
			return user[:1] + "***@" + domain
		}
		return user[:1] + "***" + user[len(user)-1:] + "@" + domain
	}
	if len(account) <= 4 {
		return "***"
	}
	return account[:2] + "***" + account[len(account)-2:]
}

// FormatDuration formats a time.Duration into human readable string like "4h 52m" or "32m".
func FormatDuration(d time.Duration) string {
	if d <= 0 {
		return "0m"
	}
	hours := int(d.Hours())
	mins := int(d.Minutes()) % 60
	if hours > 0 {
		return fmt.Sprintf("%dh %dm", hours, mins)
	}
	return fmt.Sprintf("%dm", mins)
}

// RenderProgressBar generates an ANSI/Unicode progress bar of the given character width.
func RenderProgressBar(usedPercent float64, width int) string {
	if width <= 0 {
		width = 20
	}
	if usedPercent < 0 {
		usedPercent = 0
	}
	if usedPercent > 100 {
		usedPercent = 100
	}
	filledCount := int(float64(width) * (usedPercent / 100.0))
	if filledCount > width {
		filledCount = width
	}
	emptyCount := width - filledCount

	filled := strings.Repeat("█", filledCount)
	empty := strings.Repeat("░", emptyCount)
	return "[" + filled + empty + "]"
}

// FormatNumber formats an integer with thousand commas.
func FormatNumber(n int64) string {
	if n < 0 {
		return "-" + FormatNumber(-n)
	}
	s := fmt.Sprintf("%d", n)
	if len(s) <= 3 {
		return s
	}
	var res []string
	for len(s) > 3 {
		res = append([]string{s[len(s)-3:]}, res...)
		s = s[:len(s)-3]
	}
	if len(s) > 0 {
		res = append([]string{s}, res...)
	}
	return strings.Join(res, ",")
}
