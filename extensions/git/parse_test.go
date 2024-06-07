package git

import (
	"github.com/stretchr/testify/assert"
	"os"
	"os/exec"
	filepath "path"
	"testing"
)

func TestBasicGitParsing(t *testing.T) {
	//remove temp_testdata
	defer os.RemoveAll(filepath.Clean("./temp_testdata/"))

	err := exec.Command("git", "clone", "https://github.com/RyanSusana/AsyncFX.git", filepath.Clean("./temp_testdata/")).Run()
	if err != nil {
		t.Error(err)
	}

	// reset to commit
	err = exec.Command("git", "-C", filepath.Clean("./temp_testdata/"), "reset", "--hard", "6e59365").Run()
	if err != nil {
		t.Error(err)
	}

	log, err := parseGitLog(filepath.Clean("./temp_testdata/"))
	if err != nil {
		t.Error(err)
	}

	assert.Len(t, log, 46)
}
