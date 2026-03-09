package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/alexanderramin/kairos/internal/cli/formatter"
	"github.com/alexanderramin/kairos/internal/service"
	tea "github.com/charmbracelet/bubbletea"
)

func (c *commandBar) cmdCalendarSync(args []string) tea.Cmd {
	_, flags := parseShellFlags(args)
	daysAhead := 14
	if v, ok := flags["days"]; ok {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			daysAhead = n
		}
	}
	return tea.Batch(
		loadingCmd(fmt.Sprintf("Syncing due items (next %dd) to Google Calendar...", daysAhead)),
		asyncOutputCmd(func() string {
			ctx := context.Background()
			result, err := execCalendarSync(ctx, c.state.App, daysAhead)
			if err != nil {
				return shellError(err)
			}
			return result
		}),
	)
}

func execCalendarSync(ctx context.Context, app *App, daysAhead int) (string, error) {
	gwsBin, err := exec.LookPath("gws")
	if err != nil {
		gwsBin = "/opt/homebrew/bin/gws"
	}

	items, err := app.WorkItems.ListDueItems(ctx, daysAhead)
	if err != nil {
		return "", fmt.Errorf("listing due items: %w", err)
	}
	if len(items) == 0 {
		return formatter.Dim(fmt.Sprintf("No items with due dates in the next %d days.", daysAhead)), nil
	}

	existing, err := listKairosEvents(ctx, gwsBin, daysAhead)
	if err != nil {
		// Fail-open: proceed without dedup if listing fails.
		existing = map[string]bool{}
	}

	var sb strings.Builder
	var synced, skipped, failed int
	for _, item := range items {
		key := fmt.Sprintf("[Kairos] %s|%s", item.Title, item.DueDate.Format("2006-01-02"))
		if existing[key] {
			sb.WriteString(fmt.Sprintf("  %s %s → %s\n",
				formatter.Dim("↷"),
				formatter.Dim(item.Title),
				formatter.Dim(item.DueDate.Format("2006-01-02"))))
			skipped++
			continue
		}
		if err := pushCalendarEvent(ctx, gwsBin, item); err != nil {
			sb.WriteString(fmt.Sprintf("  %s %s\n",
				formatter.StyleRed.Render("✗"),
				formatter.Dim(fmt.Sprintf("%q: %s", item.Title, err.Error()))))
			failed++
			continue
		}
		sb.WriteString(fmt.Sprintf("  %s Synced %s → %s\n",
			formatter.StyleGreen.Render("✔"),
			formatter.Bold(item.Title),
			formatter.Dim(item.DueDate.Format("2006-01-02"))))
		synced++
	}

	sb.WriteString(fmt.Sprintf("\n  %d synced", synced))
	if skipped > 0 {
		sb.WriteString(fmt.Sprintf(", %d already synced", skipped))
	}
	if failed > 0 {
		sb.WriteString(fmt.Sprintf(", %s failed", formatter.StyleRed.Render(strconv.Itoa(failed))))
	}

	return formatter.RenderBox("Calendar Sync", sb.String()), nil
}

// listKairosEvents returns a set of existing [Kairos] event keys (summary|date) in the sync window.
func listKairosEvents(ctx context.Context, gwsBin string, daysAhead int) (map[string]bool, error) {
	now := time.Now().UTC()
	timeMin := now.Format(time.RFC3339)
	timeMax := now.AddDate(0, 0, daysAhead).Format(time.RFC3339)

	params := fmt.Sprintf(`{"calendarId":"primary","timeMin":%q,"timeMax":%q}`, timeMin, timeMax)
	cmd := exec.CommandContext(ctx, gwsBin,
		"calendar", "events", "list",
		"--params", params,
		"--format", "json",
	)
	out, err := cmd.Output()
	if err != nil {
		return nil, err
	}

	var resp struct {
		Items []struct {
			Summary string `json:"summary"`
			Start   struct {
				Date string `json:"date"`
			} `json:"start"`
		} `json:"items"`
	}
	if err := json.Unmarshal(out, &resp); err != nil {
		return nil, err
	}

	existing := make(map[string]bool, len(resp.Items))
	for _, ev := range resp.Items {
		if strings.HasPrefix(ev.Summary, "[Kairos]") {
			existing[ev.Summary+"|"+ev.Start.Date] = true
		}
	}
	return existing, nil
}

// pushCalendarEvent calls `gws calendar events insert` to create an all-day event.
func pushCalendarEvent(ctx context.Context, gwsBin string, item service.DueItem) error {
	endDate := item.DueDate.AddDate(0, 0, 1)

	desc := fmt.Sprintf("Project: %s\nEstimated: %dmin\nStatus: %s",
		item.ProjectName, item.PlannedMin, string(item.Status))

	type calDate struct {
		Date string `json:"date"`
	}
	event := struct {
		Summary     string  `json:"summary"`
		Description string  `json:"description"`
		Start       calDate `json:"start"`
		End         calDate `json:"end"`
	}{
		Summary:     fmt.Sprintf("[Kairos] %s", item.Title),
		Description: desc,
		Start:       calDate{Date: item.DueDate.Format("2006-01-02")},
		End:         calDate{Date: endDate.Format("2006-01-02")},
	}

	eventJSON, err := json.Marshal(event)
	if err != nil {
		return err
	}

	cmd := exec.CommandContext(ctx, gwsBin,
		"calendar", "events", "insert",
		"--params", `{"calendarId":"primary"}`,
		"--json", string(eventJSON),
	)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			msg = err.Error()
		}
		return fmt.Errorf("%s", msg)
	}
	return nil
}
