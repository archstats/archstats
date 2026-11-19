package git

import (
	"os"
	"os/exec"
	"strings"
)

// pinnedConfig holds the git settings that change what a scan reads, at
// git's own defaults, so the same history scans the same on every machine.
// The user's config still applies to everything else: safe.directory,
// credentials, includes.
//
//   - core.quotepath=off keeps non-ASCII paths as written instead of
//     C-quoting them, which no file in the scan would ever match.
//   - core.abbrev=auto: commits are named by their abbreviated hash, and a
//     user's core.abbrev=12 renamed every one of them.
//   - log.showSignature=false: with it on, git verified the signature of
//     every commit it printed, one gpg or ssh-keygen run per commit.
//   - diff.algorithm and diff.renameLimit decide the added and deleted line
//     counts and which moves are found.
//   - i18n.logOutputEncoding: commit messages and author names arrive in
//     UTF-8, whatever the user reads them in.
var pinnedConfig = []string{
	"-c", "core.quotepath=off",
	"-c", "core.abbrev=auto",
	"-c", "log.showSignature=false",
	"-c", "diff.algorithm=myers",
	"-c", "diff.renameLimit=1000",
	"-c", "i18n.logOutputEncoding=UTF-8",
}

// localEnvVars are the variables that tell git which repository it is in
// (`git rev-parse --local-env-vars`). A scan run from a git hook, or from
// anything else that exports GIT_DIR, read that repository's history for
// every repository it scanned: -C does not override them.
var localEnvVars = map[string]bool{
	"GIT_ALTERNATE_OBJECT_DIRECTORIES": true,
	"GIT_CONFIG":                       true,
	"GIT_CONFIG_PARAMETERS":            true,
	"GIT_CONFIG_COUNT":                 true,
	"GIT_OBJECT_DIRECTORY":             true,
	"GIT_DIR":                          true,
	"GIT_WORK_TREE":                    true,
	"GIT_IMPLICIT_WORK_TREE":           true,
	"GIT_GRAFT_FILE":                   true,
	"GIT_INDEX_FILE":                   true,
	"GIT_NO_REPLACE_OBJECTS":           true,
	"GIT_REPLACE_REF_BASE":             true,
	"GIT_PREFIX":                       true,
	"GIT_INTERNAL_SUPER_PREFIX":        true,
	"GIT_SHALLOW_FILE":                 true,
	"GIT_COMMON_DIR":                   true,
}

// gitCommand is every git the extension runs: the repository is the one -C
// names, and the settings that would change the answer are pinned.
func gitCommand(args ...string) *exec.Cmd {
	cmd := exec.Command("git", append(append([]string{}, pinnedConfig...), args...)...)
	cmd.Env = gitEnv(os.Environ())
	return cmd
}

func gitEnv(environ []string) []string {
	out := make([]string, 0, len(environ))
	for _, kv := range environ {
		name, _, _ := strings.Cut(kv, "=")
		if localEnvVars[strings.ToUpper(name)] {
			continue
		}
		out = append(out, kv)
	}
	return out
}
