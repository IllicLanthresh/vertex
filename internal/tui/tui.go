package tui

import (
	"context"
	"fmt"
	"io"
	"log"
	"strings"
	"time"

	"github.com/IllicLanthresh/vertex/internal/config"
	"github.com/IllicLanthresh/vertex/internal/traffic"
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

const (
	tickInterval  = 2 * time.Second
	maxLogEntries = 1500
)

// App represents the TUI application.
type App struct {
	cfg *config.Config
	gen *traffic.Generator

	program *tea.Program
	logger  *logWriter

	prevLogOutput io.Writer
}

// New creates a new TUI app.
func New(cfg *config.Config, gen *traffic.Generator) *App {
	return &App{cfg: cfg, gen: gen}
}

// Run starts the TUI (blocking). Returns nil on clean exit.
func (a *App) Run(ctx context.Context) error {
	m := newModel(ctx, a.cfg, a.gen)
	p := tea.NewProgram(m, tea.WithAltScreen())
	a.program = p

	a.prevLogOutput = log.Writer()
	a.logger = &logWriter{app: a}
	log.SetOutput(a.logger)
	defer log.SetOutput(a.prevLogOutput)

	_, err := p.Run()
	if err != nil {
		return err
	}

	return nil
}

type refreshMsg struct {
	running    bool
	interfaces []interfaceView
}

type interfaceView struct {
	name        string
	isUp        bool
	bytesSent   uint64
	bytesRecv   uint64
	vdevCount   int
	vdevActive  int
	ipAddresses []string
}

type actionDoneMsg struct {
	action string
	err    error
}

type logMsg struct {
	line string
}

type ctxDoneMsg struct{}

type quitReadyMsg struct {
	err error
}

type model struct {
	ctx context.Context
	cfg *config.Config
	gen *traffic.Generator

	keys   keyMap
	styles styles

	width  int
	height int

	logs []string
	vp   viewport.Model

	running    bool
	interfaces []interfaceView

	busy      bool
	busyLabel string
	quitting  bool

	discoverTried bool
}

func newModel(ctx context.Context, cfg *config.Config, gen *traffic.Generator) model {
	vp := viewport.New(0, 0)
	vp.YPosition = 0
	vp.MouseWheelEnabled = true

	return model{
		ctx:        ctx,
		cfg:        cfg,
		gen:        gen,
		keys:       defaultKeyMap(),
		styles:     defaultStyles(),
		vp:         vp,
		interfaces: make([]interfaceView, 0),
		logs:       make([]string, 0, 128),
	}
}

func (m model) Init() tea.Cmd {
	return tea.Batch(m.waitForContextCancel(), m.refreshNow(), m.tick())
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.resizeViewport()

	case tea.KeyMsg:
		if key.Matches(msg, m.keys.Quit) {
			if m.quitting {
				return m, nil
			}
			m.quitting = true
			m.busy = true
			m.busyLabel = "Shutting down"
			return m, m.stopAndQuit()
		}

		if key.Matches(msg, m.keys.Up) {
			m.vp.LineUp(1)
			return m, nil
		}
		if key.Matches(msg, m.keys.Down) {
			m.vp.LineDown(1)
			return m, nil
		}
		if key.Matches(msg, m.keys.PageUp) {
			m.vp.HalfViewUp()
			return m, nil
		}
		if key.Matches(msg, m.keys.PageDown) {
			m.vp.HalfViewDown()
			return m, nil
		}

		if m.busy {
			return m, nil
		}

		switch {
		case key.Matches(msg, m.keys.Start):
			m.busy = true
			m.busyLabel = "Starting"
			return m, m.startTraffic()

		case key.Matches(msg, m.keys.Stop):
			m.busy = true
			m.busyLabel = "Stopping"
			return m, m.stopTraffic()

		case key.Matches(msg, m.keys.Restart):
			m.busy = true
			m.busyLabel = "Restarting"
			return m, m.restartTraffic()

		case key.Matches(msg, m.keys.IncVDev):
			m.cfg.NetworkSimulation.VirtualDevices++
			m.gen.UpdateConfig(m.cfg)
			m.appendLog(fmt.Sprintf("config updated: virtual_devices=%d", m.cfg.NetworkSimulation.VirtualDevices))

		case key.Matches(msg, m.keys.DecVDev):
			if m.cfg.NetworkSimulation.VirtualDevices > 1 {
				m.cfg.NetworkSimulation.VirtualDevices--
				m.gen.UpdateConfig(m.cfg)
				m.appendLog(fmt.Sprintf("config updated: virtual_devices=%d", m.cfg.NetworkSimulation.VirtualDevices))
			}

		case key.Matches(msg, m.keys.IncDepth):
			m.cfg.MaxDepth++
			m.gen.UpdateConfig(m.cfg)
			m.appendLog(fmt.Sprintf("config updated: max_depth=%d", m.cfg.MaxDepth))

		case key.Matches(msg, m.keys.DecDepth):
			if m.cfg.MaxDepth > 1 {
				m.cfg.MaxDepth--
				m.gen.UpdateConfig(m.cfg)
				m.appendLog(fmt.Sprintf("config updated: max_depth=%d", m.cfg.MaxDepth))
			}

		case key.Matches(msg, m.keys.IncSleep):
			m.cfg.MaxSleep++
			m.gen.UpdateConfig(m.cfg)
			m.appendLog(fmt.Sprintf("config updated: sleep=%d-%ds", m.cfg.MinSleep, m.cfg.MaxSleep))

		case key.Matches(msg, m.keys.DecSleep):
			if m.cfg.MaxSleep > m.cfg.MinSleep {
				m.cfg.MaxSleep--
				m.gen.UpdateConfig(m.cfg)
				m.appendLog(fmt.Sprintf("config updated: sleep=%d-%ds", m.cfg.MinSleep, m.cfg.MaxSleep))
			}
		}

	case refreshMsg:
		m.running = msg.running
		m.interfaces = msg.interfaces
		if len(m.interfaces) == 0 && !m.discoverTried {
			m.discoverTried = true
			return m, m.discoverInterfaces()
		}
		return m, m.tick()

	case actionDoneMsg:
		m.busy = false
		m.busyLabel = ""
		if msg.err != nil {
			m.appendLog(fmt.Sprintf("%s failed: %v", msg.action, msg.err))
		} else {
			m.appendLog(fmt.Sprintf("%s complete", msg.action))
		}
		return m, m.refreshNow()

	case logMsg:
		m.appendLog(msg.line)

	case ctxDoneMsg:
		if m.quitting {
			return m, nil
		}
		m.quitting = true
		m.busy = true
		m.busyLabel = "Shutting down"
		return m, m.stopAndQuit()

	case quitReadyMsg:
		if msg.err != nil {
			m.appendLog(fmt.Sprintf("shutdown warning: %v", msg.err))
		}
		return m, tea.Quit
	}

	return m, nil
}

func (m model) View() string {
	if m.width == 0 || m.height == 0 {
		return "loading..."
	}

	header := m.renderHeader()
	body := m.renderBody()
	logs := m.renderLogsPanel()
	footer := m.renderFooter()

	view := lipgloss.JoinVertical(lipgloss.Left, header, body, logs, footer)
	return m.styles.appFrame.Width(m.width).Render(view)
}

func (m *model) appendLog(line string) {
	line = strings.TrimSpace(line)
	if line == "" {
		return
	}

	wasAtBottom := m.vp.AtBottom()
	m.logs = append(m.logs, line)
	if len(m.logs) > maxLogEntries {
		m.logs = m.logs[len(m.logs)-maxLogEntries:]
	}

	m.vp.SetContent(strings.Join(m.logs, "\n"))
	if wasAtBottom {
		m.vp.GotoBottom()
	}
}

func (m *model) resizeViewport() {
	if m.width == 0 || m.height == 0 {
		return
	}

	panelWidth := max(20, m.width-2)
	logHeight := max(6, m.height-20)
	m.vp.Width = panelWidth - 2
	m.vp.Height = logHeight - 2
	m.vp.SetContent(strings.Join(m.logs, "\n"))
	if m.vp.AtBottom() {
		m.vp.GotoBottom()
	}
}

func (m model) renderHeader() string {
	title := m.styles.headerTitle.Render("VERTEX TRAFFIC GENERATOR")

	statusText := "Stopped"
	statusStyle := m.styles.statusStopped
	if m.busy {
		statusText = m.busyLabel
		statusStyle = m.styles.statusTransition
	} else if m.running {
		statusText = "Running"
		statusStyle = m.styles.statusRunning
	}

	status := statusStyle.Render(statusText)
	left := title
	right := status

	gap := m.width - lipgloss.Width(left) - lipgloss.Width(right) - 6
	if gap < 1 {
		gap = 1
	}
	content := left + strings.Repeat(" ", gap) + right
	return m.styles.headerBox.Width(m.width - 2).Render(content)
}

func (m model) renderBody() string {
	leftWidth := max(30, (m.width-3)/2)
	rightWidth := max(30, m.width-leftWidth-3)

	interfaces := m.renderInterfacesPanel(leftWidth)
	config := m.renderConfigPanel(rightWidth)
	return lipgloss.JoinHorizontal(lipgloss.Top, interfaces, config)
}

func (m model) renderInterfacesPanel(width int) string {
	var b strings.Builder
	b.WriteString(m.styles.panelTitle.Render("Interfaces"))
	b.WriteString("\n")

	if len(m.interfaces) == 0 {
		b.WriteString(m.styles.muted.Render("No interfaces discovered yet."))
	} else {
		for i, iface := range m.interfaces {
			if i > 0 {
				b.WriteString("\n\n")
			}

			status := m.styles.ifaceDown.Render("○ DOWN")
			if iface.isUp {
				status = m.styles.ifaceUp.Render("● UP")
			}

			b.WriteString(fmt.Sprintf("%s  %s", m.styles.ifaceName.Render(iface.name), status))
			b.WriteString("\n")
			b.WriteString(fmt.Sprintf("%s %s", m.styles.metricLabel.Render("TX:"), m.styles.metricValue.Render(humanBytes(iface.bytesSent))))
			b.WriteString("\n")
			b.WriteString(fmt.Sprintf("%s %s", m.styles.metricLabel.Render("RX:"), m.styles.metricValue.Render(humanBytes(iface.bytesRecv))))
			b.WriteString("\n")
			b.WriteString(fmt.Sprintf("%s %s", m.styles.metricLabel.Render("VDevs:"), m.styles.metricValue.Render(fmt.Sprintf("%d/%d active", iface.vdevActive, iface.vdevCount))))
			if len(iface.ipAddresses) > 0 {
				b.WriteString("\n")
				b.WriteString(fmt.Sprintf("%s %s", m.styles.metricLabel.Render("IPs:"), m.styles.muted.Render(strings.Join(iface.ipAddresses, ", "))))
			}
		}
	}

	return m.styles.panel.Width(width).Render(b.String())
}

func (m model) renderConfigPanel(width int) string {
	cfg := m.cfg
	var b strings.Builder
	b.WriteString(m.styles.panelTitle.Render("Configuration"))
	b.WriteString("\n")
	b.WriteString(configRow(m.styles, "Virtual Devices", fmt.Sprintf("%d", cfg.NetworkSimulation.VirtualDevices)))
	b.WriteString("\n")
	b.WriteString(configRow(m.styles, "Sleep Range", fmt.Sprintf("%d-%ds", cfg.MinSleep, cfg.MaxSleep)))
	b.WriteString("\n")
	b.WriteString(configRow(m.styles, "Max Depth", fmt.Sprintf("%d", cfg.MaxDepth)))
	b.WriteString("\n")
	b.WriteString(configRow(m.styles, "DHCP Retries", fmt.Sprintf("%d", cfg.NetworkSimulation.DHCPRetries)))
	b.WriteString("\n")
	b.WriteString(configRow(m.styles, "Target URLs", fmt.Sprintf("%d", len(cfg.RootURLs))))
	b.WriteString("\n")
	b.WriteString(configRow(m.styles, "Interfaces", fmt.Sprintf("%d configured", len(cfg.NetworkSimulation.Interfaces))))
	b.WriteString("\n\n")
	b.WriteString(m.styles.muted.Render("Config keys: </> vdev  [/] depth  +/- sleep"))

	return m.styles.panel.Width(width).Render(b.String())
}

func (m model) renderLogsPanel() string {
	height := max(6, m.height-20)
	content := m.styles.panelTitle.Render("Logs") + "\n" + m.vp.View()
	return m.styles.panel.Width(m.width - 2).Height(height).Render(content)
}

func (m model) renderFooter() string {
	parts := []string{
		helpPair(m.styles, "[s]", "start"),
		helpPair(m.styles, "[x]", "stop"),
		helpPair(m.styles, "[r]", "restart"),
		helpPair(m.styles, "[q]", "quit"),
		helpPair(m.styles, "[↑↓]", "scroll"),
		"│",
		helpPair(m.styles, "[</>]", "vdev"),
		helpPair(m.styles, "[/]", "depth"),
		helpPair(m.styles, "[+/-]", "sleep"),
	}
	return m.styles.footer.Width(m.width - 2).Render(strings.Join(parts, "   "))
}

func (m model) tick() tea.Cmd {
	return tea.Tick(tickInterval, func(time.Time) tea.Msg {
		return refreshSnapshot(m.gen)
	})
}

func (m model) refreshNow() tea.Cmd {
	return func() tea.Msg {
		return refreshSnapshot(m.gen)
	}
}

func (m model) discoverInterfaces() tea.Cmd {
	return func() tea.Msg {
		if err := m.gen.GetInterfaceManager().DiscoverPhysicalInterfaces(); err != nil {
			return logMsg{line: fmt.Sprintf("interface discovery failed: %v", err)}
		}
		return refreshSnapshot(m.gen)
	}
}

func (m model) startTraffic() tea.Cmd {
	return func() tea.Msg {
		err := m.gen.Start(m.ctx)
		return actionDoneMsg{action: "start", err: err}
	}
}

func (m model) stopTraffic() tea.Cmd {
	return func() tea.Msg {
		err := m.gen.Stop()
		return actionDoneMsg{action: "stop", err: err}
	}
}

func (m model) restartTraffic() tea.Cmd {
	return func() tea.Msg {
		if m.gen.IsRunning() {
			if err := m.gen.Stop(); err != nil {
				return actionDoneMsg{action: "restart", err: err}
			}
		}
		err := m.gen.Start(m.ctx)
		return actionDoneMsg{action: "restart", err: err}
	}
}

func (m model) stopAndQuit() tea.Cmd {
	return func() tea.Msg {
		if m.gen.IsRunning() {
			if err := m.gen.Stop(); err != nil {
				return quitReadyMsg{err: err}
			}
		}
		return quitReadyMsg{}
	}
}

func (m model) waitForContextCancel() tea.Cmd {
	return func() tea.Msg {
		<-m.ctx.Done()
		return ctxDoneMsg{}
	}
}

func refreshSnapshot(gen *traffic.Generator) refreshMsg {
	mgr := gen.GetInterfaceManager()
	interfaces := mgr.GetPhysicalInterfaces()
	rows := make([]interfaceView, 0, len(interfaces))

	for _, iface := range interfaces {
		stats, err := mgr.GetInterfaceStats(iface)
		if err != nil {
			continue
		}

		vdevs := mgr.GetVirtualDevices(iface)
		active := 0
		for _, v := range vdevs {
			if strings.TrimSpace(v.IP) != "" {
				active++
			}
		}

		rows = append(rows, interfaceView{
			name:        iface,
			isUp:        stats.IsUp,
			bytesSent:   stats.BytesSent,
			bytesRecv:   stats.BytesRecv,
			vdevCount:   len(vdevs),
			vdevActive:  active,
			ipAddresses: stats.IPs,
		})
	}

	return refreshMsg{running: gen.IsRunning(), interfaces: rows}
}

type logWriter struct {
	app *App
}

func (w *logWriter) Write(p []byte) (int, error) {
	if w.app == nil || w.app.program == nil {
		return len(p), nil
	}

	lines := strings.Split(string(p), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		w.app.program.Send(logMsg{line: line})
	}

	return len(p), nil
}

func configRow(s styles, keyName, value string) string {
	return fmt.Sprintf("%s: %s", s.metricLabel.Render(keyName), s.metricValue.Render(value))
}

func helpPair(s styles, k, text string) string {
	return s.helpKey.Render(k) + s.helpText.Render(text)
}

func humanBytes(v uint64) string {
	const unit = 1024
	if v < unit {
		return fmt.Sprintf("%d B", v)
	}

	div, exp := uint64(unit), 0
	for n := v / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}

	value := float64(v) / float64(div)
	return fmt.Sprintf("%.1f %ciB", value, "KMGTPE"[exp])
}
