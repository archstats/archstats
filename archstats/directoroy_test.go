package archstats

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

func Test(t *testing.T) {
	var root Directory
	//root = &directory{
	//	path: "rootPath",
	//	subDirectories: []*directory{
	//		{
	//			path: "relPath1",
	//			subDirectories: []*directory{
	//				{
	//					path: "relPath2",
	//					subDirectories: []*directory{
	//						{path: "relPathWithFile1"},
	//						{path: "relPathWithFiles2", files: []*file{}},
	//					}}}}}}

	assert.Equal(t, "rootPath", root.Name())
	assert.Len(t, root.SubDirectoriesRecursive(), 4)

}
