package repo

import (
	"github.com/archstats/archstats/cmd"
	"github.com/jmoiron/sqlx"
	"os"
	"os/exec"
	"path"
)

type Repo struct {
	Location string
	URL      string
}

// EnsureCloned a repo and checks out a specific commit
// if the repo already exists, only checkout the commit
func EnsureCloned(repo, commit string) (*Repo, error) {
	dir := path.Base(repo)
	repoLocation := "temp_testdata/" + dir
	var err error
	//check if repo exists in location
	if _, err = os.Stat(repoLocation); os.IsNotExist(err) {
		err = exec.Command("git", "clone", repo, repoLocation).Run()
	} else {
		//err = exec.Command("git", "-C", repoLocation, "pull", "-f").Run()
	}
	if err != nil {
		return nil, err
	}
	err = exec.Command("git", "checkout", "-C", repoLocation, commit).Run()
	return &Repo{
		Location: repoLocation,
		URL:      repo,
	}, nil
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
