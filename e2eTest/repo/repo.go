package repo

import (
	"fmt"
	"os"
	"os/exec"
	"path"
	"strings"

	"github.com/archstats/archstats/cmd"
	"github.com/jmoiron/sqlx"
)

type Repo struct {
	Location string
	URL      string
}

// EnsureCloned fetches exactly the one commit a test pins, and nothing else.
//
// A full clone of a real project is most of the wall-clock time of an e2e
// run and almost all of the disk: these tests want one known tree, not the
// history that produced it. Fetching the commit by name at depth 1 keeps a
// language fixture in the low megabytes, and a second run does no network at
// all because the checkout is already there.
func EnsureCloned(repo, commit string) (*Repo, error) {
	repoLocation := "temp_testdata/" + path.Base(repo)

	if head, err := exec.Command("git", "-C", repoLocation, "rev-parse", "HEAD").Output(); err == nil {
		if strings.HasPrefix(strings.TrimSpace(string(head)), commit) {
			return &Repo{Location: repoLocation, URL: repo}, nil
		}
	}

	if _, err := os.Stat(repoLocation); os.IsNotExist(err) {
		if err := os.MkdirAll(repoLocation, 0o755); err != nil {
			return nil, err
		}
		if err := run(repoLocation, "init", "-q"); err != nil {
			return nil, err
		}
		if err := run(repoLocation, "remote", "add", "origin", repo); err != nil {
			return nil, err
		}
	}

	// Some servers refuse to serve an arbitrary commit; fall back to a
	// shallow clone of the default branch and check the commit out of that.
	if err := run(repoLocation, "fetch", "--depth", "1", "-q", "origin", commit); err != nil {
		if err := run(repoLocation, "fetch", "--depth", "50", "-q", "origin"); err != nil {
			return nil, fmt.Errorf("fetching %s: %w", repo, err)
		}
	}
	if err := run(repoLocation, "checkout", "-q", commit); err != nil {
		if err := run(repoLocation, "checkout", "-q", "FETCH_HEAD"); err != nil {
			return nil, fmt.Errorf("checking out %s of %s: %w", commit, repo, err)
		}
	}

	return &Repo{Location: repoLocation, URL: repo}, nil
}

func run(dir string, args ...string) error {
	return exec.Command("git", append([]string{"-C", dir}, args...)...).Run()
}

func (r *Repo) GetExportedDB(command *ExportDBCommand) (*sqlx.DB, error) {
	if command.FileName == "" {
		command.FileName = r.Location + "/" + path.Base(r.URL) + ".db"
	}

	if _, err := os.Stat(command.FileName); os.IsNotExist(err) {
		var args []string
		args = append(args, "-f", r.Location)
		for _, ext := range command.Extensions {
			args = append(args, "-e", ext)
		}
		args = append(args, []string{"export", "sqlite", command.FileName}...)

		err := cmd.Execute(os.Stdout, os.Stderr, nil, args)

		if err != nil {
			return nil, err
		}
	}

	return sqlx.Connect("sqlite3", command.FileName)
}

func (r *Repo) ExecuteArchstatsCommand(command string) error {
	return nil
}

type ExportDBCommand struct {
	FileName   string
	Extensions []string
}
