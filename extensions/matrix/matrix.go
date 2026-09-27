package matrix

import (
	"github.com/archstats/archstats/cmd/config"
	"github.com/archstats/archstats/core"
	"github.com/archstats/archstats/core/unit"
	"github.com/samber/lo"
	"github.com/spf13/cobra"
	"path/filepath"
	"sort"
	"strings"
)

const FileMatrix = "file-matrix"

func Extension() core.Extension {
	return &extension{}
}

func CLIExtension() *config.CLIConfiguredExtension {
	return &config.CLIConfiguredExtension{
		Name:        "matrix",
		Description: "Pairwise co-change, name similarity and path distance",
		Arguments: config.Arguments{
			FileMatrix: {
				Default:     false,
				Description: "Also write file_matrix, every pair of files that changed together (large; git_commits and git_file_shared_commits hold the same co-change)",
				Required:    false,
				Type:        config.Bool,
			},
		},
		Initializer: func(command *cobra.Command) (core.Extension, error) {
			fileMatrix, err := command.Flags().GetBool(FileMatrix)
			if err != nil {
				return nil, err
			}
			return &extension{fileMatrix: fileMatrix}, nil
		},
	}
}

type extension struct {
	// fileMatrix writes file_matrix. It is opt-in: the table was the largest
	// in most snapshots (every co-changed pair of files, both ways round),
	// and building it held a path distance from every file to every file.
	fileMatrix bool
}

func (v *extension) Init(settings core.Analyzer) error {
	settings.RegisterView(&core.ViewFactory{
		Name:           "component_matrix",
		CreateViewFunc: ComponentMatrixView,
	})

	if v.fileMatrix {
		settings.RegisterView(&core.ViewFactory{
			Name:           "file_matrix",
			CreateViewFunc: FileMatrixView,
		})
	}

	return nil
}

// Tokenizer & Jaccard logic

func tokenize(s string) []string {
	tokens := make([]string, 0)
	var current strings.Builder
	for i, r := range s {
		if i > 0 {
			prev := rune(s[i-1])

			// Transitions:
			// 1. Lowercase to Uppercase (e.g., eC -> e, C)
			isCapTransition := isLower(prev) && isUpper(r)

			// 2. Acronym termination (e.g., XMLP -> XML, P)
			isUpperTransition := false
			if isUpper(prev) && isUpper(r) {
				if i+1 < len(s) && isLower(rune(s[i+1])) {
					isUpperTransition = true
				}
			}

			// 3. Letter to Digit (e.g., r2 -> r, 2)
			isLetterToDigit := isLetter(prev) && isDigit(r)

			// 4. Digit to Letter (e.g., 2c -> 2, c)
			isDigitToLetter := isDigit(prev) && isLetter(r)

			if isCapTransition || isUpperTransition || isLetterToDigit || isDigitToLetter {
				if current.Len() > 0 {
					tokens = append(tokens, strings.ToLower(current.String()))
					current.Reset()
				}
			}
		}
		if isAlphanumeric(r) {
			current.WriteRune(r)
		} else {
			if current.Len() > 0 {
				tokens = append(tokens, strings.ToLower(current.String()))
				current.Reset()
			}
		}
	}
	if current.Len() > 0 {
		tokens = append(tokens, strings.ToLower(current.String()))
	}
	return tokens
}

func isLower(r rune) bool {
	return r >= 'a' && r <= 'z'
}
func isUpper(r rune) bool {
	return r >= 'A' && r <= 'Z'
}
func isDigit(r rune) bool {
	return r >= '0' && r <= '9'
}
func isLetter(r rune) bool {
	return isLower(r) || isUpper(r)
}
func isAlphanumeric(r rune) bool {
	return isLower(r) || isUpper(r) || isDigit(r)
}

var suffixFilter = map[string]bool{
	"controller":     true,
	"restcontroller": true,
	"service":        true,
	"impl":           true,
	"repository":     true,
	"entity":         true,
	"dto":            true,
	"model":          true,
	"config":         true,
	"facade":         true,
	"helper":         true,
	"util":           true,
	"test":           true,
}

func getCoreTokens(s string) []string {
	tokens := tokenize(s)
	filtered := make([]string, 0)
	for _, t := range tokens {
		if !suffixFilter[t] {
			filtered = append(filtered, t)
		}
	}
	return filtered
}

func jaccardSimilarity(a, b []string) float64 {
	if len(a) == 0 || len(b) == 0 {
		return 0.0
	}
	setA := make(map[string]bool)
	for _, t := range a {
		setA[t] = true
	}
	setB := make(map[string]bool)
	for _, t := range b {
		setB[t] = true
	}

	intersection := 0
	for t := range setA {
		if setB[t] {
			intersection++
		}
	}
	union := len(setA) + len(setB) - intersection
	if union == 0 {
		return 0.0
	}
	return float64(intersection) / float64(union)
}

func jaccardSimilarityPrecomputed(setA, setB map[string]bool) float64 {
	if len(setA) == 0 || len(setB) == 0 {
		return 0.0
	}
	intersection := 0
	for t := range setA {
		if setB[t] {
			intersection++
		}
	}
	union := len(setA) + len(setB) - intersection
	if union == 0 {
		return 0.0
	}
	return float64(intersection) / float64(union)
}

func getFileBaseName(path string) string {
	base := filepath.Base(path)
	ext := filepath.Ext(base)
	return strings.TrimSuffix(base, ext)
}

// Git commits helper

func getGitCommitsMapping(results *core.Results) (map[string]map[string]bool, map[string]map[string]bool) {
	fileToCommits := make(map[string]map[string]bool)
	compToCommits := make(map[string]map[string]bool)

	gitCommitsView, err := results.RenderView("git_commits")
	if err == nil {
		// First pass: count files per commit
		commitFileCount := make(map[string]int)
		for _, row := range gitCommitsView.Rows {
			hashVal, existsHash := row.Data["commit_hash"]
			if existsHash {
				hashStr, ok := hashVal.(string)
				if ok && hashStr != "" {
					commitFileCount[hashStr]++
				}
			}
		}

		// Second pass: leave sweeping commits out. The threshold is the git
		// extension's default for co-change (--git-max-changes-per-commit),
		// so "co-change" means the same thing in this matrix as it does for
		// components, cycles and the git coupling views. It used to be 50
		// here and 100 there.
		for _, row := range gitCommitsView.Rows {
			fileVal, existsFile := row.Data["file"]
			hashVal, existsHash := row.Data["commit_hash"]
			compVal, existsComp := row.Data["component"]

			var hashStr string
			if existsHash {
				hashStr, _ = hashVal.(string)
			}
			if hashStr == "" || commitFileCount[hashStr] > sweepingCommitFiles {
				continue
			}

			if existsFile {
				if fileStr, ok := fileVal.(string); ok && fileStr != "" {
					if _, exists := fileToCommits[fileStr]; !exists {
						fileToCommits[fileStr] = make(map[string]bool)
					}
					fileToCommits[fileStr][hashStr] = true

					var compStr string
					if existsComp {
						compStr, _ = compVal.(string)
					}
					if compStr == "" {
						compStr = results.FileToComponent[fileStr]
					}
					if compStr != "" {
						if _, exists := compToCommits[compStr]; !exists {
							compToCommits[compStr] = make(map[string]bool)
						}
						compToCommits[compStr][hashStr] = true
					}
				}
			}
		}
	}
	return fileToCommits, compToCommits
}

// Graph path distance calculation (BFS)

func computePathDistances(nodes []string, adj map[string][]string) map[string]map[string]int {
	distances := make(map[string]map[string]int)
	for _, start := range nodes {
		startDist := make(map[string]int)
		queue := []string{start}
		startDist[start] = 0

		for len(queue) > 0 {
			curr := queue[0]
			queue = queue[1:]

			currDist := startDist[curr]
			for _, next := range adj[curr] {
				if _, visited := startDist[next]; !visited {
					startDist[next] = currDist + 1
					queue = append(queue, next)
				}
			}
		}
		distances[start] = startDist
	}
	return distances
}

func FileMatrixView(results *core.Results) *core.View {
	var files []string
	for f := range results.StatRecordsByFile {
		files = append(files, f)
	}
	sort.Strings(files)

	// Count frequencies of all core tokens
	tokenFrequencies := make(map[string]int)
	for _, f := range files {
		baseName := getFileBaseName(f)
		tokens := getCoreTokens(baseName)
		for _, t := range tokens {
			tokenFrequencies[t]++
		}
	}

	// Filter out tokens that appear in more than maxFreq files
	// maxFreq defaults to 2% of total files, capped between 15 and 50
	maxFreq := len(files) / 50
	if maxFreq < 15 {
		maxFreq = 15
	}
	if maxFreq > 50 {
		maxFreq = 50
	}

	// Precompute token sets for all files, excluding high-frequency terms
	fileTokenSets := make([]map[string]bool, len(files))
	for i, f := range files {
		baseName := getFileBaseName(f)
		tokens := getCoreTokens(baseName)
		set := make(map[string]bool)
		for _, t := range tokens {
			if tokenFrequencies[t] <= maxFreq {
				set[t] = true
			}
		}
		fileTokenSets[i] = set
	}

	// Git commits
	fileToCommits, _ := getGitCommitsMapping(results)
	// Pre-map commits to index list for O(1) loop lookup
	fileCommitsList := make([]map[string]bool, len(files))
	for i, f := range files {
		fileCommitsList[i] = fileToCommits[f]
	}

	// Dependency graph: one unit using another, lifted to the files they are
	// declared in. It used to be guessed from import strings -- an import
	// matched any directory ending in its name, picked in map order, and
	// then counted as a dependency on every file in that directory -- so
	// the distances changed from one scan to the next and mostly measured
	// nothing. Every language now records resolved unit references.
	fileToDeps := make(map[string][]string)
	for _, c := range unit.Connections(results.Units) {
		from, to := results.UnitByID[c.From], results.UnitByID[c.To]
		if from == nil || to == nil || len(from.Files) == 0 || len(to.Files) == 0 {
			continue
		}
		fromFile, toFile := from.Files[0], to.Files[0]
		if fromFile != toFile {
			fileToDeps[fromFile] = append(fileToDeps[fromFile], toFile)
		}
	}

	// Deduplicate
	for f, deps := range fileToDeps {
		fileToDeps[f] = lo.Uniq(deps)
	}

	// Path distances
	distances := computePathDistances(files, fileToDeps)

	// Build rows
	var rows []*core.Row
	for i, f1 := range files {
		f1Distances := distances[f1]
		f1Commits := fileCommitsList[i]
		f1TokenSet := fileTokenSets[i]

		for j, f2 := range files {
			if i == j {
				continue
			}

			// 1. Git co-commits
			sharedCommits := 0
			f2Commits := fileCommitsList[j]
			if len(f1Commits) > 0 && len(f2Commits) > 0 {
				c1, c2 := f1Commits, f2Commits
				if len(c1) > len(c2) {
					c1, c2 = c2, c1
				}
				for c := range c1 {
					if c2[c] {
						sharedCommits++
					}
				}
			}

			// 2. Path distance
			pathDist := -1
			if d, found := f1Distances[f2]; found {
				pathDist = d
			}

			// Only pairs that changed together: the rest (nearby files that
			// never co-changed) were two thirds of the rows, the largest table
			// of a Java snapshot, and nothing reads them.
			if sharedCommits > 0 {
				lingSim := jaccardSimilarityPrecomputed(f1TokenSet, fileTokenSets[j])
				rows = append(rows, &core.Row{
					Data: map[string]interface{}{
						"from":                  f1,
						"to":                    f2,
						"linguistic_similarity": lingSim,
						"git_co_changes":        sharedCommits,
						"path_distance":         pathDist,
					},
				})
			}
		}
	}

	return &core.View{
		Name: "file_matrix",
		Columns: []*core.Column{
			core.StringColumn("from"),
			core.StringColumn("to"),
			core.FloatColumn("linguistic_similarity"),
			core.IntColumn("git_co_changes"),
			core.IntColumn("path_distance"),
		},
		Rows: rows,
	}
}

func ComponentMatrixView(results *core.Results) *core.View {
	var components []string
	for c := range results.ComponentToFiles {
		components = append(components, c)
	}
	sort.Strings(components)

	// Count frequencies of all component tokens
	tokenFrequencies := make(map[string]int)
	for _, c := range components {
		lastSeg := c
		if idx := strings.LastIndex(c, "/"); idx != -1 {
			lastSeg = c[idx+1:]
		}
		tokens := getCoreTokens(lastSeg)
		for _, t := range tokens {
			tokenFrequencies[t]++
		}
	}

	// Filter out tokens that appear in more than maxFreq components
	// maxFreq defaults to 5% of components, capped between 5 and 20
	maxFreq := len(components) / 20
	if maxFreq < 5 {
		maxFreq = 5
	}
	if maxFreq > 20 {
		maxFreq = 20
	}

	// Precompute token sets for all components, excluding high-frequency terms
	compTokenSets := make([]map[string]bool, len(components))
	for i, c := range components {
		lastSeg := c
		if idx := strings.LastIndex(c, "/"); idx != -1 {
			lastSeg = c[idx+1:]
		}
		tokens := getCoreTokens(lastSeg)
		set := make(map[string]bool)
		for _, t := range tokens {
			if tokenFrequencies[t] <= maxFreq {
				set[t] = true
			}
		}
		compTokenSets[i] = set
	}

	// Git commits
	fileToCommits, _ := getGitCommitsMapping(results)
	compToCommits := make(map[string]map[string]bool)
	for comp, files := range results.ComponentToFiles {
		compToCommits[comp] = make(map[string]bool)
		for _, f := range files {
			for c := range fileToCommits[f] {
				compToCommits[comp][c] = true
			}
		}
	}
	compCommitsList := make([]map[string]bool, len(components))
	for i, c := range components {
		compCommitsList[i] = compToCommits[c]
	}

	// Dependency graph
	compToDeps := make(map[string][]string)
	for _, conn := range results.Connections {
		if conn.From != conn.To && conn.From != "" && conn.To != "" {
			compToDeps[conn.From] = append(compToDeps[conn.From], conn.To)
		}
	}
	for c, deps := range compToDeps {
		compToDeps[c] = lo.Uniq(deps)
	}

	// Path distances
	distances := computePathDistances(components, compToDeps)

	// Build rows
	var rows []*core.Row
	for i, c1 := range components {
		c1Distances := distances[c1]
		c1Commits := compCommitsList[i]
		c1TokenSet := compTokenSets[i]

		for j, c2 := range components {
			if i == j {
				continue
			}

			// 1. Git co-commits
			sharedCommits := 0
			c2Commits := compCommitsList[j]
			if len(c1Commits) > 0 && len(c2Commits) > 0 {
				c1c, c2c := c1Commits, c2Commits
				if len(c1c) > len(c2c) {
					c1c, c2c = c2c, c1c
				}
				for c := range c1c {
					if c2c[c] {
						sharedCommits++
					}
				}
			}

			// 2. Path distance
			pathDist := -1
			if d, found := c1Distances[c2]; found {
				pathDist = d
			}

			// One row per pair that changed together, the pair written once
			// (from < to): co-change is symmetric, and the rows for pairs
			// that only sat within three hops were two thirds of the table
			// and read by nothing. path_distance is from `from` to `to`.
			if sharedCommits > 0 && c1 < c2 {
				lingSim := jaccardSimilarityPrecomputed(c1TokenSet, compTokenSets[j])
				rows = append(rows, &core.Row{
					Data: map[string]interface{}{
						"from":                  c1,
						"to":                    c2,
						"linguistic_similarity": lingSim,
						"git_co_changes":        sharedCommits,
						"path_distance":         pathDist,
					},
				})
			}
		}
	}

	return &core.View{
		Name: "component_matrix",
		Columns: []*core.Column{
			core.StringColumn("from"),
			core.StringColumn("to"),
			core.FloatColumn("linguistic_similarity"),
			core.IntColumn("git_co_changes"),
			core.IntColumn("path_distance"),
		},
		Rows: rows,
	}
}

// sweepingCommitFiles is the size above which a commit says nothing about
// which files change together. Kept equal to the git extension's default.
const sweepingCommitFiles = 100
