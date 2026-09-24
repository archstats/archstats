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

	e := &extension{}
	log, err := e.parseGitLog(filepath.Clean("./temp_testdata/"))
	if err != nil {
		t.Error(err)
	}

	// 46 commits, two of them merges that change nothing of their own: a
	// merge counts only for what was written while resolving it.
	assert.Len(t, log, 44)
}

func TestParseSurvivesSeparatorsInSubjectsAndSpacesInPaths(t *testing.T) {
	out := "\x1e[-archstatscommit-]abc123\x1f1700000000\x1fJeff Fischer\x1fjf@x.org\x1ffix -- again\x1d\n" +
		"3\t1\tsrc/main/Order Service.java\n" +
		"-\t-\timg/Sorting icons.psd\n" +
		"\x1e[-archstatscommit-]def456\x1f1700000100\x1fAnn\x1fann@x.org\x1fplain\x1d\n" +
		"1\t0\tREADME.md\n"
	commits := parseGitLogString("repo", out)
	assert.Len(t, commits, 2)
	assert.Equal(t, "fix -- again", commits[0].Message)
	assert.Equal(t, "Jeff Fischer", commits[0].AuthorName)
	assert.Len(t, commits[0].Files, 2)
	assert.Equal(t, "src/main/Order Service.java", commits[0].Files[0].Path)
	assert.Equal(t, 3, commits[0].Files[0].Additions)
	assert.Equal(t, "img/Sorting icons.psd", commits[0].Files[1].Path)
	assert.Equal(t, "README.md", commits[1].Files[0].Path)
}

// A Latin-1 name from a commit without an encoding header is read as the
// name it is, not as bytes no reader can decode.
func TestLatin1AuthorNamesAreDecoded(t *testing.T) {
	out := "\x1e[-archstatscommit-]abc\x1f1700000000\x1fJavier Gonz\xe1lez\x1fjavi@x.es\x1fcaf\xe9\x1d\n1\t0\ta.php\n"
	commits := parseGitLogString("repo", out)
	assert.Equal(t, "Javier González", commits[0].AuthorName)
	assert.Equal(t, "café", commits[0].Message)
	// Already UTF-8 is left alone.
	assert.Equal(t, "Paweł", asUTF8("Paweł"))
}
