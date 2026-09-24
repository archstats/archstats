package lines

import (
	"github.com/stretchr/testify/assert"
	"io/fs"
	"os"
	"testing"
	"time"
)

func TestFileInput(t *testing.T) {
	// read real_test.txt
	content, err := os.ReadFile("real_test.txt")
	if err != nil {
		return
	}

	analyzer := &extension{}

	results := analyzer.AnalyzeFile(&fakeFile{
		content: content,
	})

	assert.Len(t, results.Stats, 1)
	assert.Equal(t, results.Stats[0].StatType, LineCount)
	// real_test.txt has seven lines and ends with a newline, as `wc -l` agrees.
	assert.Equal(t, 7, results.Stats[0].Value)
}

func TestLinesAreCountedLikeAnEditorCountsThem(t *testing.T) {
	cases := map[string]int{
		"":               0,
		"\n":             1,
		"one":            1,
		"one\n":          1,
		"one\ntwo":       2,
		"one\ntwo\n":     2,
		"one\r\ntwo\r\n": 2,
		"one\n\n":        2,
	}
	for content, want := range cases {
		assert.Equalf(t, want, countLines([]byte(content)), "%q", content)
	}
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
