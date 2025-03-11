package indentations

import (
	"github.com/archstats/archstats/core/stats"
	"github.com/samber/lo"
	"github.com/stretchr/testify/assert"
	"io/fs"
	"io/ioutil"
	"os"
	"testing"
	"time"
)

func TestIndentationLogic(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want int
	}{
		{"no spaces", "testIndentation", 0},
		{"2 spaces", "  testIndentation", 0},
		{"4 spaces", "    testIndentation", 1},
		{"5 spaces", "     testIndentation", 1},
		{"8 spaces", "        testIndentation", 2},
		{"12 spaces", "            testIndentation", 3},
		{"1 tab", "\ttestIndentation", 1},
		{"2 tabs", "\t\ttestIndentation", 2},
		{"3 tabs", "\t\t\ttestIndentation", 3},
	}
	ext := FourTabs()
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			assert.Equal(t, ext.getLeadingIndentation([]byte(test.in)), test.want)
		})
	}
}

func TestFileInput(t *testing.T) {
	// read real_test.txt
	content, err := ioutil.ReadFile("real_test.txt")
	if err != nil {
		return
	}

	analyzer := FourTabs()

	results := analyzer.AnalyzeFile(&fakeFile{
		content: content,
	})

	stats := lo.GroupBy(results.Stats, func(stat *stats.Record) string {
		return stat.StatType
	})

	assert.Equal(t, stats[Max][0].Value, 4)
	assert.Equal(t, stats[Count][0].Value, 17)
}

func TestMax(t *testing.T) {
	accumulator := maxAccumulator([]interface{}{1, 2, 10, 4, 5})
	assert.Equal(t, accumulator, 10)

}

type fakeFile struct {
	content []byte
}

func (f *fakeFile) Name() string {
	//TODO implement me
	panic("implement me")
}

func (f *fakeFile) Size() int64 {
	//TODO implement me
	panic("implement me")
}

func (f *fakeFile) Mode() fs.FileMode {
	//TODO implement me
	panic("implement me")
}

func (f *fakeFile) ModTime() time.Time {
	//TODO implement me
	panic("implement me")
}

func (f *fakeFile) IsDir() bool {
	//TODO implement me
	panic("implement me")
}

func (f *fakeFile) Sys() any {
	//TODO implement me
	panic("implement me")
}

func (f *fakeFile) Path() string {
	return ""
}

func (f *fakeFile) Info() os.FileInfo {
	return nil
}

func (f *fakeFile) Content() []byte {
	return f.content
}
