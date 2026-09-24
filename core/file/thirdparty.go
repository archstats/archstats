package file

import (
	"bytes"
	"path"
	"regexp"
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
func IsThirdParty(filePath string, content []byte) bool {
	p := strings.ToLower(strings.ReplaceAll(filePath, "\\", "/"))
	for _, dir := range thirdPartyDirs {
		if InDirOutsideSourceRoot(p, dir) {
			return true
		}
	}
	base := path.Base(p)
	if minifiedName.MatchString(base) || knownLibrary.MatchString(base) {
		return true
	}
	ext := path.Ext(base)
	if (ext == ".js" || ext == ".mjs" || ext == ".cjs" || ext == ".css") && content != nil {
		return looksMinified(content) || hasLibraryBanner(content)
	}
	return false
}

// hasLibraryBanner reports whether a script opens with a distributed
// library's banner: a licence and a release version in its first comment,
// "DOMPurify 3.4.14 | (c) Cure53 | Released under the Apache license".
//
// Names alone cannot keep up. nopCommerce's forum plugin carries purify.js
// and Broadleaf's admin carries spectrum.js, and both led the "crowded
// module" finding with 65 and 79 declarations nobody there wrote. A
// project's own files may carry a licence header, but not a three-part
// release number: Broadleaf's reads "Fair Use License Agreement, Version 1.0".
func hasLibraryBanner(content []byte) bool {
	head := content
	if len(head) > 1024 {
		head = head[:1024]
	}
	comment := leadingComment.Find(head)
	if comment == nil {
		return false
	}
	return bannerLicence.Match(comment) && releaseVersion.Match(comment)
}

var (
	leadingComment = regexp.MustCompile(`^\x{FEFF}?\s*(?:/\*(?s:.*?)\*/|(?://[^\n]*\n\s*)+)`)
	bannerLicence  = regexp.MustCompile(`(?i)@license|license:?\s*(mit|bsd|isc|apache)|released under the (mit|bsd|apache)|licensed (under the )?(mit|bsd|apache)|\|\s*(mit|bsd|isc)\b|\(c\)|copyright`)
	releaseVersion = regexp.MustCompile(`\bv?\d+\.\d+\.\d+\b`)
)

var thirdPartyDirs = []string{
	"node_modules/", "bower_components/", "jspm_packages/", "vendor/", "vendors/",
	"third_party/", "third-party/", "thirdparty/", "lib_npm/", "wwwroot/lib/",
}

var minifiedName = regexp.MustCompile(`[.-]min\.(js|mjs|cjs|css)$|\.(bundle|pack)\.js$|\.min\.[a-z0-9.]+\.(js|css)$`)

// Libraries recognised by their distribution file names. Anchored on the
// whole name, so a project's own `chart-panel.js` or `moment-utils.ts` is not
// mistaken for one.
var knownLibrary = regexp.MustCompile(`^(` +
	`jquery([.-][a-z0-9.-]*)?|` +
	`bootstrap(\.bundle)?|modernizr([.-][a-z0-9.-]*)?|` +
	`moment(-with-locales|-timezone[a-z0-9.-]*)?|` +
	`redactor|typeahead(\.bundle|\.jquery)?|select2(\.full)?|` +
	`underscore|lodash(\.core)?|backbone|handlebars(\.runtime)?|mustache|knockout(-[0-9.]+)?|` +
	`require|tinymce|ckeditor|codemirror|highcharts|raphael|d3(\.v[0-9]+)?|` +
	`angular|react(-dom)?(\.development|\.production)?|vue(\.runtime)?(\.global|\.esm-browser)?|` +
	`popper|chart(\.umd)?|font-awesome|fontawesome(-all)?` +
	`)\.(js|css)$`)

// looksMinified is the content test for bundles that do not say so in their
// name: minifiers put hundreds of characters on a line, people do not.
func looksMinified(content []byte) bool {
	lines := bytes.Count(content, []byte("\n")) + 1
	if len(content)/lines < 200 {
		return false
	}
	for _, line := range bytes.Split(content, []byte("\n")) {
		if len(line) > 1000 {
			return true
		}
	}
	return false
}

// InDirOutsideSourceRoot reports whether a path runs through the given
// directory ("vendor/", "build/") in a way that makes it that directory
// rather than a package named like it.
//
// In JVM and .NET code a directory is a package, and packages are called
// vendor, build and target all the time: Broadleaf's
// src/main/java/.../common/vendor/service holds 22 of its own classes, which
// were read as vendored code. For those languages the directory counts only
// when it comes before the source root ("module/target/generated/..."), not
// inside it ("module/src/main/java/org/x/build/...").
func InDirOutsideSourceRoot(lowerPath, dir string) bool {
	idx := -1
	if strings.HasPrefix(lowerPath, dir) {
		idx = 0
	} else if i := strings.Index(lowerPath, "/"+dir); i >= 0 {
		idx = i + 1
	}
	if idx < 0 {
		return false
	}
	if packageDirLanguages[path.Ext(lowerPath)] {
		if src := strings.Index(lowerPath, "src/"); src >= 0 && src < idx {
			return false
		}
	}
	return true
}

var packageDirLanguages = map[string]bool{
	".java": true, ".kt": true, ".kts": true, ".scala": true, ".groovy": true,
	".cs": true, ".vb": true, ".fs": true,
}
