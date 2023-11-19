package common

import "github.com/archstats/archstats/core/file"

func DirectoryBasedComponentResolution(r *file.Results) string {
	return r.Directory
}
