package common

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
)

func TestGetEnabledExtensions_AutoDiscovery(t *testing.T) {
	// Create a temp directory inside the workspace (avoid /tmp to honor instructions)
	wd, err := os.Getwd()
	assert.NoError(t, err)
	tempDir, err := os.MkdirTemp(wd, "archstats-test-*")
	assert.NoError(t, err)
	defer os.RemoveAll(tempDir)

	// Create a dummy .java file and .git folder
	err = os.WriteFile(filepath.Join(tempDir, "Main.java"), []byte("public class Main {}"), 0644)
	assert.NoError(t, err)

	err = os.Mkdir(filepath.Join(tempDir, ".git"), 0755)
	assert.NoError(t, err)

	// Create a dummy cobra Command
	cmd := &cobra.Command{}
	cmd.Flags().StringSlice(FlagExtension, nil, "")
	cmd.Flags().String(FlagWorkingDirectory, tempDir, "")

	// 1. Test auto-discovery (no explicit extensions)
	exts, err := GetEnabledExtensions(cmd)
	assert.NoError(t, err)

	var names []string
	for _, e := range exts {
		names = append(names, e.Name)
	}

	// Should contain always-enabled extensions + git + java
	assert.Contains(t, names, "basic")
	assert.Contains(t, names, "indentations")
	assert.Contains(t, names, "git")
	assert.Contains(t, names, "java")
	assert.NotContains(t, names, "csharp")

	// 2. Test explicit extensions (disables auto-discovery)
	cmd2 := &cobra.Command{}
	cmd2.Flags().StringSlice(FlagExtension, []string{"csharp"}, "")
	cmd2.Flags().String(FlagWorkingDirectory, tempDir, "")

	exts2, err := GetEnabledExtensions(cmd2)
	assert.NoError(t, err)

	var names2 []string
	for _, e := range exts2 {
		names2 = append(names2, e.Name)
	}

	// Should contain always-enabled extensions + csharp, but NOT git or java
	assert.Contains(t, names2, "basic")
	assert.Contains(t, names2, "csharp")
	assert.NotContains(t, names2, "git")
	assert.NotContains(t, names2, "java")
}
