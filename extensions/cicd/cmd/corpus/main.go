// Command corpus reads every pipeline under the folders it is given with the
// cicd extension's readers, and prints how well each CI system was read. It
// is a measuring tool for the readers, not part of archstats.
//
//	go run ./extensions/cicd/cmd/corpus [-v] [-jobs] dir…
package main

import (
	"flag"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/archstats/archstats/engine"
	"github.com/archstats/archstats/extensions/cicd"
)

var skipDirs = map[string]bool{"node_modules": true, ".git": true, "vendor": true, "target": true, "build": true, "dist": true, ".gradle": true, ".idea": true, ".venv": true, "venv": true, "__pycache__": true, ".claude": true}

// classifier judges files by every pack's conventions, as a scan would.
var classifier = engine.Classifier()

func main() {
	verbose := flag.Bool("v", false, "list every partial and unread pipeline")
	jobs := flag.Bool("jobs", false, "print each pipeline's jobs and steps")
	only := flag.String("system", "", "only this system")
	flags := flag.Bool("flags", false, "print every hygiene flag with the line it points at, tab-separated")
	flag.Parse()
	type stat struct{ files, full, partial, none, jobs, steps int }
	stats := map[string]*stat{}
	reasons := map[string]map[string]int{}
	for _, root := range flag.Args() {
		in := &cicd.Input{Tools: engine.BuildTools(), Files: map[string][]byte{}, Kinds: map[string]string{}, Roles: map[string]string{}, Components: map[string]string{}}
		_ = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return nil
			}
			if d.IsDir() {
				if path != root && (skipDirs[d.Name()] || strings.HasPrefix(d.Name(), ".") && d.Name() != ".github" && d.Name() != ".gitlab" && d.Name() != ".circleci" && d.Name() != ".azure-pipelines" && d.Name() != ".tekton" && d.Name() != ".buildkite") {
					return filepath.SkipDir
				}
				return nil
			}
			rel, _ := filepath.Rel(root, path)
			rel = filepath.ToSlash(rel)
			info, err := d.Info()
			if err != nil || info.Size() > 2<<20 {
				return nil
			}
			content, err := os.ReadFile(path)
			if err != nil {
				return nil
			}
			in.AllFiles = append(in.AllFiles, rel)
			in.Roles[rel] = classifier.Role(rel, false, false, classifier.SystemKind(rel, content))
			if kind, keep := cicd.KindOf(rel, content, engine.BuildTools()); keep {
				in.Files[rel] = content
				if kind != "" {
					in.Kinds[rel] = kind
				}
			}
			return nil
		})
		sort.Strings(in.AllFiles)
		in.RepoOf = func(p string) string {
			dir := filepath.Dir(p)
			for dir != "." && dir != "/" {
				if _, err := os.Stat(filepath.Join(root, dir, ".git")); err == nil {
					return filepath.ToSlash(dir)
				}
				dir = filepath.Dir(dir)
			}
			return filepath.Base(root)
		}
		in.Remotes = cicd.Remotes(root, in.AllFiles, in.RepoOf)
		m := cicd.Build(in)
		if *flags {
			for _, f := range m.Flags {
				text := ""
				if b, err := os.ReadFile(filepath.Join(root, f.File)); err == nil {
					if lines := strings.Split(string(b), "\n"); f.Line >= 1 && f.Line <= len(lines) {
						text = strings.TrimSpace(lines[f.Line-1])
					}
				}
				fmt.Printf("%s\t%s\t%s:%d\t%s\t%s\t%.160s\n", f.Rule, root, f.File, f.Line, f.Subject, f.Detail, text)
			}
		}
		unread := map[string][]*cicd.UnreadRow{}
		for _, u := range m.Unread {
			unread[u.Pipeline] = append(unread[u.Pipeline], u)
		}
		for _, p := range m.Pipelines {
			if *only != "" && p.System != *only {
				continue
			}
			s := stats[p.System]
			if s == nil {
				s = &stat{}
				stats[p.System] = s
				reasons[p.System] = map[string]int{}
			}
			s.files++
			s.jobs += p.Jobs
			s.steps += p.Steps
			switch p.Read {
			case "full":
				s.full++
			case "partial":
				s.partial++
			default:
				s.none++
			}
			for _, u := range unread[p.ID] {
				reasons[p.System][u.Reason]++
			}
			if *verbose && p.Read != "full" {
				var rs []string
				for _, u := range unread[p.ID] {
					rs = append(rs, fmt.Sprintf("%d-%d %s", u.FromLine, u.ToLine, u.Reason))
				}
				fmt.Printf("  %-8s %s/%s  %s\n", p.Read, root, p.File, strings.Join(rs, "; "))
			}
			if *jobs {
				fmt.Printf("\n== %s/%s [%s %s, %s] jobs=%d steps=%d\n", root, p.File, p.System, p.Kind, p.Read, p.Jobs, p.Steps)
				for _, t := range m.Triggers {
					if t.Pipeline == p.ID {
						fmt.Printf("   on %s %s\n", t.Event, t.Filter)
					}
				}
				for _, j := range m.Jobs {
					if j.Pipeline != p.ID {
						continue
					}
					fmt.Printf("   [%d] %s  grp=%q needs=[%s] runner=%q env=%q gate=%q matrix=%d cond=%q\n", j.Depth, j.ID, j.Group, j.Needs, j.Runner, j.Environment, j.Gate, j.Matrix, j.Condition)
					for _, s := range m.Steps {
						if s.Pipeline == p.ID && s.Job == j.ID {
							fmt.Printf("        %2d %-8s %-8s %-28q %s\n", s.Index, s.Verb, s.Tool, s.Target, s.Text)
						}
					}
				}
				for _, h := range m.Handoffs {
					if h.Pipeline == p.ID {
						fmt.Printf("   → %s %s @%s (%s) via=%s local=%s\n", h.Kind, h.Target, h.Ref, h.Pinned, h.Via, h.LocalPipeline)
					}
				}
				for _, o := range m.Outputs {
					if o.Pipeline == p.ID {
						fmt.Printf("   ⇒ %s %s %s env=%q artifact=%s ctx=%q\n", o.Kind, o.Platform, o.Target, o.Environment, o.Artifact, o.Context)
					}
				}
				for _, f := range m.Flags {
					if f.File == p.File {
						fmt.Printf("   ! %s %s (line %d)\n", f.Rule, f.Detail, f.Line)
					}
				}
			}
		}
	}
	var systems []string
	for s := range stats {
		systems = append(systems, s)
	}
	sort.Strings(systems)
	fmt.Printf("\n%-20s %6s %6s %8s %6s %7s %6s %7s  %s\n", "system", "files", "full", "partial", "none", "full%", "jobs", "steps", "unread reasons")
	for _, s := range systems {
		st := stats[s]
		pct := 0.0
		if st.files > 0 {
			pct = 100 * float64(st.full) / float64(st.files)
		}
		var rs []string
		for r, n := range reasons[s] {
			rs = append(rs, fmt.Sprintf("%s=%d", r, n))
		}
		sort.Strings(rs)
		fmt.Printf("%-20s %6d %6d %8d %6d %6.1f%% %6d %7d  %s\n", s, st.files, st.full, st.partial, st.none, pct, st.jobs, st.steps, strings.Join(rs, " "))
	}
}
