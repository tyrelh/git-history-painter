// Package grid models the 7x52 contribution grid and the date math
// that maps cell coordinates back to calendar days.
package grid

import "time"

const (
	Rows = 7
	Cols = 52
)

// IntensityCommits maps a cell's intensity level (0-4) to the number
// of commits to make for that day. Level 0 means no commits.
// The non-zero counts loosely mirror GitHub's historical bucket cutoffs.
var IntensityCommits = [5]int{0, 1, 3, 6, 10}

// Grid is a 7-row by 52-column board of intensity levels.
// Row 0 is Sunday, row 6 is Saturday. Column 0 is the leftmost (oldest) week.
type Grid struct {
	Cells [Rows][Cols]int
	// Start is the Sunday represented by Cells[0][0] (UTC, midnight).
	Start time.Time
}

// New returns a Grid whose rightmost column ends on the most recent
// Saturday on or before `today`. Cells[0][0] is the Sunday 363 days
// before that Saturday — i.e. exactly 52 weeks of history ending the
// day before the next Sunday.
func New(today time.Time) *Grid {
	today = time.Date(today.Year(), today.Month(), today.Day(), 0, 0, 0, 0, time.UTC)
	// Days to step back to reach the most recent Saturday (inclusive of today).
	// Weekday: Sun=0, Mon=1, ..., Sat=6. Distance Sat -> day:
	//   Sat(6)=0, Sun(0)=1, Mon(1)=2, Tue(2)=3, Wed(3)=4, Thu(4)=5, Fri(5)=6.
	daysSinceSat := (int(today.Weekday()) + 1) % 7
	endSat := today.AddDate(0, 0, -daysSinceSat)
	start := endSat.AddDate(0, 0, -(Rows*Cols - 1))
	return &Grid{Start: start}
}

// DateAt returns the calendar date represented by cell (row, col).
func (g *Grid) DateAt(row, col int) time.Time {
	return g.Start.AddDate(0, 0, col*Rows+row)
}

// CellAt returns the (row, col) for the given calendar date, or ok=false
// if the date falls outside the grid's 52-week window. The date is
// truncated to UTC midnight before mapping.
func (g *Grid) CellAt(date time.Time) (row, col int, ok bool) {
	d := time.Date(date.Year(), date.Month(), date.Day(), 0, 0, 0, 0, time.UTC)
	days := int(d.Sub(g.Start).Hours() / 24)
	if days < 0 || days >= Rows*Cols {
		return 0, 0, false
	}
	return days % Rows, days / Rows, true
}

// LevelForCount maps a per-day commit count back to the highest intensity
// level whose canonical commit count does not exceed n. Counts above the
// top bucket clamp to level 4.
func LevelForCount(n int) int {
	if n <= 0 {
		return 0
	}
	level := 0
	for i := 1; i < len(IntensityCommits); i++ {
		if IntensityCommits[i] <= n {
			level = i
		}
	}
	return level
}

// Set assigns intensity to a cell, clamping to [0, 4].
func (g *Grid) Set(row, col, intensity int) {
	if intensity < 0 {
		intensity = 0
	}
	if intensity > 4 {
		intensity = 4
	}
	g.Cells[row][col] = intensity
}

// Cycle advances the intensity at (row, col) by one, wrapping 4 -> 0.
func (g *Grid) Cycle(row, col int) {
	g.Cells[row][col] = (g.Cells[row][col] + 1) % 5
}

// TotalCommits returns how many commits would be produced if the grid
// were submitted as-is.
func (g *Grid) TotalCommits() int {
	n := 0
	for row := 0; row < Rows; row++ {
		for col := 0; col < Cols; col++ {
			n += IntensityCommits[g.Cells[row][col]]
		}
	}
	return n
}
