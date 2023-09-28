package e2eTest

import (
	"bytes"
	"fmt"
	"github.com/RyanSusana/archstats/cmd"
	"github.com/RyanSusana/archstats/core"
	"github.com/RyanSusana/archstats/extensions/regex"
	"github.com/jszwec/csvutil"
	"github.com/stretchr/testify/assert"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

const TestDataPath = "./temp_testdata"

// TODO: revisit this test
//func Test_E2E_Extensions(t *testing.T) {
//
//	os.Mkdir(TestDataPath, 0755)
//	//defer os.RemoveAll(TestDataPath) TODO: Should not remove all in the case of mutation testing
//
//	file, _ := os.ReadFile("e2e_input.yaml")
//
//	config := &Config{}
//	yaml.Unmarshal(file, config)
//
//	for _, theCase := range config.Cases {
//		testCase(t, theCase)
//	}
//}

func Test_SimpleComponents_AfferentEfferentCoupling(t *testing.T) {
	simpleComponentsTest(t, "components", "name,file_count,afferent_couplings,efferent_couplings", []Component{
		component("a", 2, 1, 2),
		component("b", 1, 2, 1),
		component("c", 1, 2, 1),
		component("d", 1, 1, 0),
	})
}

func Test_SimpleComponents_DirectConnections(t *testing.T) {
	simpleComponentsTest(t, "component_connections_direct", "from,to,file,reference_count", []ComponentConnectionDirect{
		directConnection("a", "b", "a/a_1", 1),
		directConnection("a", "c", "a/a_1", 1),
		directConnection("a", "b", "a/a_2", 1),
		directConnection("a", "d", "a/a_2", 1),
		directConnection("b", "c", "b/b_1", 2),
		directConnection("c", "a", "c/c_1", 1),
	})
}
func Test_SimpleComponents_IndirectConnections(t *testing.T) {
	simpleComponentsTest(t, "component_connections_indirect", "from,to,shortest_path_length,shortest_path", []ComponentConnectionIndirect{
		indirectConnection("a", "d", 2, "a -> d"),
		indirectConnection("a", "c", 2, "a -> c"),
		indirectConnection("a", "b", 2, "a -> b"),
		indirectConnection("b", "d", 4, "b -> c -> a -> d"),
		indirectConnection("b", "c", 2, "b -> c"),
		indirectConnection("b", "a", 3, "b -> c -> a"),
		indirectConnection("c", "b", 3, "c -> a -> b"),
		indirectConnection("c", "d", 3, "c -> a -> d"),
		indirectConnection("c", "a", 2, "c -> a"),
	})
}

func simpleComponentsTest[T any](t *testing.T, view string, columns string, expectedOutput []T) {
	ext := &regex.Extension{
		OnlyStats: false,
		Glob:      nil,
		Patterns: []*regexp.Regexp{
			regexp.MustCompile(`component (?P<component_declaration>.*)`),
			regexp.MustCompile(`depends on (?P<component_import>.*)`),
		},
	}

	output := bytes.NewBufferString("")

	//cmd.Reset()
	cmd.Execute(output, bytes.NewBufferString(""), []core.Extension{
		ext,
	}, []string{"-c", columns, "-o", "csv", "-f", "simple_components", "view", view}) //"-c", "name,afferent_couplings,efferent_couplings",

	var actualOutput []T
	err := csvutil.Unmarshal(output.Bytes(), &actualOutput)

	if err != nil {
		assert.Fail(t, "Failed to unmarshal output: %s", err)
	}
	assert.ElementsMatch(t, expectedOutput, actualOutput)

}
func component(name string, fileCount, afferentCouplings, efferentCouplings int) Component {
	return Component{
		Name:              name,
		FileCount:         fileCount,
		AfferentCouplings: afferentCouplings,
		EfferentCouplings: efferentCouplings,
	}
}

type Component struct {
	Name              string `csv:"NAME"`
	FileCount         int    `csv:"FILE_COUNT"`
	AfferentCouplings int    `csv:"AFFERENT_COUPLINGS,omitempty"`
	EfferentCouplings int    `csv:"EFFERENT_COUPLINGS,omitempty"`
}

func directConnection(from, to, file string, referenceCount int) ComponentConnectionDirect {
	return ComponentConnectionDirect{
		From:           from,
		To:             to,
		File:           file,
		ReferenceCount: referenceCount,
	}
}

type ComponentConnectionDirect struct {
	From           string `csv:"FROM"`
	To             string `csv:"TO"`
	File           string `csv:"FILE"`
	ReferenceCount int    `csv:"REFERENCE_COUNT"`
}

func indirectConnection(from, to string, shortestPathLength int, shortestPath string) ComponentConnectionIndirect {
	return ComponentConnectionIndirect{
		From:               from,
		To:                 to,
		ShortestPathLength: shortestPathLength,
		ShortestPath:       shortestPath,
	}
}

type ComponentConnectionIndirect struct {
	From               string `csv:"FROM"`
	To                 string `csv:"TO"`
	ShortestPathLength int    `csv:"SHORTEST_PATH_LENGTH"`
	ShortestPath       string `csv:"SHORTEST_PATH"`
}

func testCase(t *testing.T, theCase Case) {
	log.Println("Testing case:", theCase.Name)

	repoDirName := filepath.Clean(strings.ReplaceAll(fmt.Sprintf("%s/%s", TestDataPath, theCase.Repo[strings.LastIndex(theCase.Repo, "/"):]), ".git", ""))
	if _, err := os.Stat(repoDirName); os.IsNotExist(err) {
		log.Println("Cloning repo:", theCase.Repo)
		err := exec.Command("git", "clone", theCase.Repo, repoDirName).Run()
		if err != nil {
			assert.Fail(t, "Failed to clone repo: {}", theCase.Repo)
		}
	} else {
		log.Println("Repo already exists, skipping clone...")
	}

	err := exec.Command("git", "-C", repoDirName, "reset", "--hard", theCase.Commit).Run()
	if err != nil {
		assert.Fail(t, "Failed to reset repo (%s) to commit '%s'", theCase.Repo, theCase.Commit)
	}

	err = os.WriteFile(fmt.Sprintf("%s/%s", repoDirName, ".archstatsignore"), []byte(theCase.Ignore), 0644)
	defer os.Remove(fmt.Sprintf("%s/%s", repoDirName, ".archstatsignore"))
	if err != nil {
		assert.Fail(t, "Failed to write .archstatsignore file")
	}
	allArgs := append([]string{"-f", repoDirName}, strings.Fields(theCase.OptionArgs)...)

	output := bytes.NewBufferString("")
	err = cmd.Execute(output, bytes.NewBufferString(""), nil, allArgs)

	if err != nil {
		assert.Fail(t, "Failed to run archstats: %s", err)
	}

	expectedOutputBytes, err := os.ReadFile(theCase.ExpectedOutputFile)
	if err != nil {
		assert.Fail(t, "Failed to read expected output file: %s", theCase.ExpectedOutputFile)
	}

	expectedOutput := strings.TrimSpace(string(expectedOutputBytes))
	actualOutput := output.String()
	var passed bool
	passed = assert.Equal(t, expectedOutput, actualOutput, "Actual output does not match expected output")

	if passed {
		log.Println("Case passed:", theCase.Name)
		log.Println()
	} else {
		log.Printf("\n\nExpected output `archstats %s`):\n%s\n", strings.Join(allArgs, " "), expectedOutput)
		log.Println("\n" + expectedOutput)
		log.Printf("\n\nActual output for `archstats %s`):\n%s\n", strings.Join(allArgs, " "), actualOutput)
	}
}

type Config struct {
	Cases []Case `yaml:"cases"`
}

type Case struct {
	Name               string `yaml:"name"`
	Repo               string `yaml:"repo"`
	Commit             string `yaml:"commit"`
	OptionArgs         string `yaml:"options"`
	Ignore             string `yaml:"ignore"`
	ExpectedOutputFile string `yaml:"expectedOutputFile"`
}
