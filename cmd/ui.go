package cmd

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/omar0ali/watchd/internal/api"
	"github.com/omar0ali/watchd/internal/ipc"
	"github.com/omar0ali/watchd/internal/paths"
	"github.com/spf13/cobra"
)

var uiCmd = &cobra.Command{
	Use:     "ui",
	Aliases: []string{"dash"},
	Short:   "Live dashboard of every project and app",
	Long: `Open a live, full-screen dashboard of every loaded project and app.

  ↑/↓ or j/k   move            enter   show the app's logs
  r            restart app     s       stop / start app
  R            restart all apps of the project
  c            reload the project's config
  d            stop and unload the project (asks first)
  q            quit (apps keep running)`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		if !interactive() {
			return errors.New("watchd ui needs a terminal; use watchd ps for plain output")
		}
		sock, err := paths.Socket()
		if err != nil {
			return err
		}
		_, err = tea.NewProgram(newUIModel(sock), tea.WithAltScreen(), tea.WithMouseCellMotion()).Run()
		return err
	},
}

func init() { rootCmd.AddCommand(uiCmd) }

const (
	refreshEvery = time.Second
	maxLogLines  = 2000
	statusFor    = 4 * time.Second
)

var (
	uiTitle   = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("0")).Background(lipgloss.Color("6")).Padding(0, 1)
	uiProject = lipgloss.NewStyle().Bold(true)
	uiDim     = lipgloss.NewStyle().Faint(true)
	uiCursor  = lipgloss.NewStyle().Foreground(lipgloss.Color("6")).Bold(true)
	uiSel     = lipgloss.NewStyle().Background(lipgloss.AdaptiveColor{Light: "254", Dark: "236"})
	uiGreen   = lipgloss.NewStyle().Foreground(lipgloss.Color("2"))
	uiYellow  = lipgloss.NewStyle().Foreground(lipgloss.Color("3"))
	uiRed     = lipgloss.NewStyle().Foreground(lipgloss.Color("1"))
	uiKey     = lipgloss.NewStyle().Foreground(lipgloss.Color("6"))
)

type uiMode int

const (
	modeList uiMode = iota
	modeLogs
)

type uiModel struct {
	sock     string
	apps     []api.AppStatus
	projects map[string]api.ProjectStatus
	err      error // last refresh error (usually: daemon not running)
	loaded   bool
	selected string // "project/app" under the cursor, kept across refreshes
	cursor   int
	width    int
	height   int

	status    string
	statusErr bool
	statusAt  time.Time

	confirm   string  // question awaiting y/n
	confirmed tea.Cmd // run on "y"

	mode      uiMode
	logTarget string
	logLines  []string
	logView   viewport.Model
	logCancel context.CancelFunc
	logGen    int // ignores lines from a previous log stream
}

type (
	tickMsg    struct{}
	refreshMsg struct {
		apps     []api.AppStatus
		projects []api.ProjectStatus
		err      error
	}
	actionMsg struct {
		text string
		err  error
	}
	logLineMsg struct {
		gen  int
		line api.LogLine
		ch   <-chan api.LogLine
	}
	logEndMsg struct{ gen int }
)

func newUIModel(sock string) *uiModel {
	return &uiModel{sock: sock, projects: map[string]api.ProjectStatus{}, logView: viewport.New(0, 0)}
}

func (m *uiModel) Init() tea.Cmd { return m.refresh() }

func tick() tea.Cmd {
	return tea.Tick(refreshEvery, func(time.Time) tea.Msg { return tickMsg{} })
}

func (m *uiModel) refresh() tea.Cmd {
	sock := m.sock
	return func() tea.Msg {
		var msg refreshMsg
		msg.err = ipc.Do(sock, &api.Request{Cmd: api.CmdPS, All: true}, func(r api.Message) error {
			msg.apps = append(msg.apps, r.Apps...)
			return nil
		})
		if msg.err == nil {
			msg.err = ipc.Do(sock, &api.Request{Cmd: api.CmdList}, func(r api.Message) error {
				msg.projects = append(msg.projects, r.Projects...)
				return nil
			})
		}
		return msg
	}
}

// action sends a request and reports its text (or error) in the status bar.
func (m *uiModel) action(req *api.Request) tea.Cmd {
	sock := m.sock
	return func() tea.Msg {
		var texts []string
		err := ipc.Do(sock, req, func(r api.Message) error {
			if r.Text != "" {
				texts = append(texts, r.Text)
			}
			return nil
		})
		return actionMsg{text: strings.Join(texts, "; "), err: err}
	}
}

func (m *uiModel) current() (api.AppStatus, bool) {
	if m.cursor < 0 || m.cursor >= len(m.apps) {
		return api.AppStatus{}, false
	}
	return m.apps[m.cursor], true
}

func (m *uiModel) setStatus(text string, isErr bool) {
	m.status, m.statusErr, m.statusAt = text, isErr, time.Now()
}

func (m *uiModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		m.logView.Width, m.logView.Height = msg.Width, max(1, msg.Height-2)
		m.renderLogs()
		return m, nil

	case tickMsg:
		return m, m.refresh()

	case refreshMsg:
		m.loaded = true
		m.err = msg.err
		m.apps = msg.apps
		m.projects = map[string]api.ProjectStatus{}
		for _, p := range msg.projects {
			m.projects[p.Name] = p
		}
		// Keep the cursor on the same app if it still exists.
		m.cursor = min(m.cursor, max(0, len(m.apps)-1))
		for i, a := range m.apps {
			if a.Project+"/"+a.Name == m.selected {
				m.cursor = i
			}
		}
		if a, ok := m.current(); ok {
			m.selected = a.Project + "/" + a.Name
		}
		return m, tick()

	case actionMsg:
		if msg.err != nil {
			m.setStatus(msg.err.Error(), true)
		} else {
			m.setStatus(msg.text, false)
		}
		return m, m.refresh()

	case logLineMsg:
		if msg.gen != m.logGen {
			return m, nil
		}
		atBottom := m.logView.AtBottom()
		m.logLines = append(m.logLines, formatLogLine(msg.line))
		if len(m.logLines) > maxLogLines {
			m.logLines = m.logLines[len(m.logLines)-maxLogLines:]
		}
		m.renderLogs()
		if atBottom {
			m.logView.GotoBottom()
		}
		return m, waitLog(msg.gen, msg.ch)

	case logEndMsg:
		if msg.gen == m.logGen && m.mode == modeLogs {
			m.logLines = append(m.logLines, uiDim.Render("── log stream ended ──"))
			m.renderLogs()
			m.logView.GotoBottom()
		}
		return m, nil

	case tea.KeyMsg:
		if m.confirm != "" {
			cmd := m.confirmed
			m.confirm, m.confirmed = "", nil
			if msg.String() == "y" || msg.String() == "Y" {
				return m, cmd
			}
			m.setStatus("cancelled", false)
			return m, nil
		}
		if m.mode == modeLogs {
			return m.updateLogs(msg)
		}
		return m.updateList(msg)
	}

	if m.mode == modeLogs {
		var cmd tea.Cmd
		m.logView, cmd = m.logView.Update(msg)
		return m, cmd
	}
	return m, nil
}

func (m *uiModel) updateList(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	move := func(d int) {
		if len(m.apps) == 0 {
			return
		}
		m.cursor = (m.cursor + d + len(m.apps)) % len(m.apps)
		a := m.apps[m.cursor]
		m.selected = a.Project + "/" + a.Name
	}

	switch msg.String() {
	case "q", "ctrl+c", "esc":
		return m, tea.Quit
	case "up", "k":
		move(-1)
	case "down", "j", "tab":
		move(1)
	}

	a, ok := m.current()
	if !ok {
		return m, nil
	}
	target := a.Project + "/" + a.Name
	switch msg.String() {
	case "enter", "l":
		return m, m.openLogs(target)
	case "r":
		m.setStatus("restarting "+target+"…", false)
		return m, m.action(&api.Request{Cmd: api.CmdRestart, Apps: []string{target}})
	case "s":
		if a.State == api.StateStopped {
			m.setStatus("starting "+target+"…", false)
			return m, m.action(&api.Request{Cmd: api.CmdStart, Apps: []string{target}})
		}
		m.setStatus("stopping "+target+"…", false)
		return m, m.action(&api.Request{Cmd: api.CmdStop, Apps: []string{target}})
	case "R":
		m.setStatus("restarting "+a.Project+"…", false)
		return m, m.action(&api.Request{Cmd: api.CmdRestart, Project: a.Project})
	case "c":
		m.setStatus("reloading "+a.Project+"…", false)
		return m, m.action(&api.Request{Cmd: api.CmdReload, Project: a.Project})
	case "d":
		m.confirm = fmt.Sprintf("Stop and unload project %s? (y/n)", a.Project)
		m.confirmed = m.action(&api.Request{Cmd: api.CmdDown, Apps: []string{a.Project}})
	}
	return m, nil
}

func (m *uiModel) updateLogs(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c":
		m.closeLogs()
		return m, tea.Quit
	case "q", "esc", "backspace":
		m.closeLogs()
		return m, m.refresh()
	case "r":
		m.setStatus("restarting "+m.logTarget+"…", false)
		return m, m.action(&api.Request{Cmd: api.CmdRestart, Apps: []string{m.logTarget}})
	case "G", "end", "f":
		m.logView.GotoBottom()
		return m, nil
	case "g", "home":
		m.logView.GotoTop()
		return m, nil
	}
	var cmd tea.Cmd
	m.logView, cmd = m.logView.Update(msg)
	return m, cmd
}

func (m *uiModel) openLogs(target string) tea.Cmd {
	m.closeLogs()
	m.mode = modeLogs
	m.logTarget = target
	m.logLines = nil
	m.logGen++
	m.renderLogs()

	ctx, cancel := context.WithCancel(context.Background())
	m.logCancel = cancel
	ch := make(chan api.LogLine, 256)
	sock, gen := m.sock, m.logGen
	go func() {
		defer close(ch)
		_ = ipc.DoContext(ctx, sock, &api.Request{Cmd: api.CmdLogs, Apps: []string{target}, Lines: 500, Follow: true},
			func(r api.Message) error {
				if r.Log != nil {
					select {
					case ch <- *r.Log:
					case <-ctx.Done():
						return ctx.Err()
					}
				}
				return nil
			})
	}()
	return waitLog(gen, ch)
}

func waitLog(gen int, ch <-chan api.LogLine) tea.Cmd {
	return func() tea.Msg {
		l, ok := <-ch
		if !ok {
			return logEndMsg{gen: gen}
		}
		return logLineMsg{gen: gen, line: l, ch: ch}
	}
}

func (m *uiModel) closeLogs() {
	if m.logCancel != nil {
		m.logCancel()
		m.logCancel = nil
	}
	m.logGen++
	m.mode = modeList
}

func (m *uiModel) renderLogs() {
	if m.width == 0 {
		return
	}
	wrap := lipgloss.NewStyle().Width(m.width)
	var b strings.Builder
	for i, l := range m.logLines {
		if i > 0 {
			b.WriteByte('\n')
		}
		b.WriteString(wrap.Render(l))
	}
	m.logView.SetContent(b.String())
}

func formatLogLine(l api.LogLine) string {
	ts := uiDim.Render(l.Time.Format("15:04:05"))
	switch l.Stream {
	case api.StreamSystem:
		return ts + " " + uiDim.Render("» "+l.Text)
	case api.StreamStderr:
		return ts + " " + uiYellow.Render("│") + " " + l.Text
	default:
		return ts + " " + uiDim.Render("│") + " " + l.Text
	}
}

func (m *uiModel) View() string {
	if m.width == 0 {
		return ""
	}
	if m.mode == modeLogs {
		header := uiTitle.Render("watchd") + " " + uiProject.Render(m.logTarget) + uiDim.Render("  logs")
		if !m.logView.AtBottom() {
			header += uiYellow.Render("  (paused, G to follow)")
		}
		return header + "\n" + m.logView.View() + "\n" + m.footer(
			"↑/↓ scroll", "G follow", "r restart", "esc back", "ctrl+c quit")
	}
	return m.listView()
}

func (m *uiModel) listView() string {
	var b strings.Builder

	running := 0
	for _, a := range m.apps {
		if a.State == api.StateRunning {
			running++
		}
	}
	b.WriteString(uiTitle.Render("watchd"))
	if m.loaded && m.err == nil {
		b.WriteString(uiDim.Render(fmt.Sprintf("  %d project(s) · %d/%d apps running", len(m.projects), running, len(m.apps))))
	}
	b.WriteString("\n\n")

	switch {
	case !m.loaded:
		b.WriteString(uiDim.Render("  connecting…\n"))
	case errors.Is(m.err, ipc.ErrNoDaemon):
		b.WriteString("  The daemon is not running.\n")
		b.WriteString(uiDim.Render("  Start a project with `watchd up` and it will show up here.\n"))
	case m.err != nil:
		b.WriteString(uiRed.Render("  "+m.err.Error()) + "\n")
	case len(m.apps) == 0:
		b.WriteString("  No projects are loaded.\n")
		b.WriteString(uiDim.Render("  Run `watchd up` in a project directory and it will show up here.\n"))
	default:
		nameW := 4
		for _, a := range m.apps {
			nameW = max(nameW, len(a.Name))
		}
		project := ""
		for i, a := range m.apps {
			if a.Project != project {
				if project != "" {
					b.WriteByte('\n')
				}
				project = a.Project
				p := m.projects[a.Project]
				header := " " + uiProject.Render(a.Project) + uiDim.Render("  "+tildePath(p.ConfigPath))
				b.WriteString(lipgloss.NewStyle().MaxWidth(m.width).Render(header) + "\n")
			}
			b.WriteString(m.appRow(a, i == m.cursor, nameW) + "\n")
		}
	}

	body := b.String()
	// Push the footer to the bottom of the screen.
	if pad := m.height - lipgloss.Height(body) - 2; pad > 0 {
		body += strings.Repeat("\n", pad)
	}
	return body + "\n" + m.footer(
		"↑/↓ move", "enter logs", "r restart", "s stop/start", "R restart project", "c reload", "d down", "q quit")
}

func (m *uiModel) appRow(a api.AppStatus, selected bool, nameW int) string {
	icon, style := "○", uiDim
	switch a.State {
	case api.StateRunning:
		icon, style = "●", uiGreen
		if a.Health == api.HealthUnhealthy {
			style = uiRed
		} else if a.Health == api.HealthStarting {
			style = uiYellow
		}
	case api.StateWaiting, api.StateBackoff:
		icon, style = "◐", uiYellow
	case api.StateFailed:
		icon, style = "✗", uiRed
	case api.StateExited:
		icon = "◌"
	}
	state := stateCell(a).text

	pid, uptime := "", ""
	if a.PID != 0 {
		pid = "pid " + strconv.Itoa(a.PID)
		uptime = "up " + humanDuration(time.Since(a.StartedAt))
	}
	restarts := ""
	if a.Restarts > 0 {
		restarts = fmt.Sprintf("↻ %d", a.Restarts)
	}

	cursor := "  "
	if selected {
		cursor = uiCursor.Render("❯ ")
	}
	row := fmt.Sprintf("%s%s %-*s  %s  %s  %s  %s",
		cursor,
		style.Render(icon),
		nameW, a.Name,
		style.Render(fmt.Sprintf("%-20s", state)),
		uiDim.Render(fmt.Sprintf("%-10s", pid)),
		uiDim.Render(fmt.Sprintf("%-10s", uptime)),
		uiDim.Render(restarts))
	if a.State == api.StateFailed && a.LastError != "" {
		row += "  " + uiRed.Render(a.LastError)
	}
	if selected {
		row = uiSel.Width(m.width).Render(row)
	}
	return lipgloss.NewStyle().MaxWidth(m.width).Render(row)
}

func (m *uiModel) footer(keys ...string) string {
	if m.confirm != "" {
		return uiYellow.Bold(true).Render(" " + m.confirm)
	}
	if m.status != "" && time.Since(m.statusAt) < statusFor {
		if m.statusErr {
			return uiRed.Render(" ✗ " + m.status)
		}
		return uiGreen.Render(" ✓ " + m.status)
	}
	parts := make([]string, len(keys))
	for i, k := range keys {
		key, desc, _ := strings.Cut(k, " ")
		parts[i] = uiKey.Render(key) + " " + uiDim.Render(desc)
	}
	return lipgloss.NewStyle().MaxWidth(m.width).Render(" " + strings.Join(parts, uiDim.Render(" · ")))
}
