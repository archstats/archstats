package mobile

import (
	"bytes"
	"encoding/xml"
	"sort"
	"strings"
)

// What an Apple app declares, from Info.plist and its entitlements: the URL
// schemes it opens, the background modes it runs in, the private data it asks
// for (every NS...UsageDescription is a permission prompt), the domains its
// universal links answer, and whether it switched off App Transport Security.

// A plistValue is one node of an XML property list.
type plistValue struct {
	Kind  string // dict, array, string, bool, number, date, data
	Str   string
	Dict  map[string]*plistValue
	Keys  []string
	Array []*plistValue
	Line  int
}

// readPlist parses an XML property list. A binary plist, or one that is not
// XML, yields nil.
func readPlist(content []byte) *plistValue {
	if !bytes.Contains(content[:min(len(content), 512)], []byte("<plist")) {
		return nil
	}
	lines := newLineIndex(content)
	dec := xml.NewDecoder(bytes.NewReader(content))
	dec.Strict = false
	var parse func(start xml.StartElement, line int) *plistValue
	parse = func(start xml.StartElement, line int) *plistValue {
		v := &plistValue{Line: line}
		switch start.Name.Local {
		case "dict":
			v.Kind, v.Dict = "dict", map[string]*plistValue{}
			key := ""
			for {
				off := dec.InputOffset()
				tok, err := dec.Token()
				if err != nil {
					return v
				}
				switch t := tok.(type) {
				case xml.StartElement:
					if t.Name.Local == "key" {
						key = text(dec)
						continue
					}
					child := parse(t, lines.at(int(off)))
					if _, dup := v.Dict[key]; !dup {
						v.Keys = append(v.Keys, key)
					}
					v.Dict[key] = child
				case xml.EndElement:
					return v
				}
			}
		case "array":
			v.Kind = "array"
			for {
				off := dec.InputOffset()
				tok, err := dec.Token()
				if err != nil {
					return v
				}
				switch t := tok.(type) {
				case xml.StartElement:
					v.Array = append(v.Array, parse(t, lines.at(int(off))))
				case xml.EndElement:
					return v
				}
			}
		case "true", "false":
			v.Kind, v.Str = "bool", start.Name.Local
			_ = dec.Skip()
		case "string", "integer", "real", "date", "data":
			v.Kind = start.Name.Local
			v.Str = text(dec)
		default:
			_ = dec.Skip()
		}
		return v
	}
	for {
		off := dec.InputOffset()
		tok, err := dec.Token()
		if err != nil {
			return nil
		}
		if t, ok := tok.(xml.StartElement); ok && t.Name.Local != "plist" {
			return parse(t, lines.at(int(off)))
		}
	}
}

// text reads an element's character data up to its end tag.
func text(dec *xml.Decoder) string {
	var b strings.Builder
	for {
		tok, err := dec.Token()
		if err != nil {
			return b.String()
		}
		switch t := tok.(type) {
		case xml.CharData:
			b.Write(t)
		case xml.EndElement:
			return strings.TrimSpace(b.String())
		}
	}
}

func (v *plistValue) get(key string) *plistValue {
	if v == nil || v.Dict == nil {
		return nil
	}
	return v.Dict[key]
}

// strings flattens a value to the strings in it.
func (v *plistValue) strings() []string {
	if v == nil {
		return nil
	}
	switch v.Kind {
	case "array":
		var out []string
		for _, x := range v.Array {
			out = append(out, x.strings()...)
		}
		return out
	case "dict":
		return nil
	}
	return []string{v.Str}
}

// infoPlistEntries reads what an Info.plist declares.
func infoPlistEntries(root *plistValue) []manifestEntry {
	if root == nil || root.Kind != "dict" {
		return nil
	}
	var out []manifestEntry
	if id := root.get("CFBundleIdentifier"); id != nil && id.Str != "" {
		out = append(out, manifestEntry{Kind: "bundle_id", Name: id.Str, Line: id.Line})
	}
	if types := root.get("CFBundleURLTypes"); types != nil {
		for _, t := range types.Array {
			for _, scheme := range t.get("CFBundleURLSchemes").strings() {
				out = append(out, manifestEntry{Kind: "url_scheme", Name: scheme, Line: t.Line})
			}
		}
	}
	if modes := root.get("UIBackgroundModes"); modes != nil {
		for _, m := range modes.strings() {
			out = append(out, manifestEntry{Kind: "background_mode", Name: m, Line: modes.Line})
		}
	}
	for _, k := range root.Keys {
		if strings.HasPrefix(k, "NS") && strings.HasSuffix(k, "UsageDescription") {
			v := root.Dict[k]
			out = append(out, manifestEntry{Kind: "usage_description", Name: k, Value: v.Str, Line: v.Line})
		}
	}
	if ats := root.get("NSAppTransportSecurity"); ats != nil {
		for _, k := range []string{"NSAllowsArbitraryLoads", "NSAllowsArbitraryLoadsInWebContent", "NSAllowsLocalNetworking"} {
			if v := ats.get(k); v != nil && v.Str == "true" {
				out = append(out, manifestEntry{Kind: "ats_exception", Name: k, Value: "true", Line: v.Line})
			}
		}
		if domains := ats.get("NSExceptionDomains"); domains != nil {
			for _, d := range domains.Keys {
				out = append(out, manifestEntry{Kind: "ats_exception", Name: "NSExceptionDomains", Value: d, Line: domains.Dict[d].Line})
			}
		}
	}
	if scenes := root.get("UIApplicationSceneManifest"); scenes != nil {
		if cfgs := scenes.get("UISceneConfigurations"); cfgs != nil {
			for _, role := range cfgs.Keys {
				for _, c := range cfgs.Dict[role].Array {
					if d := c.get("UISceneDelegateClassName"); d != nil && d.Str != "" {
						out = append(out, manifestEntry{Kind: "scene_delegate", Name: d.Str, Line: d.Line})
					}
				}
			}
		}
	}
	return out
}

// entitlementEntries reads an .entitlements file: one row per entitlement,
// and one deep link per associated domain.
func entitlementEntries(root *plistValue) []manifestEntry {
	if root == nil || root.Kind != "dict" {
		return nil
	}
	var out []manifestEntry
	keys := append([]string{}, root.Keys...)
	sort.Strings(keys)
	for _, k := range keys {
		v := root.Dict[k]
		values := v.strings()
		out = append(out, manifestEntry{Kind: "entitlement", Name: k, Value: strings.Join(values, ", "), Line: v.Line})
		if k == "com.apple.developer.associated-domains" {
			for _, d := range values {
				if strings.HasPrefix(d, "applinks:") {
					out = append(out, manifestEntry{Kind: "deep_link", Name: "universal link", Value: "https://" + strings.TrimPrefix(d, "applinks:"), Line: v.Line})
				}
			}
		}
	}
	return out
}
