package navigation

import (
	"fmt"
	"regexp"
	"strings"
)

// The NLGov REST API Design Rules (Logius, "ADR") are what Dutch government
// APIs are held to. Most are about behaviour and need the running API; the
// ones below can be read from a contract or a route as written. Rule names
// follow the Logius Spectral ruleset where one exists. A finding is a fact
// with its file and line, never a verdict on conformance.

type finding struct {
	Rule, Severity, Subject string
	Entry                   int
	Spec, Method, Path      string
	Detail                  string
	File                    string
	Line                    int
}

var (
	majorVersion = regexp.MustCompile(`(?i)^v\d+$`)
	kebabSegment = regexp.MustCompile(`^[a-z0-9]+(?:-[a-z0-9]+)*(?:\.[a-z0-9]+)?$`)
	semver       = regexp.MustCompile(`^\d+\.\d+\.\d+(?:-[0-9A-Za-z.-]+)?(?:\+[0-9A-Za-z.-]+)?$`)
)

// specFindings are the rules a provided contract can be checked against.
func specFindings(s *spec) []finding {
	var out []finding
	add := func(rule, severity, subject, method, p, detail string, line int) {
		out = append(out, finding{Rule: rule, Severity: severity, Subject: subject, Spec: s.File, Method: method, Path: p, Detail: detail, File: s.File, Line: line})
	}
	if s.Format == "swagger2" {
		add("openapi3", "must", "spec", "", "", "Swagger "+s.Version+"; the rules ask for OpenAPI 3", s.Line)
	}
	if s.APIVer != "" && !semver.MatchString(s.APIVer) {
		add("semver", "must", "spec", "", "", fmt.Sprintf("info.version %q is not MAJOR.MINOR.PATCH", s.APIVer), s.InfoLine)
	}
	if !s.Contact {
		add("info-contact-fields-exist", "should", "spec", "", "", "info has no contact", s.InfoLine)
	}
	versioned := false
	for _, sv := range s.Servers {
		if strings.HasPrefix(strings.ToLower(sv.URL), "http://") && !isLocalHost(hostOf(sv.URL)) {
			add("servers-use-https", "must", "spec", "", "", "server "+sv.URL+" is plain http", sv.Line)
		}
		if hasMajorVersion(basePath(sv.URL)) {
			versioned = true
		}
	}
	for _, op := range s.Ops {
		if hasMajorVersion(op.Path) {
			versioned = true
		}
		out = append(out, pathFindings(op.Method, op.Path, func(rule, severity, detail string) {
			add(rule, severity, "operation", op.Method, op.Path, detail, op.Line)
		})...)
		if op.Method == "TRACE" {
			add("http-methods", "must", "operation", op.Method, op.Path, "TRACE is not one of the methods the rules allow", op.Line)
		}
	}
	if !versioned && len(s.Ops) > 0 {
		add("include-major-version-in-uri", "must", "spec", "", "", "no major version (/v1) in the server URL or the paths", s.Line)
	}
	if n := len(s.Ops); n > 0 && s.VersionHeaders < n {
		add("missing-version-header", "must", "spec", "", "", fmt.Sprintf("%d of %d operations declare no API-Version response header", n-s.VersionHeaders, n), s.Line)
	}
	if s.ErrorBodies > s.ProblemBodies {
		add("use-problem-schema", "should", "spec", "", "", fmt.Sprintf("%d of %d error responses are not application/problem+json", s.ErrorBodies-s.ProblemBodies, s.ErrorBodies), s.Line)
	}
	return out
}

// pathFindings are the rules a single path breaks, written once for both a
// contract's operations and the code's routes.
func pathFindings(method, p string, add func(rule, severity, detail string)) []finding {
	if len(p) > 1 && strings.HasSuffix(p, "/") {
		add("paths-no-trailing-slash", "must", "path ends with a slash")
	}
	for _, seg := range strings.Split(strings.Trim(pathTemplate(p), "/"), "/") {
		if seg == "" || seg == "{}" || strings.Contains(seg, "{}") || majorVersion.MatchString(seg) {
			continue
		}
		if !kebabSegment.MatchString(seg) {
			add("paths-kebab-case", "should", fmt.Sprintf("segment %q is not lower-case kebab-case", seg))
			break
		}
	}
	return nil
}

func hasMajorVersion(p string) bool {
	for _, seg := range strings.Split(p, "/") {
		if majorVersion.MatchString(seg) {
			return true
		}
	}
	return false
}

// routeFindings are the rules the code's own API routes break. Only routes
// that look like an API are checked: under a provided contract, or below an
// /api segment; a server-rendered page has no such rules.
func routeFindings(rows []entryRow, inContract map[int]bool, apiComponents map[string]bool) []finding {
	var out []finding
	unversioned := map[string]*entryRow{}
	for i := range rows {
		r := &rows[i]
		if r.Kind != kindHTTP {
			continue
		}
		api := inContract[r.ID] || apiComponents[r.Component] || strings.HasPrefix(strings.ToLower(r.Template), "/api") || strings.Contains(strings.ToLower(r.Template), "/api/")
		if !api {
			continue
		}
		pathFindings(r.Method, r.Path, func(rule, severity, detail string) {
			out = append(out, finding{Rule: rule, Severity: severity, Subject: "route", Entry: r.ID, Method: r.Method, Path: r.Path, Detail: detail, File: r.File, Line: r.Line})
		})
		if !hasMajorVersion(r.Template) {
			if _, ok := unversioned[r.Component]; !ok {
				unversioned[r.Component] = r
			}
		}
	}
	for comp, r := range unversioned {
		n := 0
		for i := range rows {
			if rows[i].Component == comp && rows[i].Kind == kindHTTP && !hasMajorVersion(rows[i].Template) {
				n++
			}
		}
		out = append(out, finding{Rule: "include-major-version-in-uri", Severity: "must", Subject: "route", Entry: r.ID, Method: r.Method, Path: r.Path,
			Detail: fmt.Sprintf("%d routes in %s have no major version (/v1) in their path", n, comp), File: r.File, Line: r.Line})
	}
	return out
}
