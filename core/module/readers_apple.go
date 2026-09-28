package module

import (
	"os"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// ---------------------------------------------------------------------------
// swiftpm: Swift packages
//
// A Swift target is a module: every file in it sees every other without an
// import, and `import Timeline` names the target, not a directory. That makes
// the target the unit the Swift pack resolves type references within, and
// IceCubesApp and isowords keep their whole architecture in Package.swift --
// thirteen packages in one, 124 targets in the other.
// ---------------------------------------------------------------------------

type swiftpmReader struct{}

func (swiftpmReader) Kind() string         { return "swiftpm" }
func (swiftpmReader) Claims(b string) bool { return b == "Package.swift" }

var (
	swiftTargetCall = regexp.MustCompile(`\.(target|executableTarget|testTarget|macro|plugin|systemLibrary)\s*\(`)
	swiftName       = regexp.MustCompile(`\bname\s*:\s*"([^"]+)"`)
	swiftPath       = regexp.MustCompile(`\bpath\s*:\s*"([^"]+)"`)
	swiftDepsLabel  = regexp.MustCompile(`\bdependencies\s*:\s*\[`)
	// A dependency is a bare string, .target(name:), .byName(name:) or
	// .product(name:package:); each names a module by its first string.
	swiftDepEntry = regexp.MustCompile(`"([^"]+)"|\.(?:target|byName|product)\s*\(\s*name\s*:\s*"([^"]+)"`)
)

func (swiftpmReader) Read(root, manifest string) []*Module {
	raw, ok := readFile(manifest)
	if !ok {
		return nil
	}
	raw = stripSwiftComments(raw)
	dir := relDir(root, manifest)
	// `.target(name:)` inside a dependency list is a reference to a target,
	// not a declaration of one.
	var lists [][2]int
	for _, m := range swiftDepsLabel.FindAllStringIndex(raw, -1) {
		lists = append(lists, [2]int{m[1] - 1, m[1] + len(balanced(raw, m[1]-1, '[', ']'))})
	}
	inList := func(at int) bool {
		for _, l := range lists {
			if at > l[0] && at <= l[1] {
				return true
			}
		}
		return false
	}
	var out []*Module
	for _, loc := range swiftTargetCall.FindAllStringSubmatchIndex(raw, -1) {
		if inList(loc[0]) {
			continue
		}
		kind := raw[loc[2]:loc[3]]
		body := balanced(raw, loc[1]-1, '(', ')')
		name := firstGroup(swiftName, body)
		if name == "" {
			continue
		}
		targetDir := firstGroup(swiftPath, body)
		if targetDir == "" {
			// SwiftPM's own search: the first of its source folders holding
			// a directory of the target's name. Kickstarter keeps its test
			// targets under Sources/, not Tests/.
			bases := []string{"Sources", "Source", "src", "srcs"}
			switch kind {
			case "testTarget":
				bases = append([]string{"Tests"}, bases...)
			case "plugin":
				bases = []string{"Plugins"}
			}
			targetDir = bases[0] + "/" + name
			for _, b := range bases {
				if isDir(filepath.Join(filepath.Dir(manifest), b, name)) {
					targetDir = b + "/" + name
					break
				}
			}
		}
		var deps []string
		if m := swiftDepsLabel.FindStringIndex(body); m != nil {
			list := balanced(body, m[1]-1, '[', ']')
			for _, d := range swiftDepEntry.FindAllStringSubmatch(list, -1) {
				dep := d[1]
				if dep == "" {
					dep = d[2]
				}
				// `.product(name: "X", package: "y")` also matches "y" as a
				// bare string; the package is not a module.
				if isPackageArg(list, dep) {
					continue
				}
				deps = appendUnique(deps, dep)
			}
		}
		typ := map[string]string{"target": "library", "executableTarget": "executable", "testTarget": "test",
			"macro": "macro", "plugin": "plugin", "systemLibrary": "system-library"}[kind]
		out = append(out, &Module{
			Name:      name,
			Dir:       joinRel(dir, targetDir),
			Manifest:  relFile(root, manifest),
			DependsOn: deps,
			Type:      typ,
		})
	}
	return out
}

func isPackageArg(list, value string) bool {
	return regexp.MustCompile(`package\s*:\s*"`+regexp.QuoteMeta(value)+`"`).MatchString(list) &&
		!regexp.MustCompile(`name\s*:\s*"`+regexp.QuoteMeta(value)+`"`).MatchString(list)
}

// ---------------------------------------------------------------------------
// xcode: project.pbxproj
//
// An Xcode project is where an app target lives -- the app, its widgets and
// share extensions -- and which Swift packages each one links. The file is an
// old-style property list, parsed as one; each native target becomes a
// module whose files are those in its Sources build phase, or in the folders
// Xcode 16 synchronises with it.
// ---------------------------------------------------------------------------

type xcodeReader struct{}

func (xcodeReader) Kind() string         { return "xcode" }
func (xcodeReader) Claims(b string) bool { return b == "project.pbxproj" }

func (xcodeReader) Read(root, manifest string) []*Module {
	raw, ok := readFile(manifest)
	if !ok {
		return nil
	}
	parsed, ok := parseOldPlist(raw).(map[string]any)
	if !ok {
		return nil
	}
	objects, _ := parsed["objects"].(map[string]any)
	if objects == nil {
		return nil
	}
	obj := func(id any) map[string]any {
		s, _ := id.(string)
		o, _ := objects[s].(map[string]any)
		return o
	}
	str := func(o map[string]any, k string) string {
		s, _ := o[k].(string)
		return s
	}
	list := func(o map[string]any, k string) []any {
		l, _ := o[k].([]any)
		return l
	}
	// The directory holding the .xcodeproj is SOURCE_ROOT.
	projectDir := relDir(root, filepath.Dir(manifest))

	// Every group and file's path, resolved through the group tree.
	paths := map[string]string{}
	var walk func(id, parent string)
	walk = func(id, parent string) {
		o := objects[id]
		m, _ := o.(map[string]any)
		if m == nil {
			return
		}
		p := str(m, "path")
		here := parent
		switch str(m, "sourceTree") {
		case "<group>":
			if p != "" {
				here = joinRel(parent, p)
			}
		case "SOURCE_ROOT":
			here = joinRel(projectDir, p)
		default:
			// SDKROOT, BUILT_PRODUCTS_DIR, DEVELOPER_DIR: not in the checkout.
			if p != "" {
				here = ""
			}
		}
		paths[id] = here
		for _, c := range list(m, "children") {
			if cs, ok := c.(string); ok {
				walk(cs, here)
			}
		}
	}
	for _, v := range objects {
		if m, ok := v.(map[string]any); ok && str(m, "isa") == "PBXProject" {
			walk(str(m, "mainGroup"), projectDir)
		}
	}

	// A synchronised folder's exception set names a target and some files.
	// For the target that owns the folder they are left out; for any other
	// target they are added: IceCubesNotifications owns no folder and is
	// two files of the folder with its name.
	owns := map[string]map[string]bool{} // target id -> synchronised groups it lists
	for id, v := range objects {
		if m, ok := v.(map[string]any); ok && str(m, "isa") == "PBXNativeTarget" {
			owns[id] = map[string]bool{}
			for _, g := range list(m, "fileSystemSynchronizedGroups") {
				if gs, ok := g.(string); ok {
					owns[id][gs] = true
				}
			}
		}
	}
	added := map[string][]string{} // target id -> files
	for gid, v := range objects {
		g, ok := v.(map[string]any)
		if !ok || str(g, "isa") != "PBXFileSystemSynchronizedRootGroup" || paths[gid] == "" {
			continue
		}
		for _, e := range list(g, "exceptions") {
			ex := obj(e)
			target := str(ex, "target")
			if owns[target] == nil || owns[target][gid] {
				continue
			}
			for _, f := range list(ex, "membershipExceptions") {
				if fs, ok := f.(string); ok {
					added[target] = append(added[target], joinRel(paths[gid], fs))
				}
			}
		}
	}

	var out []*Module
	names := map[string]string{} // target id -> name
	for id, v := range objects {
		if m, ok := v.(map[string]any); ok && str(m, "isa") == "PBXNativeTarget" {
			names[id] = str(m, "name")
		}
	}
	ids := make([]string, 0, len(names))
	for id := range names {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	for _, id := range ids {
		t := obj(id)
		mod := &Module{Name: names[id], Manifest: relFile(root, manifest), Type: xcodeProductType(str(t, "productType"))}
		var files, dirs []string
		for _, phase := range list(t, "buildPhases") {
			ph := obj(phase)
			if str(ph, "isa") != "PBXSourcesBuildPhase" {
				continue
			}
			for _, bf := range list(ph, "files") {
				if p := paths[str(obj(bf), "fileRef")]; p != "" {
					files = append(files, p)
				}
			}
		}
		for _, g := range list(t, "fileSystemSynchronizedGroups") {
			gs, _ := g.(string)
			if p := paths[gs]; p != "" {
				dirs = append(dirs, p)
			}
		}
		for _, d := range list(t, "dependencies") {
			if n := names[str(obj(d), "target")]; n != "" {
				mod.DependsOn = appendUnique(mod.DependsOn, n)
			}
		}
		for _, d := range list(t, "packageProductDependencies") {
			if n := str(obj(d), "productName"); n != "" {
				mod.DependsOn = appendUnique(mod.DependsOn, n)
			}
		}
		// The directory is where the target's own code is: the folders it
		// owns, else its Sources files. An empty directory would claim every
		// file in the repository nothing else claims, so a target whose files
		// share no folder takes the folder most of them are in.
		own, extra := sourceFiles(files), sourceFiles(added[id])
		switch {
		case len(dirs) > 0:
			mod.Dir, mod.Dirs = dirs[0], dirs[1:]
		case len(own) > 0:
			mod.Dir = commonDir(parentsOf(own))
			if mod.Dir == "" {
				mod.Dir = mostCommon(parentsOf(own))
			}
		case len(extra) > 0:
			mod.Dir = mostCommon(parentsOf(extra))
		default:
			mod.Dir = projectDir
		}
		mod.Files = append(files, added[id]...)
		out = append(out, mod)
	}
	return out
}

// xcodeProductType shortens com.apple.product-type.application and its
// relatives to what they build.
func xcodeProductType(t string) string {
	t = strings.TrimPrefix(t, "com.apple.product-type.")
	switch {
	case t == "application", strings.HasPrefix(t, "application."):
		return "ios-application"
	case strings.Contains(t, "extension"):
		return "app-extension"
	case strings.HasPrefix(t, "bundle.unit-test"), strings.HasPrefix(t, "bundle.ui-testing"):
		return "test"
	case t == "framework", t == "framework.static":
		return "framework"
	case strings.HasPrefix(t, "library"):
		return "library"
	case t == "":
		return ""
	}
	return t
}

func parentsOf(files []string) []string {
	out := make([]string, 0, len(files))
	for _, f := range files {
		out = append(out, path.Dir(f))
	}
	return out
}

// sourceFiles keeps the code: a target's Sources phase also compiles string
// catalogs and model files, which say nothing about where its code lives.
func sourceFiles(files []string) []string {
	var out []string
	for _, f := range files {
		switch strings.ToLower(path.Ext(f)) {
		case ".swift", ".m", ".mm", ".h", ".c", ".cpp", ".kt", ".java":
			out = append(out, f)
		}
	}
	return out
}

func mostCommon(dirs []string) string {
	count := map[string]int{}
	best := ""
	for _, d := range dirs {
		count[d]++
		if count[d] > count[best] || (count[d] == count[best] && d < best) {
			best = d
		}
	}
	return best
}

// commonDir is the deepest directory every path is inside.
func commonDir(dirs []string) string {
	if len(dirs) == 0 {
		return ""
	}
	common := strings.Split(dirs[0], "/")
	for _, d := range dirs[1:] {
		parts := strings.Split(d, "/")
		n := 0
		for n < len(common) && n < len(parts) && common[n] == parts[n] {
			n++
		}
		common = common[:n]
	}
	out := strings.Join(common, "/")
	if out == "." {
		return ""
	}
	return out
}

func joinRel(dir, p string) string {
	if dir == "" || dir == "." {
		return path.Clean(p)
	}
	return path.Clean(dir + "/" + p)
}

func isDir(p string) bool {
	info, err := os.Stat(p)
	return err == nil && info.IsDir()
}

func firstGroup(re *regexp.Regexp, s string) string {
	if m := re.FindStringSubmatch(s); m != nil {
		return m[1]
	}
	return ""
}

// balanced returns the text between the bracket at open and its match,
// skipping string literals.
func balanced(s string, open int, l, r byte) string {
	depth := 0
	inString := false
	for i := open; i < len(s); i++ {
		c := s[i]
		switch {
		case inString:
			if c == '\\' {
				i++
			} else if c == '"' {
				inString = false
			}
		case c == '"':
			inString = true
		case c == l:
			depth++
		case c == r:
			depth--
			if depth == 0 {
				return s[open+1 : i]
			}
		}
	}
	return s[open+1:]
}

// stripSwiftComments removes // and /* */ comments outside string literals:
// a URL's "https://" is not a comment.
func stripSwiftComments(s string) string {
	var b strings.Builder
	inString := false
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case inString:
			b.WriteByte(c)
			if c == '\\' && i+1 < len(s) {
				i++
				b.WriteByte(s[i])
			} else if c == '"' {
				inString = false
			}
		case c == '"':
			inString = true
			b.WriteByte(c)
		case strings.HasPrefix(s[i:], "//"):
			for i < len(s) && s[i] != '\n' {
				i++
			}
			if i < len(s) {
				b.WriteByte('\n')
			}
		case strings.HasPrefix(s[i:], "/*"):
			end := strings.Index(s[i+2:], "*/")
			if end < 0 {
				return b.String()
			}
			i += end + 3
		default:
			b.WriteByte(c)
		}
	}
	return b.String()
}

// ParseOldPlist reads an old-style property list: dictionaries become
// map[string]any, arrays []any, everything else a string.
func ParseOldPlist(s string) any { return parseOldPlist(s) }

// parseOldPlist reads the NeXTSTEP property list format Xcode writes
// project.pbxproj in: dictionaries in braces, arrays in parentheses, strings
// quoted or bare, and comments.
func parseOldPlist(s string) any {
	p := &plistParser{s: s}
	return p.value()
}

type plistParser struct {
	s string
	i int
}

func (p *plistParser) skip() {
	for p.i < len(p.s) {
		c := p.s[p.i]
		switch {
		case c == ' ' || c == '\t' || c == '\n' || c == '\r':
			p.i++
		case strings.HasPrefix(p.s[p.i:], "/*"):
			end := strings.Index(p.s[p.i+2:], "*/")
			if end < 0 {
				p.i = len(p.s)
				return
			}
			p.i += end + 4
		case strings.HasPrefix(p.s[p.i:], "//"):
			end := strings.IndexByte(p.s[p.i:], '\n')
			if end < 0 {
				p.i = len(p.s)
				return
			}
			p.i += end
		default:
			return
		}
	}
}

func (p *plistParser) value() any {
	p.skip()
	if p.i >= len(p.s) {
		return nil
	}
	switch p.s[p.i] {
	case '{':
		p.i++
		m := map[string]any{}
		for {
			p.skip()
			if p.i >= len(p.s) {
				return m
			}
			if p.s[p.i] == '}' {
				p.i++
				return m
			}
			key, _ := p.value().(string)
			p.skip()
			if p.i < len(p.s) && p.s[p.i] == '=' {
				p.i++
			}
			m[key] = p.value()
			p.skip()
			if p.i < len(p.s) && p.s[p.i] == ';' {
				p.i++
			}
		}
	case '(':
		p.i++
		var l []any
		for {
			p.skip()
			if p.i >= len(p.s) {
				return l
			}
			if p.s[p.i] == ')' {
				p.i++
				return l
			}
			l = append(l, p.value())
			p.skip()
			if p.i < len(p.s) && p.s[p.i] == ',' {
				p.i++
			}
		}
	case '"':
		p.i++
		var b strings.Builder
		for p.i < len(p.s) && p.s[p.i] != '"' {
			if p.s[p.i] == '\\' && p.i+1 < len(p.s) {
				p.i++
				switch p.s[p.i] {
				case 'n':
					b.WriteByte('\n')
				case 't':
					b.WriteByte('\t')
				default:
					b.WriteByte(p.s[p.i])
				}
			} else {
				b.WriteByte(p.s[p.i])
			}
			p.i++
		}
		p.i++
		return b.String()
	default:
		start := p.i
		for p.i < len(p.s) && !strings.ContainsRune(" \t\r\n{}();,=\"", rune(p.s[p.i])) {
			if strings.HasPrefix(p.s[p.i:], "/*") {
				break
			}
			p.i++
		}
		if p.i == start {
			// A character the format does not allow here; step over it
			// rather than loop.
			p.i++
			return ""
		}
		return p.s[start:p.i]
	}
}
