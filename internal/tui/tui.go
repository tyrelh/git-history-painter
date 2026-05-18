// Package tui provides the interactive Bubble Tea program for painting
// the contribution grid.
package tui

import (
	"fmt"
	"strings"

	"git-history/internal/grid"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// Model is the Bubble Tea model. After the program exits, callers should
// check Submitted to decide whether to write commits.
type Model struct {
	grid      *grid.Grid
	cursorRow int
	cursorCol int
	painting  int // -1 = not painting; otherwise the level being painted
	submitted bool
	quitting  bool
	width     int
	height    int
	loaded    int // number of tool commits found in the repo on startup
}

// New builds the model. loaded is the number of pre-existing tool commits
// the grid was populated from (0 for a fresh repo); it is displayed in the
// status bar so the user knows whether they're editing or starting fresh.
func New(g *grid.Grid, loaded int) Model {
	return Model{grid: g, painting: -1, loaded: loaded}
}

func (m Model) Submitted() bool  { return m.submitted }
func (m Model) Grid() *grid.Grid { return m.grid }

func (m Model) Init() tea.Cmd { return nil }

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
	case tea.KeyMsg:
		return m.handleKey(msg)
	}
	return m, nil
}

func (m Model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c", "q":
		m.quitting = true
		return m, tea.Quit
	case "up", "k":
		if m.cursorRow > 0 {
			m.cursorRow--
		}
	case "down", "j":
		if m.cursorRow < grid.Rows-1 {
			m.cursorRow++
		}
	case "left", "h":
		if m.cursorCol > 0 {
			m.cursorCol--
		}
	case "right", "l":
		if m.cursorCol < grid.Cols-1 {
			m.cursorCol++
		}
	case "0", "1", "2", "3", "4":
		level := int(msg.String()[0] - '0')
		m.grid.Set(m.cursorRow, m.cursorCol, level)
	case " ":
		m.grid.Cycle(m.cursorRow, m.cursorCol)
	case "-", "_":
		cur := m.grid.Cells[m.cursorRow][m.cursorCol]
		m.grid.Set(m.cursorRow, m.cursorCol, cur-1)
	case "+", "=":
		cur := m.grid.Cells[m.cursorRow][m.cursorCol]
		m.grid.Set(m.cursorRow, m.cursorCol, cur+1)
	case "x":
		m.grid.Set(m.cursorRow, m.cursorCol, 0)
	case "X":
		// Clear all cells.
		for r := 0; r < grid.Rows; r++ {
			for c := 0; c < grid.Cols; c++ {
				m.grid.Cells[r][c] = 0
			}
		}
	case "s", "enter":
		m.submitted = true
		return m, tea.Quit
	}
	return m, nil
}

// GitHub-ish palette: dark background through bright green.
var palette = [5]lipgloss.Color{
	"#161b22",
	"#0e4429",
	"#006d32",
	"#26a641",
	"#39d353",
}

func cellStyle(level int) lipgloss.Style {
	return lipgloss.NewStyle().Background(palette[level]).Foreground(lipgloss.Color("#ffffff"))
}

var (
	helpStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("#7d8590"))
	headerStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#7d8590"))
	statusStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#c9d1d9")).Bold(true)
)

func (m Model) View() string {
	if m.quitting && !m.submitted {
		return "aborted\n"
	}

	var b strings.Builder
	b.WriteString(helpStyle.Render("arrows/hjkl move  ·  0-4 set level  ·  +/- adjust  ·  space cycle  ·  x clear cell  ·  X clear all  ·  s submit  ·  q quit"))
	b.WriteString("\n")
	if m.loaded > 0 {
		b.WriteString(helpStyle.Render(fmt.Sprintf("loaded %d existing commits — submit will rewrite tool history to match", m.loaded)))
	} else {
		b.WriteString(helpStyle.Render("no existing tool history found — submit will create commits"))
	}
	b.WriteString("\n\n")

	b.WriteString(renderMonthHeader(m.grid))
	b.WriteString("\n")

	dayLabels := [grid.Rows]string{"Sun", "Mon", "Tue", "Wed", "Thu", "Fri", "Sat"}
	for row := 0; row < grid.Rows; row++ {
		// Show every other day label to reduce vertical noise.
		label := "   "
		if row%2 == 1 {
			label = dayLabels[row]
		}
		b.WriteString(headerStyle.Render(label) + " ")
		for col := 0; col < grid.Cols; col++ {
			cell := m.grid.Cells[row][col]
			style := cellStyle(cell)
			content := "  "
			if row == m.cursorRow && col == m.cursorCol {
				content = "··"
			}
			b.WriteString(style.Render(content))
		}
		b.WriteString("\n")
	}

	b.WriteString("\n")
	date := m.grid.DateAt(m.cursorRow, m.cursorCol)
	level := m.grid.Cells[m.cursorRow][m.cursorCol]
	b.WriteString(statusStyle.Render(fmt.Sprintf(
		"%s  ·  level %d (%d commits)  ·  total %d commits",
		date.Format("Mon 2006-01-02"),
		level,
		grid.IntensityCommits[level],
		m.grid.TotalCommits(),
	)))
	b.WriteString("\n")

	return b.String()
}

// renderMonthHeader builds a row aligned with the grid columns that shows
// each month's abbreviation at the column where that month begins.
func renderMonthHeader(g *grid.Grid) string {
	const cellW = 2
	const leadIn = 4 // matches the 3-char day label + 1 space
	width := leadIn + grid.Cols*cellW
	buf := make([]rune, width)
	for i := range buf {
		buf[i] = ' '
	}
	prevMonth := -1
	for col := 0; col < grid.Cols; col++ {
		d := g.DateAt(0, col)
		m := int(d.Month())
		if m == prevMonth {
			continue
		}
		prevMonth = m
		label := d.Format("Jan")
		start := leadIn + col*cellW
		for i, r := range label {
			if start+i < width {
				buf[start+i] = r
			}
		}
	}
	return headerStyle.Render(string(buf))
}
