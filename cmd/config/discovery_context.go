package config

import (
	"os"
	"path/filepath"
	"strings"
)

type DiscoveryContext struct {
	RootDir string
	Files   []string
}

func (d *DiscoveryContext) HasFileExtension(ext string) bool {
	if !strings.HasPrefix(ext, ".") {
		ext = "." + ext
	}
	extLower := strings.ToLower(ext)
	for _, f := range d.Files {
		if strings.HasSuffix(strings.ToLower(f), extLower) {
			return true
		}
	}
	return false
}

func (d *DiscoveryContext) HasPath(path string) bool {
	_, err := os.Stat(filepath.Join(d.RootDir, path))
	return err == nil
}
