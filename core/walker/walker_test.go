package walker

import (
	"bytes"
	"fmt"
	"math/rand"
	"path/filepath"
	"sort"

	"github.com/archstats/archstats/core/file"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"os"
	"runtime"
	"strings"
	"sync"
	"testing"
)

func TestGetAllFiles(t *testing.T) {
	allFiles, err := GetAllFiles("./test_example")

	assert.NoError(t, err)
	assert.Len(t, allFiles, 4)
	for _, file := range allFiles {
		assert.NotContains(t, file.Path(), "ignore")
	}
}

func TestWalkDirectoryConcurrently(t *testing.T) {

	lock := sync.Mutex{}
	var walkedFiles []string
	err := WalkDirectoryConcurrently("./test_example", func(file file.File) {
		assert.NotContains(t, file.Path(), "ignore")
		content := string(file.Content())
		assert.Equal(t, "should not be ignored", content, "file '%s' should be ignored", file.Path())
		lock.Lock()
		walkedFiles = append(walkedFiles, file.Path())
		lock.Unlock()
	})
	assert.NoError(t, err)

	expectedFilesToWalk := []string{
		"subdir2/file6.csv",
		"./file2.txt",
		"./file1.txt",
		"subdir1/file5.txt",
	}
	assert.ElementsMatch(t, expectedFilesToWalk, walkedFiles)
}

func TestIsBinary(t *testing.T) {
	tests := []struct {
		name     string
		content  []byte
		expected bool
	}{
		{"empty content", []byte(""), false},
		{"pure text", []byte("hello world this is text"), false},
		{"null byte at start", append([]byte{0}, []byte("text")...), true},
		{"null byte in middle", []byte("hello\x00world"), true},
		{"null byte at end", []byte("hello world\x00"), true},
		{"large text no null", bytes.Repeat([]byte("a"), 10000), false},
		{"large text with null inside limit", append(bytes.Repeat([]byte("a"), 4000), append([]byte{0}, bytes.Repeat([]byte("a"), 4000)...)...), true},
		{"large text with null outside limit", append(bytes.Repeat([]byte("a"), 9000), append([]byte{0}, bytes.Repeat([]byte("a"), 1000)...)...), false},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			assert.Equal(t, test.expected, isBinary(test.content))
		})
	}
}

func TestWalkerSkipsBinaryFiles(t *testing.T) {
	tempDir := t.TempDir()

	// Create a text file
	textFile := tempDir + "/test.txt"
	err := os.WriteFile(textFile, []byte("this is some text"), 0644)
	assert.NoError(t, err)

	// Create a binary file (contains null byte)
	binaryFile := tempDir + "/test.db"
	err = os.WriteFile(binaryFile, []byte("sqlite\x00database\x00file"), 0644)
	assert.NoError(t, err)

	var walkedFiles []string
	lock := sync.Mutex{}
	err = WalkDirectoryConcurrently(tempDir, func(file file.File) {
		lock.Lock()
		walkedFiles = append(walkedFiles, file.Path())
		lock.Unlock()
	})
	assert.NoError(t, err)

	assert.Len(t, walkedFiles, 1)
	assert.Contains(t, walkedFiles[0], "test.txt")
	assert.NotContains(t, walkedFiles[0], "test.db")
}

func TestWalkDirectoryConcurrentlyNonexistentRoot(t *testing.T) {
	err := WalkDirectoryConcurrently(t.TempDir()+"/does-not-exist", func(file file.File) {
		t.Errorf("visitor should not be called, but was called with %s", file.Path())
	})
	assert.Error(t, err)
}

func TestGetAllFilesNonexistentRoot(t *testing.T) {
	allFiles, err := GetAllFiles(t.TempDir() + "/does-not-exist")
	assert.Error(t, err)
	assert.Nil(t, allFiles)
}

func TestGetAllFilesSkipsUnreadableSubdirectory(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("chmod semantics differ on Windows")
	}
	if os.Geteuid() == 0 {
		t.Skip("running as root bypasses permission checks")
	}

	tempDir := t.TempDir()
	err := os.WriteFile(tempDir+"/readable.txt", []byte("readable"), 0o644)
	assert.NoError(t, err)

	lockedDir := tempDir + "/locked"
	err = os.Mkdir(lockedDir, 0o755)
	assert.NoError(t, err)
	err = os.WriteFile(lockedDir+"/hidden.txt", []byte("hidden"), 0o644)
	assert.NoError(t, err)
	err = os.Chmod(lockedDir, 0o000)
	assert.NoError(t, err)
	t.Cleanup(func() {
		_ = os.Chmod(lockedDir, 0o755)
	})

	allFiles, err := GetAllFiles(tempDir)
	assert.NoError(t, err)
	assert.Len(t, allFiles, 1)
	assert.Contains(t, allFiles[0].Path(), "readable.txt")
}

func TestWalkFilesRecoversFromVisitorPanic(t *testing.T) {
	tempDir := t.TempDir()
	err := os.WriteFile(tempDir+"/panics.txt", []byte("this file panics"), 0o644)
	assert.NoError(t, err)
	err = os.WriteFile(tempDir+"/fine.txt", []byte("this file is fine"), 0o644)
	assert.NoError(t, err)

	var walkedFiles []string
	lock := sync.Mutex{}
	err = WalkDirectoryConcurrently(tempDir, func(file file.File) {
		if strings.Contains(file.Path(), "panics") {
			panic("extension blew up")
		}
		lock.Lock()
		walkedFiles = append(walkedFiles, file.Path())
		lock.Unlock()
	})

	assert.NoError(t, err)
	assert.Len(t, walkedFiles, 1)
	assert.Contains(t, walkedFiles[0], "fine.txt")
}

func TestVersionControlMetadataIsNeverWalked(t *testing.T) {
	for _, name := range []string{".git", ".hg", ".svn", ".bzr", "_darcs", ".jj"} {
		assert.True(t, isVCSMetadata(name), name)
	}
	for _, name := range []string{".github", ".gitignore", "git", ".gitlab-ci.yml"} {
		assert.False(t, isVCSMetadata(name), name)
	}
}

// A nested ignore file's patterns are relative to its own directory, and
// apply to nothing outside it. IntelliJ commits `.idea/.gitignore` with
// anchored entries such as `/workspace.xml`; pooled with the root's patterns
// they matched nothing, and IDE state was scanned as code.
func TestNestedIgnoreFilesAreRelativeToTheirDirectory(t *testing.T) {
	root := t.TempDir()
	write := func(p string) {
		full := filepath.Join(root, p)
		require.NoError(t, os.MkdirAll(filepath.Dir(full), 0o755))
		require.NoError(t, os.WriteFile(full, []byte("x"), 0o644))
	}
	require.NoError(t, os.MkdirAll(filepath.Join(root, "sub"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(root, "sub", ".gitignore"), []byte("/dist\n*.log\n"), 0o644))
	require.NoError(t, os.MkdirAll(filepath.Join(root, ".idea"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(root, ".idea", ".gitignore"), []byte("/workspace.xml\n"), 0o644))
	for _, p := range []string{"dist/root.js", "sub/dist/sub.js", "sub/keep.js", "sub/a.log", "sibling/b.log", ".idea/workspace.xml", ".idea/misc.xml"} {
		write(p)
	}
	files, err := GetAllFiles(root)
	require.NoError(t, err)
	var got []string
	for _, f := range files {
		got = append(got, strings.TrimPrefix(f.Path(), "./"))
	}
	sort.Strings(got)
	assert.Equal(t, []string{".idea/misc.xml", "dist/root.js", "sibling/b.log", "sub/keep.js"}, got)
}

// `dir/*` ignores what is in dir, not dir itself, so a negation can bring a
// file back -- nopCommerce keeps an Index.htm in each App_Data folder that way.
func TestNegationBringsAFileBackFromAnIgnoredFolder(t *testing.T) {
	root := t.TempDir()
	for _, p := range []string{"store/Index.htm", "store/token.json", "target/out.class"} {
		full := filepath.Join(root, p)
		require.NoError(t, os.MkdirAll(filepath.Dir(full), 0o755))
		require.NoError(t, os.WriteFile(full, []byte("x"), 0o644))
	}
	require.NoError(t, os.WriteFile(filepath.Join(root, ".gitignore"), []byte("store/*\n!store/Index.htm\ntarget/\n"), 0o644))
	files, err := GetAllFiles(root)
	require.NoError(t, err)
	var got []string
	for _, f := range files {
		got = append(got, strings.TrimPrefix(f.Path(), "./"))
	}
	assert.Equal(t, []string{"store/Index.htm"}, got)
}

// What a scan leaves out is reported the way a reader thinks of it: a
// directory it never entered is one line, not every file inside it.
func TestScanReportsWhatItIgnored(t *testing.T) {
	root := t.TempDir()
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	for _, d := range []string{".git", "node_modules/react", "src"} {
		must(os.MkdirAll(filepath.Join(root, d), 0o755))
	}
	must(os.WriteFile(filepath.Join(root, ".gitignore"), []byte("node_modules/\n*.log\n"), 0o644))
	must(os.WriteFile(filepath.Join(root, "node_modules/react/index.js"), []byte("x"), 0o644))
	must(os.WriteFile(filepath.Join(root, "src/app.ts"), []byte("x"), 0o644))
	must(os.WriteFile(filepath.Join(root, "debug.log"), []byte("x"), 0o644))

	files, err := Scan(root)
	must(err)
	ig := files.Ignored()
	if ig.Dirs != 2 || ig.Files != 2 {
		t.Fatalf("ignored %d dirs, %d files (top %v)", ig.Dirs, ig.Files, ig.Top)
	}
	joined := strings.Join(ig.Top, " ")
	if !strings.Contains(joined, "node_modules/") || strings.Contains(joined, "react/index.js") {
		t.Fatalf("top = %v", ig.Top)
	}
}

// cycles.ts in archstats-ui joins keys with a literal NUL. The walker took it
// for binary and dropped it, so its tests looked like they imported a file
// that did not exist.
const tsWithNUL = "import { Edge } from \"./graph\";\n" +
	"const SEP = \"\x00\";\n" +
	"export function key(a: string, b: string): string { return a + SEP + b; }\n"

func claimsTS(path string) bool { return strings.HasSuffix(path, ".ts") }

func walkAndCollect(t *testing.T, dir string, opts ...Options) (map[string]string, *Report) {
	t.Helper()
	walked := map[string]string{}
	lock := sync.Mutex{}
	report, err := WalkAndReport(dir, func(f file.File) {
		lock.Lock()
		walked[strings.TrimPrefix(f.Path(), "./")] = string(f.Content())
		lock.Unlock()
	}, opts...)
	require.NoError(t, err)
	return walked, report
}

func TestClaimedSourceWithNULIsWalked(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "frontend", "src", "utils"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "frontend", "src", "utils", "cycles.ts"), []byte(tsWithNUL), 0o644))

	walked, report := walkAndCollect(t, dir, Options{Claims: claimsTS})

	assert.Equal(t, tsWithNUL, walked["frontend/src/utils/cycles.ts"], "the whole file, NUL included, reaches the analyzers")
	assert.Empty(t, report.Skipped)
}

func TestUnclaimedFileWithNULIsSkippedAndRecorded(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "src"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "src", "cycles.ts"), []byte(tsWithNUL), 0o644))

	// No language pack loaded that reads .ts: the NUL test applies.
	walked, report := walkAndCollect(t, dir)

	assert.Empty(t, walked)
	require.Len(t, report.Skipped, 1)
	assert.Equal(t, "src/cycles.ts", report.Skipped[0].Path)
	assert.Equal(t, SkipBinary, report.Skipped[0].Reason)
	assert.Equal(t, fmt.Sprintf("NUL byte at offset %d", strings.IndexByte(tsWithNUL, 0)), report.Skipped[0].Detail)
}

// .ts is also an MPEG transport stream. A claim is not a licence to feed
// video to a parser.
func TestClaimedFileThatIsNotTextIsSkipped(t *testing.T) {
	dir := t.TempDir()
	video := make([]byte, 188*40)
	rand.New(rand.NewSource(1)).Read(video)
	for i := 0; i < len(video); i += 188 {
		video[i] = 0x47 // sync byte
	}
	require.NoError(t, os.WriteFile(filepath.Join(dir, "clip.ts"), video, 0o644))

	walked, report := walkAndCollect(t, dir, Options{Claims: claimsTS})

	assert.Empty(t, walked)
	require.Len(t, report.Skipped, 1)
	assert.Equal(t, "clip.ts", report.Skipped[0].Path)
	assert.Equal(t, SkipBinary, report.Skipped[0].Reason)
	assert.Contains(t, report.Skipped[0].Detail, "are not text")
}

func TestSkippedFilesRecordEveryReason(t *testing.T) {
	dir := t.TempDir()
	write := func(rel, content string) {
		p := filepath.Join(dir, filepath.FromSlash(rel))
		require.NoError(t, os.MkdirAll(filepath.Dir(p), 0o755))
		require.NoError(t, os.WriteFile(p, []byte(content), 0o644))
	}
	write(".gitignore", "dist/\n*.log\n")
	write("dist/bundle.js", "minified")
	write("debug.log", "noise")
	write(".git/HEAD", "ref: refs/heads/main")
	write("logo.png", "\x89PNG\r\n\x1a\n\x00\x00\x00\rIHDR")
	write("src/cycles.ts", tsWithNUL)
	write("src/panics.txt", "an extension blows up here")
	write("src/ok.txt", "fine")

	checkUnreadable := runtime.GOOS != "windows" && os.Geteuid() != 0
	if checkUnreadable {
		write("secret.txt", "locked")
		require.NoError(t, os.Chmod(filepath.Join(dir, "secret.txt"), 0o000))
		t.Cleanup(func() { _ = os.Chmod(filepath.Join(dir, "secret.txt"), 0o644) })
	}

	walked := map[string]bool{}
	lock := sync.Mutex{}
	report, err := WalkAndReport(dir, func(f file.File) {
		if strings.Contains(f.Path(), "panics") {
			panic("extension blew up")
		}
		lock.Lock()
		walked[strings.TrimPrefix(f.Path(), "./")] = true
		lock.Unlock()
	}, Options{Claims: claimsTS})
	require.NoError(t, err)

	assert.True(t, walked["src/cycles.ts"])
	assert.True(t, walked["src/ok.txt"])

	got := map[string]string{}
	for _, s := range report.Skipped {
		got[s.Path] = s.Reason
		if s.Reason != SkipIgnored {
			assert.NotEmpty(t, s.Detail, "%s should say why", s.Path)
		}
	}
	want := map[string]string{
		".git/":          SkipIgnored,
		".gitignore":     SkipIgnored, // ignore files are never analysed
		"dist/":          SkipIgnored,
		"debug.log":      SkipIgnored,
		"logo.png":       SkipBinary,
		"src/panics.txt": SkipFailed,
	}
	if checkUnreadable {
		want["secret.txt"] = SkipUnreadable
	}
	assert.Equal(t, want, got)
	assert.True(t, sort.SliceIsSorted(report.Skipped, func(i, j int) bool {
		return report.Skipped[i].Path < report.Skipped[j].Path
	}), "skipped paths are sorted")
}

func TestSkippedRecordsUnreadableDirectory(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("chmod semantics differ on Windows")
	}
	if os.Geteuid() == 0 {
		t.Skip("running as root bypasses permission checks")
	}
	dir := t.TempDir()
	locked := filepath.Join(dir, "sub", "locked")
	require.NoError(t, os.MkdirAll(locked, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(locked, "a.txt"), []byte("a"), 0o644))
	require.NoError(t, os.Chmod(locked, 0o000))
	t.Cleanup(func() { _ = os.Chmod(locked, 0o755) })

	_, report := walkAndCollect(t, dir)

	require.Len(t, report.Skipped, 1)
	assert.Equal(t, "sub/locked/", report.Skipped[0].Path)
	assert.Equal(t, SkipUnreadable, report.Skipped[0].Reason)
}

func TestBinaryReason(t *testing.T) {
	random := make([]byte, 4000)
	rand.New(rand.NewSource(2)).Read(random)
	tests := []struct {
		name    string
		content []byte
		claimed bool
		binary  bool
	}{
		{"claimed text with a NUL", []byte(tsWithNUL), true, false},
		{"unclaimed text with a NUL", []byte(tsWithNUL), false, true},
		{"claimed plain text", []byte("export const a = 1;\n"), true, false},
		{"claimed UTF-8 text", []byte("const naïve = \"日本語\"; // ✓\n"), true, false},
		{"claimed Latin-1 comment", []byte("// caf\xe9\n" + strings.Repeat("let x = 1;\n", 20)), true, false},
		{"claimed text with ANSI escapes", []byte("const red = \"\x1b[31m\";\n"), true, false},
		{"claimed random bytes", random, true, true},
		{"claimed UTF-16", []byte("i\x00m\x00p\x00o\x00r\x00t\x00 \x00x\x00;\x00"), true, true},
		{"claimed empty", nil, true, false},
		{"multi-byte character cut at the sniff limit", append(bytes.Repeat([]byte("a"), sniffLen-1), "é"...), true, false},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			why := binaryReason(test.content, test.claimed)
			assert.Equal(t, test.binary, why != "", why)
		})
	}
}

// archstats-ui's .gitignore has `node_modules` and `!.env.example`. The
// negation can match at any depth, so every ignored directory used to be
// walked file by file in case it held one: 31,495 node_modules rows in
// skipped_files. Git never re-includes a file whose directory is excluded.
func TestExcludedDirectoryIsPrunedDespiteNegations(t *testing.T) {
	root := t.TempDir()
	for _, p := range []string{"frontend/node_modules/react/index.js", "frontend/node_modules/.env.example", "frontend/.nuxt/app.js", "frontend/src/app.ts", ".env.example", ".env"} {
		full := filepath.Join(root, filepath.FromSlash(p))
		require.NoError(t, os.MkdirAll(filepath.Dir(full), 0o755))
		require.NoError(t, os.WriteFile(full, []byte("x"), 0o644))
	}
	require.NoError(t, os.WriteFile(filepath.Join(root, ".gitignore"), []byte("node_modules\n.nuxt/\n.env\n.env.*\n!.env.example\n"), 0o644))

	walked, report := walkAndCollect(t, root)

	assert.Contains(t, walked, ".env.example", "the negation still brings back a file whose directory is kept")
	assert.Contains(t, walked, "frontend/src/app.ts")
	var skipped []string
	for _, s := range report.Skipped {
		skipped = append(skipped, s.Path)
	}
	assert.ElementsMatch(t, []string{".env", ".gitignore", "frontend/.nuxt/", "frontend/node_modules/"}, skipped)
}

// `dir/**` excludes what is inside dir, like `dir/*`, so a negation still
// reaches into it.
func TestNegationReachesIntoContentOnlyExclusion(t *testing.T) {
	root := t.TempDir()
	for _, p := range []string{"dist/keep.txt", "dist/bundle.js"} {
		full := filepath.Join(root, filepath.FromSlash(p))
		require.NoError(t, os.MkdirAll(filepath.Dir(full), 0o755))
		require.NoError(t, os.WriteFile(full, []byte("x"), 0o644))
	}
	require.NoError(t, os.WriteFile(filepath.Join(root, ".gitignore"), []byte("dist/**\n!dist/keep.txt\n"), 0o644))

	walked, _ := walkAndCollect(t, root)

	assert.Equal(t, map[string]string{"dist/keep.txt": "x"}, walked)
}

// A directory whose contents are ignored (`dist/*`) is walked in case a
// negation keeps something. When nothing is kept it is reported as one
// directory; when something is, each ignored file is listed.
func TestIgnoredContentsRollUpToTheirDirectory(t *testing.T) {
	root := t.TempDir()
	for _, p := range []string{"frontend/dist/index.html", "frontend/dist/_nuxt/a.js", "frontend/dist/_nuxt/b.js", "build/out.js", "build/.gitkeep"} {
		full := filepath.Join(root, filepath.FromSlash(p))
		require.NoError(t, os.MkdirAll(filepath.Dir(full), 0o755))
		require.NoError(t, os.WriteFile(full, []byte("x"), 0o644))
	}
	require.NoError(t, os.WriteFile(filepath.Join(root, ".gitignore"),
		[]byte("frontend/dist/*\n!frontend/dist/.gitkeep\nbuild/*\n!build/.gitkeep\n"), 0o644))

	walked, report := walkAndCollect(t, root)

	assert.Equal(t, map[string]string{"build/.gitkeep": "x"}, walked)
	var skipped []string
	for _, s := range report.Skipped {
		skipped = append(skipped, s.Path)
	}
	assert.Equal(t, []string{".gitignore", "build/out.js", "frontend/dist/"}, skipped)
}
