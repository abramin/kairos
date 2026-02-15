package cli

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/alexanderramin/kairos/internal/cli/formatter"
	"github.com/alexanderramin/kairos/internal/domain"
	"github.com/google/uuid"
	tea "github.com/charmbracelet/bubbletea"
)

// parseShellFlags extracts --key value pairs and positional args from a shell arg list.
func parseShellFlags(args []string) (positional []string, flags map[string]string) {
	flags = make(map[string]string)
	for i := 0; i < len(args); i++ {
		if strings.HasPrefix(args[i], "--") {
			key := strings.TrimPrefix(args[i], "--")
			if i+1 < len(args) && !strings.HasPrefix(args[i+1], "--") {
				flags[key] = args[i+1]
				i++
			} else {
				flags[key] = "true"
			}
		} else if len(args[i]) == 2 && args[i][0] == '-' {
			flags[string(args[i][1])] = "true"
		} else {
			positional = append(positional, args[i])
		}
	}
	return
}

// entityGroupHelp returns usage text for a bare entity group command.
func entityGroupHelp(group string) string {
	subs := map[string]string{
		"project":  "list, inspect, add, update, archive, unarchive, remove, init, import, draft",
		"node":     "add, inspect, update, remove",
		"work":     "add, inspect, update, done, archive, remove",
		"session":  "log, list, remove",
		"template": "list, show",
	}
	if s, ok := subs[group]; ok {
		return fmt.Sprintf("%s subcommands: %s", group, s)
	}
	return fmt.Sprintf("Unknown group: %s", group)
}

// dispatchEntityCommand routes entity subcommands to direct service calls.
func (c *commandBar) dispatchEntityCommand(group, sub string, args []string) tea.Cmd {
	ctx := context.Background()
	positional, flags := parseShellFlags(args)
	app := c.state.App

	var result string
	var err error

	switch group {
	case "project":
		result, err = c.dispatchProject(ctx, sub, positional, flags)
	case "node":
		result, err = c.dispatchNode(ctx, sub, positional, flags)
	case "work":
		result, err = c.dispatchWork(ctx, sub, positional, flags)
	case "session":
		result, err = c.dispatchSession(ctx, sub, positional, flags)
	case "template":
		result, err = c.dispatchTemplate(ctx, sub, positional, flags)
	default:
		return outputCmd(fmt.Sprintf("Unknown entity group: %s", group))
	}

	if err != nil {
		return outputCmd(shellError(err))
	}
	_ = app // used in sub-dispatchers via c.state.App
	return outputCmd(result)
}

// ── project dispatch ─────────────────────────────────────────────────────────

func (c *commandBar) dispatchProject(ctx context.Context, sub string, pos []string, flags map[string]string) (string, error) {
	app := c.state.App

	switch sub {
	case "list":
		_, all := flags["all"]
		projects, err := app.Projects.List(ctx, all)
		if err != nil {
			return "", err
		}
		if len(projects) == 0 {
			return "No projects found.", nil
		}
		return formatter.FormatProjectList(projects), nil

	case "inspect":
		if len(pos) == 0 {
			return "", fmt.Errorf("usage: project inspect <id>")
		}
		projectID, err := resolveProjectID(ctx, app, pos[0])
		if err != nil {
			return "", err
		}
		return buildInspectTree(app, ctx, projectID)

	case "add":
		shortID := flags["id"]
		name := flags["name"]
		domainStr := flags["domain"]
		start := flags["start"]
		if shortID == "" || name == "" || domainStr == "" || start == "" {
			return "", fmt.Errorf("usage: project add --id ID --name NAME --domain DOMAIN --start YYYY-MM-DD [--due YYYY-MM-DD]")
		}
		startDate, err := time.Parse("2006-01-02", start)
		if err != nil {
			return "", fmt.Errorf("invalid start date %q: %w", start, err)
		}
		p := &domain.Project{
			ID:        uuid.New().String(),
			ShortID:   strings.ToUpper(shortID),
			Name:      name,
			Domain:    domainStr,
			StartDate: startDate,
			Status:    domain.ProjectActive,
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}
		if due, ok := flags["due"]; ok {
			dueDate, err := time.Parse("2006-01-02", due)
			if err != nil {
				return "", fmt.Errorf("invalid due date %q: %w", due, err)
			}
			p.TargetDate = &dueDate
		}
		if err := app.Projects.Create(ctx, p); err != nil {
			return "", err
		}
		return fmt.Sprintf("%s Created project %s [%s]", formatter.StyleGreen.Render("✔"), p.Name, p.ShortID), nil

	case "update":
		if len(pos) == 0 {
			return "", fmt.Errorf("usage: project update <id> [--name NAME] [--domain DOMAIN] [--due YYYY-MM-DD] [--status STATUS]")
		}
		projectID, err := resolveProjectID(ctx, app, pos[0])
		if err != nil {
			return "", err
		}
		p, err := app.Projects.GetByID(ctx, projectID)
		if err != nil {
			return "", err
		}
		if v, ok := flags["id"]; ok {
			p.ShortID = strings.ToUpper(v)
		}
		if v, ok := flags["name"]; ok {
			p.Name = v
		}
		if v, ok := flags["domain"]; ok {
			p.Domain = v
		}
		if v, ok := flags["due"]; ok {
			dueDate, err := time.Parse("2006-01-02", v)
			if err != nil {
				return "", fmt.Errorf("invalid due date %q: %w", v, err)
			}
			p.TargetDate = &dueDate
		}
		if v, ok := flags["status"]; ok {
			p.Status = domain.ProjectStatus(v)
		}
		p.UpdatedAt = time.Now()
		if err := app.Projects.Update(ctx, p); err != nil {
			return "", err
		}
		return fmt.Sprintf("%s Updated project %s [%s]", formatter.StyleGreen.Render("✔"), p.Name, p.ShortID), nil

	case "archive":
		if len(pos) == 0 {
			return "", fmt.Errorf("usage: project archive <id>")
		}
		projectID, err := resolveProjectID(ctx, app, pos[0])
		if err != nil {
			return "", err
		}
		if err := app.Projects.Archive(ctx, projectID); err != nil {
			return "", err
		}
		return fmt.Sprintf("%s Archived project", formatter.StyleGreen.Render("✔")), nil

	case "unarchive":
		if len(pos) == 0 {
			return "", fmt.Errorf("usage: project unarchive <id>")
		}
		projectID, err := resolveProjectID(ctx, app, pos[0])
		if err != nil {
			return "", err
		}
		if err := app.Projects.Unarchive(ctx, projectID); err != nil {
			return "", err
		}
		return fmt.Sprintf("%s Unarchived project", formatter.StyleGreen.Render("✔")), nil

	case "remove":
		if len(pos) == 0 {
			return "", fmt.Errorf("usage: project remove <id> [--force]")
		}
		projectID, err := resolveProjectID(ctx, app, pos[0])
		if err != nil {
			return "", err
		}
		_, force := flags["force"]
		if err := app.Projects.Delete(ctx, projectID, force); err != nil {
			return "", err
		}
		return fmt.Sprintf("%s Removed project", formatter.StyleGreen.Render("✔")), nil

	case "init":
		templateRef := flags["template"]
		name := flags["name"]
		shortID := flags["id"]
		start := flags["start"]
		if templateRef == "" || name == "" || shortID == "" || start == "" {
			return "", fmt.Errorf("usage: project init --id ID --template REF --name NAME --start YYYY-MM-DD [--due YYYY-MM-DD] [--var K=V]")
		}
		var duePtr *string
		if due, ok := flags["due"]; ok {
			duePtr = &due
		}
		initProject := app.initProjectUseCase()
		if initProject == nil {
			return "", fmt.Errorf("init-project use case is not configured")
		}
		p, err := initProject.InitProject(ctx, templateRef, name, strings.ToUpper(shortID), start, duePtr, nil)
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("%s Initialized project %s [%s] from template %q",
			formatter.StyleGreen.Render("✔"), p.Name, p.ShortID, templateRef), nil

	case "import":
		if len(pos) == 0 {
			return "", fmt.Errorf("usage: project import <file.json>")
		}
		return execImport(ctx, app, pos[0])

	default:
		return "", fmt.Errorf("unknown project subcommand: %s", sub)
	}
}

// ── node dispatch ────────────────────────────────────────────────────────────

func (c *commandBar) dispatchNode(ctx context.Context, sub string, pos []string, flags map[string]string) (string, error) {
	app := c.state.App
	projectID := c.state.ActiveProjectID

	switch sub {
	case "add":
		pid := flags["project"]
		if pid == "" {
			pid = projectID
		}
		title := flags["title"]
		kind := flags["kind"]
		if pid == "" || title == "" || kind == "" {
			return "", fmt.Errorf("usage: node add --project ID --title TITLE --kind KIND [--parent ID] [--order N]")
		}
		n := &domain.PlanNode{
			ID:        uuid.New().String(),
			ProjectID: pid,
			Title:     title,
			Kind:      domain.NodeKind(kind),
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}
		if parentID, ok := flags["parent"]; ok {
			n.ParentID = &parentID
		}
		if orderStr, ok := flags["order"]; ok {
			if o, err := strconv.Atoi(orderStr); err == nil {
				n.OrderIndex = o
			}
		}
		if err := app.Nodes.Create(ctx, n); err != nil {
			return "", err
		}
		return fmt.Sprintf("%s Created node: %s", formatter.StyleGreen.Render("✔"), formatter.Bold(title)), nil

	case "inspect":
		if len(pos) == 0 {
			return "", fmt.Errorf("usage: node inspect <id>")
		}
		nodeID, err := resolveNodeID(ctx, app, pos[0], projectID)
		if err != nil {
			return "", err
		}
		n, err := app.Nodes.GetByID(ctx, nodeID)
		if err != nil {
			return "", err
		}
		var b strings.Builder
		b.WriteString(fmt.Sprintf("%s  %s\n", formatter.Bold(n.Title), formatter.Dim(string(n.Kind))))
		if n.Seq > 0 {
			b.WriteString(fmt.Sprintf("  ID: #%d\n", n.Seq))
		}
		b.WriteString(fmt.Sprintf("  Order: %d\n", n.OrderIndex))
		if n.DueDate != nil {
			b.WriteString(fmt.Sprintf("  Due: %s\n", n.DueDate.Format("2006-01-02")))
		}
		return b.String(), nil

	case "update":
		if len(pos) == 0 {
			return "", fmt.Errorf("usage: node update <id> [--title TITLE] [--kind KIND] [--order N]")
		}
		nodeID, err := resolveNodeID(ctx, app, pos[0], projectID)
		if err != nil {
			return "", err
		}
		n, err := app.Nodes.GetByID(ctx, nodeID)
		if err != nil {
			return "", err
		}
		if v, ok := flags["title"]; ok {
			n.Title = v
		}
		if v, ok := flags["kind"]; ok {
			n.Kind = domain.NodeKind(v)
		}
		if v, ok := flags["order"]; ok {
			if o, err := strconv.Atoi(v); err == nil {
				n.OrderIndex = o
			}
		}
		n.UpdatedAt = time.Now()
		if err := app.Nodes.Update(ctx, n); err != nil {
			return "", err
		}
		return fmt.Sprintf("%s Updated node: %s", formatter.StyleGreen.Render("✔"), formatter.Bold(n.Title)), nil

	case "remove":
		if len(pos) == 0 {
			return "", fmt.Errorf("usage: node remove <id>")
		}
		nodeID, err := resolveNodeID(ctx, app, pos[0], projectID)
		if err != nil {
			return "", err
		}
		if err := app.Nodes.Delete(ctx, nodeID); err != nil {
			return "", err
		}
		return fmt.Sprintf("%s Removed node", formatter.StyleGreen.Render("✔")), nil

	default:
		return "", fmt.Errorf("unknown node subcommand: %s", sub)
	}
}

// ── work dispatch ────────────────────────────────────────────────────────────

func (c *commandBar) dispatchWork(ctx context.Context, sub string, pos []string, flags map[string]string) (string, error) {
	app := c.state.App
	projectID := c.state.ActiveProjectID

	switch sub {
	case "add":
		nodeID := flags["node"]
		title := flags["title"]
		typ := flags["type"]
		if nodeID == "" || title == "" || typ == "" {
			return "", fmt.Errorf("usage: work add --node ID --title TITLE --type TYPE [--planned-min N] [--due-date YYYY-MM-DD]")
		}
		w := &domain.WorkItem{
			ID:        uuid.New().String(),
			NodeID:    nodeID,
			Title:     title,
			Type:      typ,
			Status:    domain.WorkItemTodo,
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}
		if v, ok := flags["planned-min"]; ok {
			if m, err := strconv.Atoi(v); err == nil {
				w.PlannedMin = m
			}
		}
		if v, ok := flags["due-date"]; ok {
			t, err := time.Parse("2006-01-02", v)
			if err != nil {
				return "", fmt.Errorf("invalid due date %q: %w", v, err)
			}
			w.DueDate = &t
		}
		if err := app.WorkItems.Create(ctx, w); err != nil {
			return "", err
		}
		return fmt.Sprintf("%s Created: %s", formatter.StyleGreen.Render("✔"), formatter.Bold(title)), nil

	case "inspect":
		if len(pos) == 0 {
			return "", fmt.Errorf("usage: work inspect <id>")
		}
		wiID, err := resolveWorkItemID(ctx, app, pos[0], projectID)
		if err != nil {
			return "", err
		}
		w, err := app.WorkItems.GetByID(ctx, wiID)
		if err != nil {
			return "", err
		}
		var b strings.Builder
		b.WriteString(fmt.Sprintf("%s  %s\n", formatter.Bold(w.Title), formatter.Dim(w.Type)))
		b.WriteString(fmt.Sprintf("  Status:  %s\n", formatter.WorkItemStatusPill(w.Status)))
		if w.Seq > 0 {
			b.WriteString(fmt.Sprintf("  ID:      #%d\n", w.Seq))
		}
		b.WriteString(fmt.Sprintf("  Planned: %s\n", formatter.FormatMinutes(w.PlannedMin)))
		b.WriteString(fmt.Sprintf("  Logged:  %s\n", formatter.FormatMinutes(w.LoggedMin)))
		if w.DueDate != nil {
			b.WriteString(fmt.Sprintf("  Due:     %s\n", formatter.RelativeDateStyled(*w.DueDate)))
		}
		return b.String(), nil

	case "update":
		if len(pos) == 0 {
			return "", fmt.Errorf("usage: work update <id> [--title T] [--type T] [--status S] [--planned-min N]")
		}
		wiID, err := resolveWorkItemID(ctx, app, pos[0], projectID)
		if err != nil {
			return "", err
		}
		w, err := app.WorkItems.GetByID(ctx, wiID)
		if err != nil {
			return "", err
		}
		if v, ok := flags["title"]; ok {
			w.Title = v
		}
		if v, ok := flags["type"]; ok {
			w.Type = v
		}
		if v, ok := flags["status"]; ok {
			w.Status = domain.WorkItemStatus(v)
		}
		if v, ok := flags["planned-min"]; ok {
			if m, err := strconv.Atoi(v); err == nil {
				w.PlannedMin = m
			}
		}
		w.UpdatedAt = time.Now()
		if err := app.WorkItems.Update(ctx, w); err != nil {
			return "", err
		}
		return fmt.Sprintf("%s Updated: %s", formatter.StyleGreen.Render("✔"), formatter.Bold(w.Title)), nil

	case "done":
		if len(pos) == 0 {
			return "", fmt.Errorf("usage: work done <id>")
		}
		wiID, err := resolveWorkItemID(ctx, app, pos[0], projectID)
		if err != nil {
			return "", err
		}
		if err := app.WorkItems.MarkDone(ctx, wiID); err != nil {
			return "", err
		}
		return fmt.Sprintf("%s Marked as done", formatter.StyleGreen.Render("✔")), nil

	case "archive":
		if len(pos) == 0 {
			return "", fmt.Errorf("usage: work archive <id>")
		}
		wiID, err := resolveWorkItemID(ctx, app, pos[0], projectID)
		if err != nil {
			return "", err
		}
		if err := app.WorkItems.Archive(ctx, wiID); err != nil {
			return "", err
		}
		return fmt.Sprintf("%s Archived work item", formatter.StyleGreen.Render("✔")), nil

	case "remove":
		if len(pos) == 0 {
			return "", fmt.Errorf("usage: work remove <id>")
		}
		wiID, err := resolveWorkItemID(ctx, app, pos[0], projectID)
		if err != nil {
			return "", err
		}
		if err := app.WorkItems.Delete(ctx, wiID); err != nil {
			return "", err
		}
		return fmt.Sprintf("%s Removed work item", formatter.StyleGreen.Render("✔")), nil

	default:
		return "", fmt.Errorf("unknown work subcommand: %s", sub)
	}
}

// ── session dispatch ─────────────────────────────────────────────────────────

func (c *commandBar) dispatchSession(ctx context.Context, sub string, pos []string, flags map[string]string) (string, error) {
	app := c.state.App
	projectID := c.state.ActiveProjectID

	switch sub {
	case "log":
		wiFlag := flags["work-item"]
		minFlag := flags["minutes"]
		if wiFlag == "" || minFlag == "" {
			return "", fmt.Errorf("usage: session log --work-item ID --minutes N [--units-done N] [--note TEXT]")
		}
		wiID, err := resolveWorkItemID(ctx, app, wiFlag, projectID)
		if err != nil {
			return "", err
		}
		minutes, err := strconv.Atoi(minFlag)
		if err != nil || minutes <= 0 {
			return "", fmt.Errorf("invalid minutes: %s", minFlag)
		}
		s := &domain.WorkSessionLog{
			ID:         uuid.New().String(),
			WorkItemID: wiID,
			StartedAt:  time.Now(),
			Minutes:    minutes,
			Note:       flags["note"],
			CreatedAt:  time.Now(),
		}
		if v, ok := flags["units-done"]; ok {
			if u, err := strconv.Atoi(v); err == nil {
				s.UnitsDoneDelta = u
			}
		}
		logSession := app.logSessionUseCase()
		if logSession == nil {
			return "", fmt.Errorf("log-session use case is not configured")
		}
		if err := logSession.LogSession(ctx, s); err != nil {
			return "", err
		}
		return fmt.Sprintf("%s Logged %s session",
			formatter.StyleGreen.Render("✔"),
			formatter.Bold(formatter.FormatMinutes(minutes))), nil

	case "list":
		wiFlag := flags["work-item"]
		daysStr := flags["days"]
		days := 7
		if daysStr != "" {
			if d, err := strconv.Atoi(daysStr); err == nil {
				days = d
			}
		}
		var sessions []*domain.WorkSessionLog
		var err error
		if wiFlag != "" {
			wiID, resolveErr := resolveWorkItemID(ctx, app, wiFlag, projectID)
			if resolveErr != nil {
				return "", resolveErr
			}
			sessions, err = app.Sessions.ListByWorkItem(ctx, wiID)
		} else {
			sessions, err = app.Sessions.ListRecent(ctx, days)
		}
		if err != nil {
			return "", err
		}
		if len(sessions) == 0 {
			return "No sessions found.", nil
		}
		headers := []string{"ID", "WORK ITEM", "STARTED", "DURATION", "UNITS", "NOTE"}
		rows := make([][]string, 0, len(sessions))
		for _, s := range sessions {
			notePreview := s.Note
			if len(notePreview) > 40 {
				notePreview = notePreview[:37] + "..."
			}
			rows = append(rows, []string{
				formatter.TruncID(s.ID),
				formatter.TruncID(s.WorkItemID),
				formatter.HumanTimestamp(s.StartedAt),
				formatter.FormatMinutes(s.Minutes),
				fmt.Sprintf("%d", s.UnitsDoneDelta),
				formatter.Dim(notePreview),
			})
		}
		return formatter.RenderBox("Sessions", formatter.RenderTable(headers, rows)), nil

	case "remove":
		if len(pos) == 0 {
			return "", fmt.Errorf("usage: session remove <id>")
		}
		if err := app.Sessions.Delete(ctx, pos[0]); err != nil {
			return "", err
		}
		return fmt.Sprintf("%s Removed session", formatter.StyleGreen.Render("✔")), nil

	default:
		return "", fmt.Errorf("unknown session subcommand: %s", sub)
	}
}

// ── template dispatch ────────────────────────────────────────────────────────

func (c *commandBar) dispatchTemplate(ctx context.Context, sub string, pos []string, _ map[string]string) (string, error) {
	app := c.state.App

	switch sub {
	case "list":
		templates, err := app.Templates.List(ctx)
		if err != nil {
			return "", err
		}
		if len(templates) == 0 {
			return "No templates found.", nil
		}
		return formatter.FormatTemplateList(templates), nil

	case "show":
		if len(pos) == 0 {
			return "", fmt.Errorf("usage: template show <ref>")
		}
		t, err := app.Templates.Get(ctx, pos[0])
		if err != nil {
			return "", err
		}
		return formatter.FormatTemplateShow(t), nil

	default:
		return "", fmt.Errorf("unknown template subcommand: %s", sub)
	}
}

// ── shared helpers ───────────────────────────────────────────────────────────

// execImport runs a project import and returns formatted output.
func execImport(ctx context.Context, app *App, filePath string) (string, error) {
	importProject := app.importProjectUseCase()
	if importProject == nil {
		return "", fmt.Errorf("import-project use case is not configured")
	}
	result, err := importProject.ImportProject(ctx, filePath)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%s Imported %s [%s] — %d nodes, %d items, %d deps",
		formatter.StyleGreen.Render("✔"),
		formatter.Bold(result.Project.Name),
		result.Project.ShortID,
		result.NodeCount, result.WorkItemCount, result.DependencyCount), nil
}

// buildInspectTree builds the inspect output for a project, returning the formatted tree.
func buildInspectTree(app *App, ctx context.Context, projectID string) (string, error) {
	p, err := app.Projects.GetByID(ctx, projectID)
	if err != nil {
		return "", err
	}

	rootNodes, err := app.Nodes.ListRoots(ctx, projectID)
	if err != nil {
		return "", fmt.Errorf("listing root nodes: %w", err)
	}

	childMap := make(map[string][]*domain.PlanNode)
	workItems := make(map[string][]*domain.WorkItem)

	var fetchErr error
	var fetchChildren func(nodes []*domain.PlanNode)
	fetchChildren = func(nodes []*domain.PlanNode) {
		for _, n := range nodes {
			if fetchErr != nil {
				return
			}
			children, err := app.Nodes.ListChildren(ctx, n.ID)
			if err != nil {
				fetchErr = fmt.Errorf("listing children of node %s: %w", n.ID, err)
				return
			}
			if len(children) > 0 {
				childMap[n.ID] = children
				fetchChildren(children)
			}
			items, err := app.WorkItems.ListByNode(ctx, n.ID)
			if err != nil {
				fetchErr = fmt.Errorf("listing work items for node %s: %w", n.ID, err)
				return
			}
			if len(items) > 0 {
				workItems[n.ID] = items
			}
		}
	}
	fetchChildren(rootNodes)
	if fetchErr != nil {
		return "", fetchErr
	}

	data := formatter.ProjectInspectData{
		Project:   p,
		RootNodes: rootNodes,
		ChildMap:  childMap,
		WorkItems: workItems,
	}

	return formatter.FormatProjectInspect(data), nil
}
