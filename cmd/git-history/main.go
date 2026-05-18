// git-history is a TUI that paints a 7x52 grid and writes backdated
// empty commits to the current git repo so the design renders on
// the GitHub contribution graph.
package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"git-history/internal/commit"
	"git-history/internal/grid"
	"git-history/internal/tui"

	tea "github.com/charmbracelet/bubbletea"
)

func main() {
	var dir string
	var dryRun bool
	var noLoad bool
	flag.StringVar(&dir, "dir", ".", "git repository directory to commit into")
	flag.BoolVar(&dryRun, "dry-run", false, "paint the grid but do not write any commits")
	flag.BoolVar(&noLoad, "no-load", false, "start with an empty grid even if the repo already has painted commits")
	flag.Parse()

	abs, err := filepath.Abs(dir)
	if err != nil {
		exitf("resolve dir: %v", err)
	}
	if info, err := os.Stat(filepath.Join(abs, ".git")); err != nil || !info.IsDir() {
		exitf("%s is not a git repository — run `git init` first (or pass --dir)", abs)
	}

	g := grid.New(time.Now())

	loaded := 0
	if !noLoad {
		n, err := commit.LoadGrid(abs, g)
		if err != nil {
			exitf("load existing history: %v", err)
		}
		loaded = n
	}

	final, err := tea.NewProgram(tui.New(g, loaded), tea.WithAltScreen()).Run()
	if err != nil {
		exitf("tui: %v", err)
	}

	fm := final.(tui.Model)
	if !fm.Submitted() {
		fmt.Println("aborted — no commits written")
		return
	}

	total := fm.Grid().TotalCommits()
	if dryRun {
		fmt.Printf("dry run — would sync to %d commits in %s\n", total, abs)
		return
	}

	fmt.Printf("syncing %d commits into %s\n", total, abs)
	if err := commit.Sync(abs, fm.Grid(), os.Stdout); err != nil {
		exitf("%v", err)
	}
	fmt.Println("done")
}

func exitf(format string, args ...any) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
	os.Exit(1)
}
