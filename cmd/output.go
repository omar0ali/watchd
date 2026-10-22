package cmd

import (
	"fmt"
	"hash/fnv"
	"os"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/omar0ali/watchd/internal/api"
)

const (
	cReset  = "\033[0m"
	cDim    = "\033[2m"
	cBold   = "\033[1m"
	cRed    = "\033[31m"
	cGreen  = "\033[32m"
	cYellow = "\033[33m"
)

var appColors = []string{"\033[36m", "\033[35m", "\033[34m", "\033[33m", "\033[32m", "\033[96m", "\033[95m", "\033[94m"}

var useColor = func() bool {
	if os.Getenv("NO_COLOR") != "" || os.Getenv("TERM") == "dumb" {
		return false
	}
	fi, err := os.Stdout.Stat()
	return err == nil && fi.Mode()&os.ModeCharDevice != 0
}()

func color(c, s string) string {
	if !useColor || c == "" {
		return s
	}
	return c + s + cReset
}

func appColor(name string) string {
	h := fnv.New32a()
	h.Write([]byte(name))
	return appColors[h.Sum32()%uint32(len(appColors))]
}

// cell is a table cell with an optional color, kept separate so ANSI codes
// don't break column alignment.
type cell struct {
	text, color string
}

func printTable(header []string, rows [][]cell) {
	widths := make([]int, len(header))
	for i, h := range header {
		widths[i] = len(h)
	}
	for _, r := range rows {
		for i, c := range r {
			widths[i] = max(widths[i], utf8.RuneCountInString(c.text))
		}
	}
	var b strings.Builder
	for i, h := range header {
		b.WriteString(color(cBold, pad(h, widths[i], i == len(header)-1)))
	}
	fmt.Println(strings.TrimRight(b.String(), " "))
	for _, r := range rows {
		b.Reset()
		for i, c := range r {
			b.WriteString(color(c.color, pad(c.text, widths[i], i == len(r)-1)))
		}
		fmt.Println(strings.TrimRight(b.String(), " "))
	}
}

func pad(s string, w int, last bool) string {
	if last {
		return s
	}
	return s + strings.Repeat(" ", w-utf8.RuneCountInString(s)+2)
}

func humanDuration(d time.Duration) string {
	d = d.Round(time.Second)
	switch {
	case d < time.Minute:
		return fmt.Sprintf("%ds", int(d.Seconds()))
	case d < time.Hour:
		return fmt.Sprintf("%dm%02ds", int(d.Minutes()), int(d.Seconds())%60)
	case d < 24*time.Hour:
		return fmt.Sprintf("%dh%02dm", int(d.Hours()), int(d.Minutes())%60)
	default:
		return fmt.Sprintf("%dd%02dh", int(d.Hours())/24, int(d.Hours())%24)
	}
}

// stateCell renders an app's state, e.g. "running (healthy)" or "failed (exit 1)".
func stateCell(s api.AppStatus) cell {
	text := s.State
	c := ""
	switch s.State {
	case api.StateRunning:
		c = cGreen
		if s.Health != "" {
			text += " (" + s.Health + ")"
			if s.Health == api.HealthUnhealthy {
				c = cRed
			} else if s.Health == api.HealthStarting {
				c = cYellow
			}
		}
	case api.StateWaiting, api.StateBackoff:
		c = cYellow
	case api.StateFailed:
		c = cRed
	case api.StateStopped:
		c = cDim
	}
	if (s.State == api.StateExited || s.State == api.StateFailed || s.State == api.StateBackoff) && s.ExitCode != nil {
		text += fmt.Sprintf(" (exit %d)", *s.ExitCode)
	}
	return cell{text, c}
}
