// Package commit translates a painted grid into backdated empty commits,
// and syncs an existing tool-managed history back into a grid model.
package commit

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"time"

	"git-history/internal/grid"
)

// ToolPrefix marks commits owned by this tool. Anything else in the
// repository is treated as the user's real work and left untouched.
const ToolPrefix = "history: "

// Generate writes one empty commit for each unit of intensity in the grid.
// Commits within a single day are spread one minute apart starting at
// 09:00 UTC so each gets a unique author/committer date and a unique
// hash without spilling into the next day.
//
// `progress`, if non-nil, receives a human-readable line for every commit.
func Generate(repoDir string, g *grid.Grid, progress io.Writer) error {
	total := g.TotalCommits()
	if total == 0 {
		return fmt.Errorf("nothing to commit: grid is empty")
	}

	done := 0
	for col := 0; col < grid.Cols; col++ {
		for row := 0; row < grid.Rows; row++ {
			n := grid.IntensityCommits[g.Cells[row][col]]
			if n == 0 {
				continue
			}
			day := g.DateAt(row, col)
			for i := 0; i < n; i++ {
				ts := time.Date(day.Year(), day.Month(), day.Day(),
					9, i, 0, 0, time.UTC)
				if err := commitAt(repoDir, ts); err != nil {
					return fmt.Errorf("commit for %s: %w", ts.Format(time.RFC3339), err)
				}
				done++
				if progress != nil {
					fmt.Fprintf(progress, "[%d/%d] %s\n", done, total, ts.Format("2006-01-02 15:04"))
				}
			}
		}
	}
	return nil
}

// LoadGrid scans repoDir for commits owned by this tool and fills in g's
// intensities to reflect the per-day commit count. Returns the number of
// tool commits that were found (including any whose dates fall outside
// the grid's 52-week window — those are counted but not painted).
func LoadGrid(repoDir string, g *grid.Grid) (int, error) {
	if !hasHEAD(repoDir) {
		return 0, nil
	}
	cmd := exec.Command("git", "log", "--format=%aI%x09%s")
	cmd.Dir = repoDir
	out, err := cmd.Output()
	if err != nil {
		return 0, fmt.Errorf("git log: %w", asExitErr(err))
	}

	counts := map[[2]int]int{}
	total := 0
	sc := bufio.NewScanner(bytes.NewReader(out))
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for sc.Scan() {
		line := sc.Text()
		tab := strings.IndexByte(line, '\t')
		if tab < 0 {
			continue
		}
		subject := line[tab+1:]
		if !strings.HasPrefix(subject, ToolPrefix) {
			continue
		}
		t, err := time.Parse(time.RFC3339, line[:tab])
		if err != nil {
			continue
		}
		total++
		if r, c, ok := g.CellAt(t.UTC()); ok {
			counts[[2]int{r, c}]++
		}
	}
	if err := sc.Err(); err != nil {
		return 0, err
	}
	for cell, n := range counts {
		g.Cells[cell[0]][cell[1]] = grid.LevelForCount(n)
	}
	return total, nil
}

// Sync rewrites the tool-managed portion of repoDir's history so it
// matches g. All tool commits are dropped and re-created from the
// painted grid; non-tool commits are preserved.
//
// If tool commits are interleaved with non-tool commits (i.e. a real
// commit lands after a tool commit in topo order), Sync refuses to
// rewrite so the user's real work isn't lost.
func Sync(repoDir string, g *grid.Grid, progress io.Writer) error {
	branch, err := currentBranch(repoDir)
	if err != nil {
		return err
	}
	base, interleaved, err := findRewriteBase(repoDir)
	if err != nil {
		return err
	}
	if interleaved {
		return fmt.Errorf("non-tool commits appear after tool commits in history; refusing to rewrite. Resolve manually, or paint into a separate repo")
	}

	if base != "" {
		if err := run(repoDir, "git", "reset", "--hard", base); err != nil {
			return err
		}
	} else if hasHEAD(repoDir) {
		// Orphan the branch: point HEAD at it, then delete the ref.
		// `git commit` will then start a new root commit on the branch.
		if err := run(repoDir, "git", "symbolic-ref", "HEAD", "refs/heads/"+branch); err != nil {
			return err
		}
		if err := run(repoDir, "git", "update-ref", "-d", "refs/heads/"+branch); err != nil {
			return err
		}
		if err := run(repoDir, "git", "read-tree", "--empty"); err != nil {
			return err
		}
	}

	if g.TotalCommits() == 0 {
		return nil
	}
	return Generate(repoDir, g, progress)
}

// findRewriteBase walks the current branch oldest-first and returns the
// hash of the last non-tool commit (or "" if there are none). If a
// non-tool commit appears after a tool commit, interleaved=true so the
// caller can bail before rewriting anything.
func findRewriteBase(repoDir string) (base string, interleaved bool, err error) {
	if !hasHEAD(repoDir) {
		return "", false, nil
	}
	cmd := exec.Command("git", "log", "--reverse", "--topo-order", "--format=%H %s")
	cmd.Dir = repoDir
	out, err := cmd.Output()
	if err != nil {
		return "", false, fmt.Errorf("git log: %w", asExitErr(err))
	}
	seenTool := false
	sc := bufio.NewScanner(bytes.NewReader(out))
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for sc.Scan() {
		line := sc.Text()
		sp := strings.IndexByte(line, ' ')
		if sp < 0 {
			continue
		}
		hash, subject := line[:sp], line[sp+1:]
		if strings.HasPrefix(subject, ToolPrefix) {
			seenTool = true
			continue
		}
		if seenTool {
			return "", true, nil
		}
		base = hash
	}
	return base, false, sc.Err()
}

func commitAt(dir string, ts time.Time) error {
	msg := fmt.Sprintf("%s%s", ToolPrefix, ts.Format(time.RFC3339))
	cmd := exec.Command("git", "commit", "--allow-empty", "-m", msg)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(),
		"GIT_AUTHOR_DATE="+ts.Format(time.RFC3339),
		"GIT_COMMITTER_DATE="+ts.Format(time.RFC3339),
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("git: %v\n%s", err, out)
	}
	return nil
}

func hasHEAD(repoDir string) bool {
	cmd := exec.Command("git", "rev-parse", "--verify", "--quiet", "HEAD")
	cmd.Dir = repoDir
	return cmd.Run() == nil
}

func currentBranch(repoDir string) (string, error) {
	cmd := exec.Command("git", "symbolic-ref", "--short", "HEAD")
	cmd.Dir = repoDir
	if out, err := cmd.Output(); err == nil {
		return strings.TrimSpace(string(out)), nil
	}
	cmd = exec.Command("git", "config", "init.defaultBranch")
	cmd.Dir = repoDir
	if out, err := cmd.Output(); err == nil {
		if b := strings.TrimSpace(string(out)); b != "" {
			return b, nil
		}
	}
	return "main", nil
}

func run(dir string, name string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s %s: %v\n%s", name, strings.Join(args, " "), err, out)
	}
	return nil
}

// asExitErr surfaces the stderr of a failed exec when available, so the
// caller sees git's own error message rather than a bare "exit status 128".
func asExitErr(err error) error {
	if ee, ok := err.(*exec.ExitError); ok && len(ee.Stderr) > 0 {
		return fmt.Errorf("%v: %s", err, bytes.TrimSpace(ee.Stderr))
	}
	return err
}
