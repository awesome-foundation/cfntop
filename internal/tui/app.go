package tui

import (
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	cfnaws "github.com/awesome-foundation/cfntop/internal/aws"
)

var spinnerFrames = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}

// Messages
type pollMsg PollResult
type tickMsg time.Time
type spinnerTickMsg time.Time
type detailsMsg struct {
	stackName string
	resources []ResourceState
	err       error
}

// Model is the main TUI model.
type Model struct {
	client          *cfnaws.Client
	ecsClient       ECSAPI
	stacks          []StackState
	cursor          int
	err             error
	width           int
	height          int
	interval        time.Duration
	humanize        bool
	quitting        bool
	loading         bool
	firstPoll       bool
	manualExpanded  map[string]bool
	loadingStacks   map[string]bool      // stacks currently fetching details
	spinnerIndex    int
	lastDetailFetch map[string]time.Time // last time we fetched details per stack
}

// NewModel creates a new TUI model.
func NewModel(client *cfnaws.Client, ecsClient ECSAPI, interval time.Duration, humanize bool) Model {
	return Model{
		client:          client,
		ecsClient:       ecsClient,
		interval:        interval,
		humanize:        humanize,
		loading:         true,
		firstPoll:       true,
		manualExpanded:  make(map[string]bool),
		loadingStacks:   make(map[string]bool),
		lastDetailFetch: make(map[string]time.Time),
	}
}

// NewDemoModel creates a model preloaded with fake data for screenshots.
func NewDemoModel() Model {
	stacks := DemoStacks()
	return Model{
		stacks:          stacks,
		interval:        999 * time.Hour, // never auto-poll
		humanize:        true,
		manualExpanded:  make(map[string]bool),
		loadingStacks:   make(map[string]bool),
		lastDetailFetch: make(map[string]time.Time),
	}
}

func (m Model) Init() tea.Cmd {
	if m.client == nil {
		// Demo mode — no polling, just render
		return nil
	}
	return tea.Batch(m.poll(), m.tick(), m.spinnerTick())
}

func (m Model) spinnerTick() tea.Cmd {
	return tea.Tick(80*time.Millisecond, func(t time.Time) tea.Msg {
		return spinnerTickMsg(t)
	})
}

const inactiveDetailInterval = time.Minute

func (m *Model) poll() tea.Cmd {
	now := time.Now()
	fetchDetails := make(map[string]bool)
	for _, s := range m.stacks {
		if !s.Expanded {
			continue
		}
		name := s.Summary.StackName
		if s.Active {
			// Active stacks: always fetch details
			fetchDetails[name] = true
			m.loadingStacks[name] = true
		} else if now.Sub(m.lastDetailFetch[name]) >= inactiveDetailInterval {
			// Inactive expanded stacks: fetch once per minute
			fetchDetails[name] = true
			m.loadingStacks[name] = true
		}
	}
	client, ecsClient := m.client, m.ecsClient
	return func() tea.Msg {
		return pollMsg(Poll(client, ecsClient, fetchDetails))
	}
}

func (m Model) tick() tea.Cmd {
	return tea.Tick(m.interval, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

func (m Model) fetchDetails(stackName string) tea.Cmd {
	return func() tea.Msg {
		rs, err := FetchStackDetails(m.client, m.ecsClient, stackName)
		return detailsMsg{stackName: stackName, resources: rs, err: err}
	}
}

const (
	autoExpandWindow = 3 * time.Hour
	maxAutoExpand    = 5
)

// autoExpandInitial runs on the first poll to expand active stacks
// and up to 5 of the freshest stacks updated within the last 3 hours.
func (m *Model) autoExpandInitial() {
	now := time.Now()
	expanded := 0

	// Stacks are already sorted: active first, then by last update descending.
	for i := range m.stacks {
		// Always expand active stacks (don't count against limit)
		if m.stacks[i].Active {
			m.stacks[i].Expanded = true
			continue
		}

		if expanded >= maxAutoExpand {
			break
		}

		ts := m.stacks[i].Summary.UpdatedAt
		if ts == "" {
			ts = m.stacks[i].Summary.CreatedAt
		}
		t, err := time.Parse("2006-01-02T15:04:05Z", ts)
		if err != nil {
			continue
		}
		if now.Sub(t) <= autoExpandWindow {
			m.stacks[i].Expanded = true
			expanded++
		}
	}
}

func (m *Model) fmtTime(ts string) string {
	if m.humanize {
		return HumanizeTime(ts)
	}
	return ts
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			m.quitting = true
			return m, tea.Quit
		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			}
		case "down", "j":
			if m.cursor < len(m.stacks)-1 {
				m.cursor++
			}
		case "enter", " ":
			if m.cursor < len(m.stacks) {
				name := m.stacks[m.cursor].Summary.StackName
				m.stacks[m.cursor].Expanded = !m.stacks[m.cursor].Expanded
				if m.stacks[m.cursor].Expanded {
					m.manualExpanded[name] = true
					if len(m.stacks[m.cursor].Resources) == 0 {
						m.loadingStacks[name] = true
						return m, tea.Batch(m.fetchDetails(name), m.spinnerTick())
					}
				} else {
					delete(m.manualExpanded, name)
				}
			}
		case "r":
			m.loading = true
			return m, tea.Batch(m.poll(), m.spinnerTick())
		}

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

	case pollMsg:
		m.loading = false
		now := time.Now()
		// Clear loading spinners and record fetch times
		for k := range m.loadingStacks {
			m.lastDetailFetch[k] = now
			delete(m.loadingStacks, k)
		}
		result := PollResult(msg)
		if result.Err != nil {
			m.err = result.Err
		} else {
			// Preserve previous state
			prev := make(map[string]StackState)
			for _, s := range m.stacks {
				prev[s.Summary.StackName] = s
			}

			m.stacks = result.Stacks

			if m.firstPoll {
				m.firstPoll = false
				m.autoExpandInitial()
			} else {
				// Auto-expand newly active stacks
				for i := range m.stacks {
					name := m.stacks[i].Summary.StackName
					if m.stacks[i].Active {
						m.stacks[i].Expanded = true
					} else if old, ok := prev[name]; ok {
						// Preserve expanded state from manual toggle or previous auto-expand
						m.stacks[i].Expanded = old.Expanded
					}
					// Keep cached resources unless the poller fetched fresh ones
					if len(m.stacks[i].Resources) == 0 {
						if old, ok := prev[name]; ok && len(old.Resources) > 0 {
							m.stacks[i].Resources = old.Resources
						}
					}
				}
			}

			if m.cursor >= len(m.stacks) && len(m.stacks) > 0 {
				m.cursor = len(m.stacks) - 1
			}
			m.err = nil
		}

	case detailsMsg:
		delete(m.loadingStacks, msg.stackName)
		m.lastDetailFetch[msg.stackName] = time.Now()
		if msg.err == nil {
			for i := range m.stacks {
				if m.stacks[i].Summary.StackName == msg.stackName {
					m.stacks[i].Resources = msg.resources
					break
				}
			}
		}

	case spinnerTickMsg:
		m.spinnerIndex = (m.spinnerIndex + 1) % len(spinnerFrames)
		if m.loading || len(m.loadingStacks) > 0 {
			return m, m.spinnerTick()
		}

	case tickMsg:
		return m, tea.Batch(m.poll(), m.tick(), m.spinnerTick())
	}

	return m, nil
}

func (m Model) View() string {
	if m.quitting {
		return ""
	}

	var b strings.Builder

	// Title bar
	title := titleStyle.Render("cfntop")
	status := ""
	if m.loading {
		status = " " + spinnerFrames[m.spinnerIndex]
	}
	if m.err != nil {
		status = " " + redStyle.Render(m.err.Error())
	}
	b.WriteString(title + status + "\n\n")

	if len(m.stacks) == 0 && !m.loading {
		b.WriteString("  No stacks found.\n")
	}

	// Column header
	if len(m.stacks) > 0 {
		header := fmt.Sprintf("  %-20s %-38s %s", "UPDATED", "STATUS", "STACK")
		b.WriteString(headerStyle.Render(header) + "\n")
	}

	// Stacks
	visible := m.height - 5
	if visible < 5 {
		visible = 20
	}
	lines := 0

	for i, s := range m.stacks {
		if lines >= visible {
			fmt.Fprintf(&b, "  ... and %d more\n", len(m.stacks)-i)
			break
		}

		updated := s.Summary.UpdatedAt
		if updated == "" {
			updated = s.Summary.CreatedAt
		}
		updated = m.fmtTime(updated)

		arrow := "▸"
		if m.loadingStacks[s.Summary.StackName] {
			arrow = spinnerFrames[m.spinnerIndex]
		} else if s.Expanded {
			arrow = "▾"
		}

		styledStatus := statusStyle(s.Summary.Status).Render(s.Summary.Status)
		line := fmt.Sprintf("  %s %-18s  %-36s %s",
			arrow, updated, styledStatus, s.Summary.StackName)

		if i == m.cursor {
			if m.width > len(line) {
				line += strings.Repeat(" ", m.width-len(line))
			}
			line = selectedStyle.Render(line)
		}

		b.WriteString(line + "\n")
		lines++

		// Expanded resources
		if s.Expanded && len(s.Resources) > 0 {
			for _, rs := range s.Resources {
				if lines >= visible {
					break
				}

				r := rs.Resource
				rTime := m.fmtTime(r.LastUpdated)

				prefix := "      "
				if rs.Deleted {
					prefix = "    × "
				}

				// Pick the style for this resource's fields
				var rStyle func(string) string
				if rs.Deleted {
					rStyle = func(s string) string { return deletedStyle.Render(s) }
				} else if !rs.Touched {
					rStyle = func(s string) string { return untouchedStyle.Render(s) }
				} else {
					st := statusStyle(r.Status)
					rStyle = func(s string) string { return st.Render(s) }
				}

				// Pad status before styling so visual width is consistent
				paddedStatus := fmt.Sprintf("%-26s", r.Status)
				rLine := fmt.Sprintf("%s%-18s  %s %-25s  %s",
					prefix, rTime, rStyle(paddedStatus), ShortenType(r.Type), r.LogicalID)

				if !rs.Touched && !rs.Deleted {
					// Grey out the non-status parts too
					rLine = fmt.Sprintf("%s%s  %s %s  %s",
						untouchedStyle.Render(prefix),
						untouchedStyle.Render(fmt.Sprintf("%-18s", rTime)),
						rStyle(paddedStatus),
						untouchedStyle.Render(fmt.Sprintf("%-25s", ShortenType(r.Type))),
						untouchedStyle.Render(r.LogicalID))
				} else if rs.Deleted {
					rLine = fmt.Sprintf("%s%s  %s %s  %s",
						deletedStyle.Render(prefix),
						deletedStyle.Render(fmt.Sprintf("%-18s", rTime)),
						rStyle(paddedStatus),
						deletedStyle.Render(fmt.Sprintf("%-25s", ShortenType(r.Type))),
						deletedStyle.Render(r.LogicalID))
				}
				b.WriteString(rLine + "\n")
				lines++

				// ECS service deployments
				if rs.ECS != nil && len(rs.ECS.Deployments) > 0 {
					for _, d := range rs.ECS.Deployments {
						if lines >= visible {
							break
						}
						dTime := m.fmtTime(d.CreatedAt)
						taskInfo := fmt.Sprintf("%d/%d desired", d.Running, d.Desired)
						if d.Pending > 0 {
							taskInfo += fmt.Sprintf(", %d pending", d.Pending)
						}
						if d.Failed > 0 {
							taskInfo += ", " + redStyle.Render(fmt.Sprintf("%d failed", d.Failed))
						}
						dStatus := statusStyle(d.RolloutState).Render(d.Status)
						dLine := fmt.Sprintf("        %-18s  %-10s %-30s  %s",
							dTime, dStatus, d.TaskDefinition, taskInfo)
						b.WriteString(dLine + "\n")
						lines++

						// Failed tasks for this deployment
						for _, ft := range d.FailedTasks {
							if lines >= visible {
								break
							}
							ftTime := m.fmtTime(ft.StoppedAt)
							reason := ft.StopReason
							if reason == "" {
								reason = ft.StopCode
							}
							ftLine := fmt.Sprintf("          %-16s %s  %s",
								ftTime, redStyle.Render("STOPPED"), redStyle.Render(reason))
							b.WriteString(ftLine + "\n")
							lines++
						}
					}
				}

				// Show event history for errored resources (newest first)
				if rs.HasError {
					for ei := len(rs.Events) - 1; ei >= 0; ei-- {
						e := rs.Events[ei]
						if lines >= visible {
							break
						}
						eTime := m.fmtTime(e.Timestamp)
						eStatus := statusStyle(e.Status).Render(e.Status)
						reason := ""
						if e.StatusReason != "" {
							reason = "  " + redStyle.Render(e.StatusReason)
						}
						eLine := fmt.Sprintf("        %-18s  %s%s", eTime, eStatus, reason)
						b.WriteString(eLine + "\n")
						lines++
					}
				}
			}
		}
	}

	// Help bar
	b.WriteString("\n")
	help := "↑/↓ navigate  enter expand/collapse  r refresh  q quit"
	if m.width > 0 {
		help = helpStyle.Render(lipgloss.PlaceHorizontal(m.width, lipgloss.Left, help))
	} else {
		help = helpStyle.Render(help)
	}
	b.WriteString(help)

	return b.String()
}
