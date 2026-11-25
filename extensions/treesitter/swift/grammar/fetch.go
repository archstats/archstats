//go:build ignore

// Fetch downloads the generated sources of tree-sitter-swift into this
// directory. They are 615,000 lines of generated C, so the repository does
// not carry them: run `go generate ./extensions/treesitter/swift/grammar`
// once after cloning, before building.
//
// Every file is pinned to the 0.7.3-with-generated-files tag (commit 31d17fe)
// and checked against its sha256; a file already present with the right hash
// is left alone, so running it again is cheap. A failed request is tried
// again with backoff: raw.githubusercontent.com answers 5xx now and then, and
// every CI job runs this before it builds.
package main

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

const base = "https://raw.githubusercontent.com/alex-pinkus/tree-sitter-swift/31d17fe7e818a2048c808b5c6fdc2dc792f4f5b5/src/"

var files = []struct{ path, sha256 string }{
	{"parser.c", "cd57689a482a162c8f5bb2b33ae199adbdbcdc3fb737acec3dba02d07dbd20a6"},
	{"scanner.c", "f3d6271d64f58c39eed544104a70ca2cf9ecbf80c5d900620f1afd38836542cb"},
	{"tree_sitter/alloc.h", "253b44a7b4313a7afd0c505c2fc6e7ce4b8e78955ebf4be3ea000532ec060673"},
	{"tree_sitter/array.h", "4ff743903dc46f5db6aa54f31c6b4d160a8a9779e5b2ab1ee59ae7ebcd850ea1"},
	{"tree_sitter/parser.h", "a1f6ef161fbaf48a0e10fca90ef5290a062462b307b3898aa562993853b9f80a"},
}

func main() {
	client := &http.Client{Timeout: 2 * time.Minute}
	for _, f := range files {
		if err := fetch(client, f.path, f.sha256); err != nil {
			fmt.Fprintf(os.Stderr, "swift grammar: %s: %v\n", f.path, err)
			os.Exit(1)
		}
	}
}

func fetch(client *http.Client, path, want string) error {
	if have, err := os.ReadFile(path); err == nil && sum(have) == want {
		return nil
	}
	var body []byte
	var err error
	for attempt, wait := 1, 2*time.Second; ; attempt, wait = attempt+1, wait*2 {
		var retry bool
		body, retry, err = get(client, base+path)
		if err == nil || !retry || attempt == 5 {
			break
		}
		fmt.Fprintf(os.Stderr, "swift grammar: %s: %v; trying again in %s\n", path, err, wait)
		time.Sleep(wait)
	}
	if err != nil {
		return err
	}
	if got := sum(body); got != want {
		return fmt.Errorf("sha256 is %s, want %s", got, want)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, body, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

// get reports whether a failure is worth another try: the network, a
// timeout, 429 or a 5xx. A 404 or other 4xx will not change.
func get(client *http.Client, url string) (body []byte, retry bool, err error) {
	resp, err := client.Get(url)
	if err != nil {
		return nil, true, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		retry = resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode >= 500
		return nil, retry, fmt.Errorf("GET %s: %s", url, resp.Status)
	}
	body, err = io.ReadAll(resp.Body)
	return body, err != nil, err
}

func sum(b []byte) string {
	h := sha256.Sum256(b)
	return hex.EncodeToString(h[:])
}
