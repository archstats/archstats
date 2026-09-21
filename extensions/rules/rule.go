// Package rules checks the one or two statements that decide whether an
// architecture is still intact.
//
// Every ecosystem surveyed has one. Sylius is 60 composer packages whose
// whole design rests on `Sylius\Component\*` -- the framework-agnostic
// domain -- never depending on `Sylius\Bundle\*`, the framework glue.
// nopCommerce is 40 .csproj files in which plugins may depend on core and
// core must never depend on a plugin. Go has `internal/`, which the compiler
// itself enforces and which is therefore stronger evidence than any ordinary
// edge.
//
// None of those are metrics. A number goes up or down and an architect has
// to decide what it means; a rule is either kept or broken, and the answer
// names the file and line where it broke. That is what people open the tool
// for, and it is cheap, because the module map and the typed edges are
// already there.
//
// Rules are declared in yaml beside the metric definitions, for the same
// reason: they are vocabulary the user reads, so they document themselves
// and reach docs/ through the same path.
package rules

import (
	"fmt"
	"regexp"
)

// A Rule is a statement about which modules may depend on which.
type Rule struct {
	Id               string `yaml:"id"`
	Name             string `yaml:"name"`
	ShortDescription string `yaml:"short_description"`
	LongDescription  string `yaml:"long_description"`
	Category         string `yaml:"category"`

	// AppliesWhen keeps a rule quiet in a codebase it was not written for.
	// Without it, "core must not depend on a plugin" would report on every
	// project that happens to have a module with Plugin in its name.
	AppliesWhen Selector `yaml:"applies_when"`

	// An edge from a module matching From to a module matching To is a
	// violation.
	From Selector `yaml:"from"`
	To   Selector `yaml:"to"`
}

// A Selector picks modules. Two patterns rather than one because Go's regexp
// has no lookahead and every rule here needs "matches this but not that".
//
// DirMatches exists because a module's name does not always carry the
// distinction a rule is about. Sylius's framework-agnostic packages are
// called `sylius/order` and `sylius/promotion` -- nothing in those names says
// "component". The split its whole design rests on is declared by where the
// package sits: `src/Sylius/Component/Order` against
// `src/Sylius/Bundle/OrderBundle`. Matching the name alone, the rule could
// never fire, which is the one thing a rule must never do quietly.
//
// Matching a path segment rather than the module's own directory was tried
// and reverted: `(?i)/component/` also matches
// `Bundle/AdminBundle/Twig/Component/`, a Twig UI component folder with
// nothing to do with Sylius's domain packages, and reported sixty
// violations that are not violations. A rule reads what a project declares,
// not what a directory happens to be called.
type Selector struct {
	Matches    string `yaml:"matches"`
	NotMatches string `yaml:"not_matches"`
	DirMatches string `yaml:"dir_matches"`
}

type compiledSelector struct {
	matches    *regexp.Regexp
	notMatches *regexp.Regexp
	dirMatches *regexp.Regexp
}

func (s Selector) compile() (compiledSelector, error) {
	var c compiledSelector
	var err error
	if s.Matches != "" {
		if c.matches, err = regexp.Compile(s.Matches); err != nil {
			return c, fmt.Errorf("matches %q: %w", s.Matches, err)
		}
	}
	if s.NotMatches != "" {
		if c.notMatches, err = regexp.Compile(s.NotMatches); err != nil {
			return c, fmt.Errorf("not_matches %q: %w", s.NotMatches, err)
		}
	}
	if s.DirMatches != "" {
		if c.dirMatches, err = regexp.Compile(s.DirMatches); err != nil {
			return c, fmt.Errorf("dir_matches %q: %w", s.DirMatches, err)
		}
	}
	return c, nil
}

// selects reports whether a module is in this selector's set. An empty
// selector selects nothing, so a rule that forgot to say what it is about
// reports no violations rather than reporting every edge.
func (c compiledSelector) selects(name, dir string) bool {
	if c.matches == nil && c.dirMatches == nil {
		return false
	}
	if c.matches != nil && !c.matches.MatchString(name) {
		return false
	}
	if c.notMatches != nil && c.notMatches.MatchString(name) {
		return false
	}
	if c.dirMatches != nil && !c.dirMatches.MatchString(dir) {
		return false
	}
	return true
}

// any reports whether anything in the list is selected. Used by AppliesWhen,
// where an empty selector means "always".
func (c compiledSelector) any(modules []NamedDir) bool {
	if c.matches == nil && c.dirMatches == nil {
		return true
	}
	for _, m := range modules {
		if c.selects(m.Name, m.Dir) {
			return true
		}
	}
	return false
}

// NamedDir is the part of a module a selector reads.
type NamedDir struct {
	Name string
	Dir  string
}

type compiledRule struct {
	rule *Rule
	when compiledSelector
	from compiledSelector
	to   compiledSelector
}

func compile(r *Rule) (*compiledRule, error) {
	when, err := r.AppliesWhen.compile()
	if err != nil {
		return nil, fmt.Errorf("rule %s applies_when: %w", r.Id, err)
	}
	from, err := r.From.compile()
	if err != nil {
		return nil, fmt.Errorf("rule %s from: %w", r.Id, err)
	}
	to, err := r.To.compile()
	if err != nil {
		return nil, fmt.Errorf("rule %s to: %w", r.Id, err)
	}
	return &compiledRule{rule: r, when: when, from: from, to: to}, nil
}

// What a rule has to say about a codebase.
//
// A rule that reports nothing is ambiguous, and the ambiguity matters: it may
// have been checked and held, or it may never have applied. "Core must not
// depend on a plugin" says nothing about a project with no plugins, and
// showing that project a clean bill of health claims something nobody
// checked. Every rule therefore reports its own status, whether or not it
// found anything.
const (
	// This edge breaks the rule.
	StatusViolation = "violation"
	// The rule applied to this codebase and found nothing.
	StatusOk = "ok"
	// The rule is about an ecosystem or a layout this codebase does not
	// have, so it has no opinion.
	StatusNotApplicable = "not_applicable"
)

// A Finding is one rule's verdict: an edge that breaks it, or the rule
// reporting that it held or did not apply.
type Finding struct {
	Rule   string
	Status string
	From   string
	To     string
	Kind   string
	File   string
	Line   int
}
