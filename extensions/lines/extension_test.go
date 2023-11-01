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

	analyzer := &Extension{}

	results := analyzer.AnalyzeFile(&fakeFile{
		content: content,
	})

	assert.Len(t, results.Stats, 1)
	assert.Equal(t, results.Stats[0].StatType, LineCount)
	assert.Equal(t, results.Stats[0].Value, 8)
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
