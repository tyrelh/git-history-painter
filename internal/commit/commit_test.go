package commit

import (
	"os/exec"
	"strings"
	"testing"
	"time"

	"git-history/internal/grid"
)

func TestSyncRoundTrip(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}
	dir := initRepo(t)

	today := time.Date(2026, 5, 17, 0, 0, 0, 0, time.UTC)

	// Paint v1: levels 1, 2, 3 on three different cells.
	g1 := grid.New(today)
	g1.Set(0, 0, 1)
	g1.Set(3, 10, 2)
	g1.Set(6, 51, 3)
	if err := Sync(dir, g1, nil); err != nil {
		t.Fatalf("Sync v1: %v", err)
	}

	// Load into a fresh grid and verify it round-trips.
	g2 := grid.New(today)
	n, err := LoadGrid(dir, g2)
	if err != nil {
		t.Fatalf("LoadGrid: %v", err)
	}
	if n != g1.TotalCommits() {
		t.Errorf("LoadGrid returned %d, want %d", n, g1.TotalCommits())
	}
	if g2.Cells != g1.Cells {
		t.Errorf("loaded grid mismatch:\n got %v\nwant %v", g2.Cells, g1.Cells)
	}

	// Paint v2: erase one cell, decrease one, add a new one.
	g2.Set(0, 0, 0)  // erase
	g2.Set(3, 10, 1) // decrease 2 -> 1
	g2.Set(2, 5, 4)  // add
	if err := Sync(dir, g2, nil); err != nil {
		t.Fatalf("Sync v2: %v", err)
	}

	// Reload and confirm v2 state is what we get back.
	g3 := grid.New(today)
	if _, err := LoadGrid(dir, g3); err != nil {
		t.Fatalf("LoadGrid after v2: %v", err)
	}
	if g3.Cells != g2.Cells {
		t.Errorf("v2 round-trip mismatch:\n got %v\nwant %v", g3.Cells, g2.Cells)
	}

	// And the raw commit count matches the painted grid.
	got := countCommits(t, dir)
	if got != g2.TotalCommits() {
		t.Errorf("repo has %d commits, want %d", got, g2.TotalCommits())
	}
}

func TestSyncPreservesNonToolCommits(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}
	dir := initRepo(t)

	// Real user commit at the base.
	mustRun(t, dir, "git", "commit", "--allow-empty", "-m", "real work")
	baseHash := revParse(t, dir, "HEAD")

	g := grid.New(time.Date(2026, 5, 17, 0, 0, 0, 0, time.UTC))
	g.Set(0, 0, 2)
	if err := Sync(dir, g, nil); err != nil {
		t.Fatalf("Sync: %v", err)
	}

	// The base commit should still exist as the root.
	rootHash := revParse(t, dir, "HEAD~"+itoa(g.TotalCommits()))
	if rootHash != baseHash {
		t.Errorf("root after sync = %s, want base %s", rootHash, baseHash)
	}

	// Erasing should drop the tool commits but keep the base.
	g.Set(0, 0, 0)
	if err := Sync(dir, g, nil); err != nil {
		t.Fatalf("Sync erase: %v", err)
	}
	if got := revParse(t, dir, "HEAD"); got != baseHash {
		t.Errorf("HEAD after erase = %s, want base %s", got, baseHash)
	}
}

func TestSyncRefusesInterleavedHistory(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}
	dir := initRepo(t)

	// Tool commit first, then a real commit on top — interleaved.
	g := grid.New(time.Date(2026, 5, 17, 0, 0, 0, 0, time.UTC))
	g.Set(0, 0, 1)
	if err := Sync(dir, g, nil); err != nil {
		t.Fatalf("Sync seed: %v", err)
	}
	mustRun(t, dir, "git", "commit", "--allow-empty", "-m", "real work on top")

	g.Set(0, 0, 2)
	err := Sync(dir, g, nil)
	if err == nil || !strings.Contains(err.Error(), "interleaved") && !strings.Contains(err.Error(), "non-tool") {
		t.Errorf("Sync should refuse interleaved history; got %v", err)
	}
}

// --- helpers ---

func initRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	mustRun(t, dir, "git", "init", "-q", "-b", "main")
	mustRun(t, dir, "git", "config", "user.email", "test@example.com")
	mustRun(t, dir, "git", "config", "user.name", "Tester")
	mustRun(t, dir, "git", "config", "commit.gpgsign", "false")
	return dir
}

func mustRun(t *testing.T, dir string, name string, args ...string) {
	t.Helper()
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("%s %s: %v\n%s", name, strings.Join(args, " "), err, out)
	}
}

func revParse(t *testing.T, dir, ref string) string {
	t.Helper()
	cmd := exec.Command("git", "rev-parse", ref)
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("rev-parse %s: %v", ref, err)
	}
	return strings.TrimSpace(string(out))
}

func countCommits(t *testing.T, dir string) int {
	t.Helper()
	cmd := exec.Command("git", "rev-list", "--count", "HEAD")
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("rev-list: %v", err)
	}
	n := 0
	for _, c := range strings.TrimSpace(string(out)) {
		n = n*10 + int(c-'0')
	}
	return n
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	return string(buf[i:])
}
