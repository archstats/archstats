package assert_test

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/archstats/archstats/cmd"
	"github.com/stretchr/testify/assert"
)

func TestAssertCommand(t *testing.T) {
	wd, err := os.Getwd()
	assert.NoError(t, err)

	tempDir, err := os.MkdirTemp(wd, "archstats-assert-test-*")
	assert.NoError(t, err)
	defer os.RemoveAll(tempDir)

	// Create directories
	servicesDir := filepath.Join(tempDir, "services")
	dbDir := filepath.Join(tempDir, "db")
	assert.NoError(t, os.MkdirAll(servicesDir, 0755))
	assert.NoError(t, os.MkdirAll(dbDir, 0755))

	// Create dummy Python files representing architectural coupling
	authPy := `
import os
from ..db import DB
`
	dbPy := `
class DB:
    pass
`
	assert.NoError(t, os.WriteFile(filepath.Join(servicesDir, "auth.py"), []byte(authPy), 0644))
	assert.NoError(t, os.WriteFile(filepath.Join(dbDir, "db.py"), []byte(dbPy), 0644))

	// 1. Create a rules file where the assertion expects NO direct db imports from services
	rulesYaml := `
assertions:
  - name: "No Direct DB Imports"
    description: "Services should not import DB directly"
    query: |
      SELECT * FROM component_connections_direct
      WHERE "from" = 'services' AND "to" = 'db'
    expect: 0
`
	rulesPath := filepath.Join(tempDir, "rules.yaml")
	assert.NoError(t, os.WriteFile(rulesPath, []byte(rulesYaml), 0644))

	// Execute command with these rules on the directory (which contains a violation)
	rootCmd, err := cmd.Cmd()
	assert.NoError(t, err)

	buf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetErr(buf)
	rootCmd.SetArgs([]string{"assert", "-r", rulesPath, "-f", tempDir, "--component-strategy", "directory"})

	err = rootCmd.ExecuteContext(context.Background())
	assert.Error(t, err)
	assert.Contains(t, buf.String(), "RULE VIOLATED: No Direct DB Imports")
	assert.Contains(t, buf.String(), "services")
	assert.Contains(t, buf.String(), "db")

	// 2. Create another rules file where we EXPECT exactly 1 violation
	rulesYamlExpect1 := `
assertions:
  - name: "Expect 1 DB Import"
    description: "Services should have exactly 1 DB import"
    query: |
      SELECT * FROM component_connections_direct
      WHERE "from" = 'services' AND "to" = 'db'
    expect: 1
`
	rulesPathExpect1 := filepath.Join(tempDir, "rules_expect1.yaml")
	assert.NoError(t, os.WriteFile(rulesPathExpect1, []byte(rulesYamlExpect1), 0644))

	rootCmd2, err := cmd.Cmd()
	assert.NoError(t, err)

	buf2 := new(bytes.Buffer)
	rootCmd2.SetOut(buf2)
	rootCmd2.SetErr(buf2)
	rootCmd2.SetArgs([]string{"assert", "-r", rulesPathExpect1, "-f", tempDir, "--component-strategy", "directory"})

	err = rootCmd2.ExecuteContext(context.Background())
	assert.NoError(t, err)
	assert.Contains(t, buf2.String(), "✅ Expect 1 DB Import: Passed")
}
