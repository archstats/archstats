package mobile

import (
	"bytes"
	"encoding/xml"
	"regexp"
	"strings"
)

// What an Android module declares to the platform, from its
// AndroidManifest.xml: the activities, services, receivers and providers the
// system may start, which of those other apps may start too, the permissions
// it asks for and the links it answers. For a mobile app this is the surface
// a service's routes are for a server, and it is written nowhere in the code.

// A manifestEntry is one element of a manifest worth a row.
type manifestEntry struct {
	Kind string // activity, service, receiver, provider, permission, feature, deep_link, application
	// A class for the components, a permission or feature name, or for a deep
	// link the component that answers it.
	Name  string
	Value string
	// "true", "false", or "" when the manifest leaves it to the default.
	Exported string
	Launcher bool
	Line     int
}

type androidManifest struct {
	Package string
	Entries []manifestEntry
}

const androidNS = "http://schemas.android.com/apk/res/android"

// readAndroidManifest reads the elements that say what an app is. A manifest
// that is not XML yields nothing: flavour manifests are sometimes templates.
func readAndroidManifest(content []byte) *androidManifest {
	m := &androidManifest{}
	lines := newLineIndex(content)
	dec := xml.NewDecoder(bytes.NewReader(content))
	dec.Strict = false

	var component *manifestEntry
	var inFilter bool
	var actions, cats []string
	var data []map[string]string
	flushFilter := func() {
		if component == nil {
			return
		}
		if contains(actions, "android.intent.action.MAIN") && contains(cats, "android.intent.category.LAUNCHER") {
			component.Launcher = true
		}
		if contains(actions, "android.intent.action.VIEW") && contains(cats, "android.intent.category.BROWSABLE") {
			for _, link := range deepLinks(data) {
				m.Entries = append(m.Entries, manifestEntry{Kind: "deep_link", Name: component.Name, Value: link, Line: component.Line})
			}
		}
		actions, cats, data = nil, nil, nil
	}
	var components []*manifestEntry
	for {
		offset := dec.InputOffset()
		tok, err := dec.Token()
		if err != nil {
			break
		}
		switch t := tok.(type) {
		case xml.StartElement:
			line := lines.at(int(offset))
			attr := func(name string) string {
				for _, a := range t.Attr {
					if a.Name.Local == name && (a.Name.Space == androidNS || a.Name.Space == "android" || a.Name.Space == "") {
						return a.Value
					}
				}
				return ""
			}
			switch t.Name.Local {
			case "manifest":
				m.Package = attr("package")
			case "application":
				if name := attr("name"); name != "" {
					m.Entries = append(m.Entries, manifestEntry{Kind: "application", Name: name, Line: line})
				}
			case "activity", "activity-alias", "service", "receiver", "provider":
				kind := strings.TrimSuffix(t.Name.Local, "-alias")
				name := attr("name")
				if t.Name.Local == "activity-alias" {
					name = attr("targetActivity")
				}
				e := &manifestEntry{Kind: kind, Name: name, Exported: attr("exported"), Line: line}
				components = append(components, e)
				component = e
			case "intent-filter":
				inFilter = true
				if component != nil && component.Exported == "" {
					// Before Android 12 a component with an intent filter was
					// exported unless it said otherwise; since then it must say.
					component.Exported = "implied"
				}
			case "action":
				if inFilter {
					actions = append(actions, attr("name"))
				}
			case "category":
				if inFilter {
					cats = append(cats, attr("name"))
				}
			case "data":
				if inFilter {
					data = append(data, map[string]string{"scheme": attr("scheme"), "host": attr("host"), "path": attr("path") + attr("pathPrefix") + attr("pathPattern")})
				}
			case "uses-permission", "uses-permission-sdk-23", "uses-permission-sdk-m":
				if name := attr("name"); name != "" {
					m.Entries = append(m.Entries, manifestEntry{Kind: "permission", Name: name, Line: line, Value: attr("maxSdkVersion")})
				}
			case "uses-feature":
				if name := attr("name"); name != "" {
					m.Entries = append(m.Entries, manifestEntry{Kind: "feature", Name: name, Value: attr("required"), Line: line})
				}
			}
		case xml.EndElement:
			switch t.Name.Local {
			case "intent-filter":
				flushFilter()
				inFilter = false
			case "activity", "activity-alias", "service", "receiver", "provider":
				component = nil
			}
		}
	}
	for _, c := range components {
		m.Entries = append(m.Entries, *c)
	}
	return m
}

// deepLinks turns a filter's <data> elements into the links it answers. The
// elements of one filter combine: a scheme on one line and a host on the next
// describe one link.
func deepLinks(data []map[string]string) []string {
	var schemes, hosts, paths []string
	for _, d := range data {
		if d["scheme"] != "" {
			schemes = appendUnique(schemes, d["scheme"])
		}
		if d["host"] != "" {
			hosts = appendUnique(hosts, d["host"])
		}
		if d["path"] != "" {
			paths = appendUnique(paths, d["path"])
		}
	}
	if len(hosts) == 0 {
		hosts = []string{""}
	}
	if len(paths) == 0 {
		paths = []string{""}
	}
	var out []string
	for _, s := range schemes {
		for _, h := range hosts {
			for _, p := range paths {
				out = append(out, s+"://"+h+p)
			}
		}
	}
	return out
}

var gradleNamespace = regexp.MustCompile(`(?m)^\s*namespace\s*=?\s*["']([A-Za-z0-9_.]+)["']`)
var gradleApplicationID = regexp.MustCompile(`(?m)^\s*applicationId\s*=?\s*["']([A-Za-z0-9_.]+)["']`)

// className resolves a manifest's class name the way the build does: a
// leading dot, or no dot at all, is relative to the package.
func className(name, pkg string) string {
	name = strings.ReplaceAll(name, "$", ".")
	switch {
	case name == "":
		return ""
	case strings.HasPrefix(name, "."):
		return pkg + name
	case !strings.Contains(name, ".") && pkg != "":
		return pkg + "." + name
	}
	return name
}

func contains(list []string, v string) bool {
	for _, x := range list {
		if x == v {
			return true
		}
	}
	return false
}

func appendUnique(list []string, v string) []string {
	if contains(list, v) {
		return list
	}
	return append(list, v)
}

// lineIndex answers which line a byte offset is on.
type lineIndex []int

func newLineIndex(content []byte) lineIndex {
	idx := lineIndex{0}
	for i, b := range content {
		if b == '\n' {
			idx = append(idx, i+1)
		}
	}
	return idx
}

func (l lineIndex) at(offset int) int {
	lo, hi := 0, len(l)-1
	for lo < hi {
		mid := (lo + hi + 1) / 2
		if l[mid] <= offset {
			lo = mid
		} else {
			hi = mid - 1
		}
	}
	return lo + 1
}
