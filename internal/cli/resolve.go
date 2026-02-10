package cli

import (
	"context"
	"fmt"
	"strconv"
)

// resolveNodeID resolves a node identifier which can be:
//   - A numeric seq (requires projectID context)
//   - A UUID string (passed through directly)
func resolveNodeID(ctx context.Context, app *App, input string, projectID string) (string, error) {
	if seq, err := strconv.Atoi(input); err == nil && seq > 0 {
		if projectID == "" {
			return "", fmt.Errorf("numeric ID #%d requires project context (use --project flag or shell 'use' command)", seq)
		}
		node, err := app.Nodes.GetBySeq(ctx, projectID, seq)
		if err != nil {
			return "", fmt.Errorf("node #%d not found in project: %w", seq, err)
		}
		return node.ID, nil
	}
	return input, nil
}

// resolveWorkItemID resolves a work item identifier which can be:
//   - A numeric seq (requires projectID context)
//   - A UUID string (passed through directly)
func resolveWorkItemID(ctx context.Context, app *App, input string, projectID string) (string, error) {
	if seq, err := strconv.Atoi(input); err == nil && seq > 0 {
		if projectID == "" {
			return "", fmt.Errorf("numeric ID #%d requires project context (use --project flag or shell 'use' command)", seq)
		}
		wi, err := app.WorkItems.GetBySeq(ctx, projectID, seq)
		if err != nil {
			// Fallback: if the seq belongs to a node with exactly one work item,
			// resolve to that work item (supports collapsed tree display).
			if node, nErr := app.Nodes.GetBySeq(ctx, projectID, seq); nErr == nil {
				if items, lErr := app.WorkItems.ListByNode(ctx, node.ID); lErr == nil && len(items) == 1 {
					return items[0].ID, nil
				}
			}
			return "", fmt.Errorf("work item #%d not found in project: %w", seq, err)
		}
		return wi.ID, nil
	}
	return input, nil
}

// resolveProjectForFlag resolves a --project flag value to a project UUID.
// The flag can be a ShortID, full UUID, or UUID prefix.
func resolveProjectForFlag(ctx context.Context, app *App, input string) (string, error) {
	if input == "" {
		return "", nil
	}
	return resolveProjectID(ctx, app, input)
}
