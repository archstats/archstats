package navigation

import (
	"path"
	"regexp"
	"strings"
)

// The written intent: decision records, READMEs, architecture notes and
// guides, each linked to the code it is about. "Why is it like this?" has an
// answer more often than a model guesses, and it is usually a file nobody
// opened.

type doc struct {
	File   string
	Kind   string
	Title  string
	Status string
	Date   string
	Words  int
	// Mentions are the code spans and paths the text names, for linking.
	Spans []string
	Paths []string
}

var (
	adrDir      = regexp.MustCompile(`(?i)(^|/)(adrs?|decisions|decision[-_]records?|architecture[-_]decisions?|architectural[-_]decisions?)/`)
	adrFile     = regexp.MustCompile(`(?i)^(adr[-_ ]?)?\d{3,4}[-_ ]`)
	docsDir     = regexp.MustCompile(`(?i)(^|/)(docs?|documentation|wiki|design|architecture|handbook|guides?)/`)
	mdHeading   = regexp.MustCompile(`(?m)^#\s+(.+?)\s*#*\s*$`)
	adocTitle   = regexp.MustCompile(`(?m)^=\s+(.+?)\s*$`)
	rstTitle    = regexp.MustCompile(`(?m)^([^\n]{3,})\n(={3,}|-{3,}|\*{3,})\s*$`)
	statusLine  = regexp.MustCompile(`(?im)^\s*(?:\*\*|__)?status(?:\*\*|__)?\s*[:=]\s*(?:\*\*|__)?\s*([A-Za-z][\w -]*)`)
	statusHead  = regexp.MustCompile(`(?im)^#{1,4}\s*status\s*$\s*^\s*(?:[-*]\s*)?(?:\*\*|__)?([A-Za-z][\w -]*)`)
	dateLine    = regexp.MustCompile(`(?i)\b(?:date|decided|accepted)\b\s*[:=]?\s*(?:\*\*|__)?\s*(\d{4}-\d{2}-\d{2})`)
	anyDate     = regexp.MustCompile(`\b(\d{4}-\d{2}-\d{2})\b`)
	codeSpan    = regexp.MustCompile("`([^`\\n]{2,120})`")
	pathMention = regexp.MustCompile(`(?:^|[\s(\[<"'])((?:[\w.@-]+/)+[\w.@-]+)`)
	adrHeadings = regexp.MustCompile(`(?im)^#{1,4}\s*(context|decision|consequences)\b`)
)

func (s *source) document() *doc {
	if s.lang != langMarkdown {
		return nil
	}
	base := path.Base(s.path)
	stem := strings.TrimSuffix(base, path.Ext(base))
	upper := strings.ToUpper(stem)
	d := &doc{File: s.path}
	switch {
	case strings.HasPrefix(upper, "README"):
		d.Kind = "readme"
	case strings.HasPrefix(upper, "CONTRIBUTING"):
		d.Kind = "contributing"
	case strings.HasPrefix(upper, "CHANGELOG") || upper == "CHANGES" || upper == "HISTORY" || upper == "RELEASE_NOTES" || upper == "RELEASES":
		d.Kind = "changelog"
	case strings.HasPrefix(upper, "LICENSE") || strings.HasPrefix(upper, "LICENCE") || upper == "NOTICE" || upper == "CODE_OF_CONDUCT" || upper == "SECURITY" || upper == "AUTHORS":
		return nil
	case adrDir.MatchString(s.path) && adrFile.MatchString(base), adrFile.MatchString(base) && len(adrHeadings.FindAllString(s.raw, -1)) >= 2:
		d.Kind = "adr"
	case strings.Contains(strings.ToLower(s.path), "architecture") || strings.Contains(strings.ToLower(stem), "design") || strings.Contains(strings.ToLower(stem), "overview"):
		d.Kind = "architecture"
	case docsDir.MatchString(s.path):
		d.Kind = "guide"
	default:
		d.Kind = "note"
	}
	text := s.raw
	head := text[:min(len(text), 4000)]
	switch {
	case mdHeading.MatchString(head):
		d.Title = mdHeading.FindStringSubmatch(head)[1]
	case adocTitle.MatchString(head):
		d.Title = adocTitle.FindStringSubmatch(head)[1]
	case rstTitle.MatchString(head):
		d.Title = rstTitle.FindStringSubmatch(head)[1]
	default:
		d.Title = stem
	}
	d.Title = strings.TrimSpace(strings.Trim(d.Title, "*_` "))
	if d.Kind == "adr" {
		if m := statusHead.FindStringSubmatch(head); m != nil {
			d.Status = firstWord(m[1])
		} else if m := statusLine.FindStringSubmatch(head); m != nil {
			d.Status = firstWord(m[1])
		}
	}
	if m := dateLine.FindStringSubmatch(head); m != nil {
		d.Date = m[1]
	} else if d.Kind == "adr" {
		if m := anyDate.FindStringSubmatch(head[:min(len(head), 1500)]); m != nil {
			d.Date = m[1]
		}
	}
	d.Words = len(strings.Fields(text))
	seen := map[string]bool{}
	for _, m := range codeSpan.FindAllStringSubmatch(text, 2000) {
		v := strings.TrimSpace(m[1])
		if !seen[v] {
			seen[v] = true
			d.Spans = append(d.Spans, v)
		}
	}
	for _, m := range pathMention.FindAllStringSubmatch(text, 2000) {
		v := strings.Trim(m[1], ".,;:)")
		if strings.Contains(v, "://") || strings.HasPrefix(v, "www.") || seen["p:"+v] {
			continue
		}
		seen["p:"+v] = true
		d.Paths = append(d.Paths, v)
	}
	return d
}

func firstWord(s string) string {
	f := strings.Fields(s)
	if len(f) == 0 {
		return ""
	}
	w := strings.ToLower(strings.Trim(f[0], "*_.,;:"))
	if w == "" {
		return ""
	}
	return strings.ToUpper(w[:1]) + w[1:]
}
