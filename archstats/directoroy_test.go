package archstats

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

func Test(t *testing.T) {

	var root Directory
	root = &directory{
		Path: "rootPath",
		SubDirectories: []*directory{
			{
				Path: "relPath1",
				SubDirectories: []*directory{
					{
						Path: "relPath2",
						SubDirectories: []*directory{
							{Path: "relPathWithFile1"},
							{Path: "relPathWithFiles2", Files: []*file{}},
						}}}}}}

	assert.Equal(t, "rootPath", root.Identity())
	assert.Len(t, root.GetDescendantSubDirectories(), 4)

}
