package ecmascript

import (
	"bytes"
	"path"
	"regexp"
	"strings"

	"github.com/archstats/archstats/core/file"
)

// Conventions are JavaScript's and TypeScript's: their test files, the
// folders npm, Bower and jspm install into, and the bundles and libraries a
// web project ships beside its own code.
func Conventions() file.Conventions {
	return file.Conventions{
		Name:           "ecmascript",
		IsTest:         isTest,
		IsThirdParty:   isThirdParty,
		ThirdPartyDirs: []string{"node_modules/", "bower_components/", "jspm_packages/", "lib_npm/"},
	}
}

// __tests__/ and cypress/ folders; *.test.ts, *.spec.jsx and the like.
var (
	testDir  = regexp.MustCompile(`(^|/)(__tests__|cypress)(/|$)`)
	testName = regexp.MustCompile(`\.(test|spec)\.[cm]?[jt]sx?$`)
)

func isTest(p string) bool {
	return testDir.MatchString(strings.ToLower(p)) || testName.MatchString(path.Base(p))
}

// isThirdParty recognises a script or stylesheet someone else wrote: by a
// minified or bundled name, a well-known library's name, or content that is
// minified or opens with a release banner.
func isThirdParty(lowerPath string, content []byte) bool {
	base := path.Base(lowerPath)
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
