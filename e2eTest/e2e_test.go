package e2eTest

import (
	"bytes"
	"github.com/archstats/archstats/cmd"
	"github.com/archstats/archstats/core"
	"github.com/archstats/archstats/extensions/regex"
	"github.com/jszwec/csvutil"
	"github.com/stretchr/testify/assert"
	"regexp"
	"strings"
	"testing"
)

func Test_SimpleComponents_AfferentEfferentCoupling(t *testing.T) {
	simpleComponentsTest(t, "components", "name,complexity:files,modularity__coupling__afferent,modularity__coupling__efferent", []Component{
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
			regexp.MustCompile(`component (?P<modularity__component__declarations>[a-z]+)`),
			regexp.MustCompile(`depends on (?P<modularity__component__imports>[a-z]+)`),
		},
	}

	output := bytes.NewBufferString("")

	//cmd.Reset()
	err := cmd.Execute(output, bytes.NewBufferString(""), []core.Extension{
		ext,
	}, []string{"-o", "csv", "-f", "simple_components", "view", view})
	if err != nil {
		t.Error(err)
		return
	} //"-c", "name,coupling:afferent:count,coupling:efferent:count",

	var actualOutput []T
	stringOutput := string(output.Bytes())
	err = csvutil.Unmarshal([]byte(stringOutput), &actualOutput)

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
	FileCount         int    `csv:"COMPLEXITY__FILES"`
	AfferentCouplings int    `csv:"MODULARITY__COUPLING__AFFERENT,omitempty"`
	EfferentCouplings int    `csv:"MODULARITY__COUPLING__EFFERENT,omitempty"`
}

func directConnection(from, to, file string, referenceCount int) ComponentConnectionDirect {
	return ComponentConnectionDirect{
		From: from,
		To:   to,
		// TODO, windows... but we need a better solution for this
		File:           strings.ReplaceAll(file, "\\", "/"),
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
