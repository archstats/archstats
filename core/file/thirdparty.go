package file

import (
	"path"
	"strings"
)

// IsThirdParty reports whether a file is someone else's code carried in the
// repository: a vendored package, a minified bundle, a well-known library.
//
// It exists because such files were read as the project's own. In
// nopCommerce 46% of the declared units came from minified JavaScript under
// wwwroot/lib_npm, named `$`, `$e` and `$i`; in BroadleafCommerce the hottest
// file in the codebase was a vendored WYSIWYG editor, and moment.js was
// reported as the module "where an extraction starts". Such files still
// count as files and lines -- they are in the repository -- but they declare
// no units and receive no code-health reading.
//
// content may be nil, in which case only the path is judged.
func (c *Classifier) IsThirdParty(filePath string, content []byte) bool {
	p := strings.ToLower(strings.ReplaceAll(filePath, "\\", "/"))
	for _, dir := range c.thirdPartyDirs {
		if c.InDirOutsideSourceRoot(p, dir) {
			return true
		}
	}
	for _, conv := range c.conventions {
		if conv.IsThirdParty != nil && conv.IsThirdParty(p, content) {
			return true
		}
	}
	return false
}

// Directories that hold someone else's code in any ecosystem. A package
// manager's own folder (node_modules/) is its pack's.
var genericThirdPartyDirs = []string{
	"vendor/", "vendors/", "third_party/", "third-party/", "thirdparty/",
}

// InDirOutsideSourceRoot reports whether a path runs through the given
// directory ("vendor/", "build/") in a way that makes it that directory
// rather than a package named like it.
//
// In some languages a directory is a package (Conventions.PackageDirExts),
// and packages are called vendor, build and target all the time: Broadleaf's
// src/main/java/.../common/vendor/service holds 22 of its own classes, which
// were read as vendored code. For those languages the directory counts only
// when it comes before the source root ("module/target/generated/..."), not
// inside it ("module/src/main/java/org/x/build/...").
func (c *Classifier) InDirOutsideSourceRoot(lowerPath, dir string) bool {
	idx := -1
	if strings.HasPrefix(lowerPath, dir) {
		idx = 0
	} else if i := strings.Index(lowerPath, "/"+dir); i >= 0 {
		idx = i + 1
	}
	if idx < 0 {
		return false
	}
	if c != nil && c.packageDirExts[path.Ext(lowerPath)] {
		if src := strings.Index(lowerPath, "src/"); src >= 0 && src < idx {
			return false
		}
	}
	return true
}
