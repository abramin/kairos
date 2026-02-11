package cli

import (
	"fmt"
	"strings"

	"github.com/alexanderramin/kairos/internal/intelligence"
)

// CommandHint produces the shell command string corresponding to a parsed intent.
// Returns an empty string if the intent cannot be meaningfully mapped.
func CommandHint(intent *intelligence.ParsedIntent) string {
	if intent == nil {
		return ""
	}
	args := intent.Arguments

	switch intent.Intent {
	case intelligence.IntentWhatNow:
		min := intArg(args, "available_min", 60)
		return fmt.Sprintf("what-now --minutes %d", min)

	case intelligence.IntentStatus:
		return "status"

	case intelligence.IntentReplan:
		cmd := "replan"
		if s, ok := stringArg(args, "strategy"); ok {
			cmd += fmt.Sprintf(" --strategy %s", s)
		}
		return cmd

	case intelligence.IntentProjectAdd:
		var parts []string
		parts = append(parts, "project add")
		parts = append(parts, "--id <SHORT_ID>")
		if name, ok := stringArg(args, "name"); ok {
			parts = append(parts, fmt.Sprintf("--name %q", name))
		} else {
			parts = append(parts, "--name <NAME>")
		}
		if domain, ok := stringArg(args, "domain"); ok {
			parts = append(parts, fmt.Sprintf("--domain %q", domain))
		} else {
			parts = append(parts, "--domain <DOMAIN>")
		}
		if start, ok := stringArg(args, "start_date"); ok {
			parts = append(parts, fmt.Sprintf("--start %s", start))
		} else {
			parts = append(parts, "--start <YYYY-MM-DD>")
		}
		if due, ok := stringArg(args, "target_date"); ok {
			parts = append(parts, fmt.Sprintf("--due %s", due))
		}
		return strings.Join(parts, " ")

	case intelligence.IntentProjectImport:
		filePath, ok := stringArg(args, "file_path")
		if !ok {
			return "project import <FILE>"
		}
		return fmt.Sprintf("project import %s", filePath)

	case intelligence.IntentProjectUpdate:
		id, _ := stringArg(args, "project_id")
		if id == "" {
			id = "<PROJECT_ID>"
		}
		cmd := fmt.Sprintf("project update %s", id)
		if name, ok := stringArg(args, "name"); ok {
			cmd += fmt.Sprintf(" --name %q", name)
		}
		if due, ok := stringArg(args, "target_date"); ok {
			cmd += fmt.Sprintf(" --due %s", due)
		}
		if status, ok := stringArg(args, "status"); ok {
			cmd += fmt.Sprintf(" --status %s", status)
		}
		return cmd

	case intelligence.IntentProjectArchive:
		id, _ := stringArg(args, "project_id")
		if id == "" {
			id = "<PROJECT_ID>"
		}
		return fmt.Sprintf("project archive %s", id)

	case intelligence.IntentProjectRemove:
		id, _ := stringArg(args, "project_id")
		if id == "" {
			id = "<PROJECT_ID>"
		}
		return fmt.Sprintf("project remove %s", id)

	case intelligence.IntentNodeAdd:
		var parts []string
		parts = append(parts, "node add")
		if pid, ok := stringArg(args, "project_id"); ok {
			parts = append(parts, fmt.Sprintf("--project %s", pid))
		} else {
			parts = append(parts, "--project <PROJECT_ID>")
		}
		if title, ok := stringArg(args, "title"); ok {
			parts = append(parts, fmt.Sprintf("--title %q", title))
		} else {
			parts = append(parts, "--title <TITLE>")
		}
		if kind, ok := stringArg(args, "kind"); ok {
			parts = append(parts, fmt.Sprintf("--kind %s", kind))
		} else {
			parts = append(parts, "--kind <KIND>")
		}
		return strings.Join(parts, " ")

	case intelligence.IntentNodeUpdate:
		id, _ := stringArg(args, "node_id")
		if id == "" {
			id = "<NODE_ID>"
		}
		cmd := fmt.Sprintf("node update %s", id)
		if title, ok := stringArg(args, "title"); ok {
			cmd += fmt.Sprintf(" --title %q", title)
		}
		if kind, ok := stringArg(args, "kind"); ok {
			cmd += fmt.Sprintf(" --kind %s", kind)
		}
		return cmd

	case intelligence.IntentNodeRemove:
		id, _ := stringArg(args, "node_id")
		if id == "" {
			id = "<NODE_ID>"
		}
		return fmt.Sprintf("node remove %s", id)

	case intelligence.IntentWorkAdd:
		var parts []string
		parts = append(parts, "work add")
		if nid, ok := stringArg(args, "node_id"); ok {
			parts = append(parts, fmt.Sprintf("--node %s", nid))
		} else {
			parts = append(parts, "--node <NODE_ID>")
		}
		if title, ok := stringArg(args, "title"); ok {
			parts = append(parts, fmt.Sprintf("--title %q", title))
		} else {
			parts = append(parts, "--title <TITLE>")
		}
		if typ, ok := stringArg(args, "type"); ok {
			parts = append(parts, fmt.Sprintf("--type %s", typ))
		} else {
			parts = append(parts, "--type <TYPE>")
		}
		if min := intArg(args, "planned_min", 0); min > 0 {
			parts = append(parts, fmt.Sprintf("--planned-min %d", min))
		}
		return strings.Join(parts, " ")

	case intelligence.IntentWorkUpdate:
		id, _ := stringArg(args, "work_item_id")
		if id == "" {
			id = "<WORK_ITEM_ID>"
		}
		cmd := fmt.Sprintf("work update %s", id)
		if title, ok := stringArg(args, "title"); ok {
			cmd += fmt.Sprintf(" --title %q", title)
		}
		if status, ok := stringArg(args, "status"); ok {
			cmd += fmt.Sprintf(" --status %s", status)
		}
		if min := intArg(args, "planned_min", 0); min > 0 {
			cmd += fmt.Sprintf(" --planned-min %d", min)
		}
		return cmd

	case intelligence.IntentWorkDone:
		id, _ := stringArg(args, "work_item_id")
		if id == "" {
			id = "<WORK_ITEM_ID>"
		}
		return fmt.Sprintf("work done %s", id)

	case intelligence.IntentWorkRemove:
		id, _ := stringArg(args, "work_item_id")
		if id == "" {
			id = "<WORK_ITEM_ID>"
		}
		return fmt.Sprintf("work remove %s", id)

	case intelligence.IntentSessionLog:
		var parts []string
		parts = append(parts, "session log")
		if wid, ok := stringArg(args, "work_item_id"); ok {
			parts = append(parts, fmt.Sprintf("--work-item %s", wid))
		} else {
			parts = append(parts, "--work-item <WORK_ITEM_ID>")
		}
		if min := intArg(args, "minutes", 0); min > 0 {
			parts = append(parts, fmt.Sprintf("--minutes %d", min))
		} else {
			parts = append(parts, "--minutes <MINUTES>")
		}
		if note, ok := stringArg(args, "note"); ok {
			parts = append(parts, fmt.Sprintf("--note %q", note))
		}
		return strings.Join(parts, " ")

	case intelligence.IntentSessionRemove:
		id, _ := stringArg(args, "session_id")
		if id == "" {
			id = "<SESSION_ID>"
		}
		return fmt.Sprintf("session remove %s", id)

	case intelligence.IntentTemplateList:
		return "template list"

	case intelligence.IntentTemplateShow:
		ref, _ := stringArg(args, "template_id")
		if ref == "" {
			ref = "<TEMPLATE_REF>"
		}
		return fmt.Sprintf("template show %s", ref)

	case intelligence.IntentTemplateDraft:
		prompt, ok := stringArg(args, "prompt")
		if !ok {
			return "template draft <DESCRIPTION>"
		}
		return fmt.Sprintf("template draft %q", prompt)

	case intelligence.IntentProjectInitFromTmpl:
		var parts []string
		parts = append(parts, "project init")
		parts = append(parts, "--id <SHORT_ID>")
		if tmpl, ok := stringArg(args, "template_id"); ok {
			parts = append(parts, fmt.Sprintf("--template %s", tmpl))
		} else {
			parts = append(parts, "--template <TEMPLATE_REF>")
		}
		if name, ok := stringArg(args, "project_name"); ok {
			parts = append(parts, fmt.Sprintf("--name %q", name))
		} else {
			parts = append(parts, "--name <NAME>")
		}
		if start, ok := stringArg(args, "start_date"); ok {
			parts = append(parts, fmt.Sprintf("--start %s", start))
		} else {
			parts = append(parts, "--start <YYYY-MM-DD>")
		}
		if due, ok := stringArg(args, "target_date"); ok {
			parts = append(parts, fmt.Sprintf("--due %s", due))
		}
		return strings.Join(parts, " ")

	case intelligence.IntentExplainNow:
		cmd := "explain now"
		if min := intArg(args, "minutes", 0); min > 0 {
			cmd += fmt.Sprintf(" --minutes %d", min)
		}
		return cmd

	case intelligence.IntentExplainWhyNot:
		cmd := "explain why-not"
		if pid, ok := stringArg(args, "project_id"); ok {
			cmd += fmt.Sprintf(" --project %s", pid)
		}
		if wid, ok := stringArg(args, "work_item_id"); ok {
			cmd += fmt.Sprintf(" --work-item %s", wid)
		}
		return cmd

	case intelligence.IntentReviewWeekly:
		return "review weekly"

	default:
		return ""
	}
}
