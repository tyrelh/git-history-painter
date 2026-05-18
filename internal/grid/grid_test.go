package grid

import (
	"testing"
	"time"
)

func TestNewStartsOnSunday(t *testing.T) {
	// Cover every day of the week — Start must always land on Sunday.
	base := time.Date(2026, 5, 11, 12, 0, 0, 0, time.UTC) // a Monday
	for i := 0; i < 7; i++ {
		today := base.AddDate(0, 0, i)
		g := New(today)
		if g.Start.Weekday() != time.Sunday {
			t.Errorf("New(%s): Start = %s (%s), want Sunday",
				today.Format("Mon 2006-01-02"),
				g.Start.Format("Mon 2006-01-02"),
				g.Start.Weekday())
		}
	}
}

func TestGridSpans364Days(t *testing.T) {
	g := New(time.Date(2026, 5, 17, 0, 0, 0, 0, time.UTC))
	last := g.DateAt(Rows-1, Cols-1)
	first := g.DateAt(0, 0)
	got := int(last.Sub(first).Hours() / 24)
	if got != Rows*Cols-1 {
		t.Errorf("span = %d days, want %d", got, Rows*Cols-1)
	}
}

func TestEndsOnMostRecentSaturday(t *testing.T) {
	cases := []struct {
		today time.Time
		want  time.Time
	}{
		// Today is Sunday 2026-05-17 → last Saturday is 2026-05-16.
		{
			time.Date(2026, 5, 17, 0, 0, 0, 0, time.UTC),
			time.Date(2026, 5, 16, 0, 0, 0, 0, time.UTC),
		},
		// Today is Saturday → ends today.
		{
			time.Date(2026, 5, 16, 0, 0, 0, 0, time.UTC),
			time.Date(2026, 5, 16, 0, 0, 0, 0, time.UTC),
		},
		// Today is Wednesday 2026-05-13 → last Saturday is 2026-05-09.
		{
			time.Date(2026, 5, 13, 0, 0, 0, 0, time.UTC),
			time.Date(2026, 5, 9, 0, 0, 0, 0, time.UTC),
		},
	}
	for _, c := range cases {
		g := New(c.today)
		end := g.DateAt(Rows-1, Cols-1)
		if !end.Equal(c.want) {
			t.Errorf("today=%s: end=%s, want %s",
				c.today.Format("2006-01-02"),
				end.Format("Mon 2006-01-02"),
				c.want.Format("Mon 2006-01-02"))
		}
	}
}

func TestDateAtMonotonic(t *testing.T) {
	g := New(time.Date(2026, 5, 17, 0, 0, 0, 0, time.UTC))
	prev := g.DateAt(0, 0).Add(-24 * time.Hour)
	for col := 0; col < Cols; col++ {
		for row := 0; row < Rows; row++ {
			d := g.DateAt(row, col)
			if !d.After(prev) {
				t.Fatalf("non-monotonic at (%d,%d): %s after %s", row, col,
					d.Format("2006-01-02"), prev.Format("2006-01-02"))
			}
			prev = d
		}
	}
}

func TestCellAtRoundTrip(t *testing.T) {
	g := New(time.Date(2026, 5, 17, 0, 0, 0, 0, time.UTC))
	for row := 0; row < Rows; row++ {
		for col := 0; col < Cols; col++ {
			d := g.DateAt(row, col)
			r, c, ok := g.CellAt(d)
			if !ok || r != row || c != col {
				t.Fatalf("CellAt(DateAt(%d,%d)) = (%d,%d,%v)", row, col, r, c, ok)
			}
		}
	}
}

func TestCellAtOutOfRange(t *testing.T) {
	g := New(time.Date(2026, 5, 17, 0, 0, 0, 0, time.UTC))
	cases := []time.Time{
		g.Start.AddDate(0, 0, -1),
		g.DateAt(Rows-1, Cols-1).AddDate(0, 0, 1),
	}
	for _, d := range cases {
		if _, _, ok := g.CellAt(d); ok {
			t.Errorf("CellAt(%s) should be out of range", d.Format("2006-01-02"))
		}
	}
}

func TestLevelForCount(t *testing.T) {
	cases := []struct{ n, want int }{
		{-1, 0}, {0, 0}, {1, 1}, {2, 1}, {3, 2}, {5, 2},
		{6, 3}, {9, 3}, {10, 4}, {100, 4},
	}
	for _, c := range cases {
		if got := LevelForCount(c.n); got != c.want {
			t.Errorf("LevelForCount(%d) = %d, want %d", c.n, got, c.want)
		}
	}
}

func TestTotalCommits(t *testing.T) {
	g := New(time.Date(2026, 5, 17, 0, 0, 0, 0, time.UTC))
	g.Set(0, 0, 1)
	g.Set(3, 10, 4)
	g.Set(6, 51, 2)
	want := IntensityCommits[1] + IntensityCommits[4] + IntensityCommits[2]
	if got := g.TotalCommits(); got != want {
		t.Errorf("TotalCommits = %d, want %d", got, want)
	}
}
