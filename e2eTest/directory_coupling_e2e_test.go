package e2eTest

import (
	"bytes"
	"github.com/archstats/archstats/cmd"
	"github.com/jszwec/csvutil"
	"github.com/stretchr/testify/assert"
	"strings"
	"testing"
)

func Test_E2E_DirectoryStrategy_RealFiles_Advanced(t *testing.T) {
	// 1. Assert Direct Connections (Direct coupling between directories)
	outputDirect := bytes.NewBufferString("")
	err := cmd.Execute(outputDirect, bytes.NewBufferString(""), nil, []string{
		"-o", "csv",
		"-f", "directory_coupling",
		"--component-strategy", "directory",
		"view", "component_connections_direct",
	})
	assert.NoError(t, err)

	var actualConnections []ComponentConnectionDirect
	stringOutputDirect := string(outputDirect.Bytes())
	t.Log("CSV Connections Output:\n" + stringOutputDirect)
	err = csvutil.Unmarshal([]byte(stringOutputDirect), &actualConnections)
	assert.NoError(t, err)

	// Normalize slashes for comparison
	var connections []string
	for _, conn := range actualConnections {
		from := strings.ReplaceAll(conn.From, "\\", "/")
		to := strings.ReplaceAll(conn.To, "\\", "/")
		connections = append(connections, from+" -> "+to)
	}

	// JS/TS circular direct links
	assert.Contains(t, connections, "js_ts/button -> js_ts/layout")
	assert.Contains(t, connections, "js_ts/layout -> js_ts/button")

	// Python circular direct links resolved from dot imports
	assert.Contains(t, connections, "python/services -> python/utils")
	assert.Contains(t, connections, "python/utils -> python/services")

	// 2. Assert Circular Dependency Cycles (Shortest Cycles metrics for directories)
	outputCycles := bytes.NewBufferString("")
	err = cmd.Execute(outputCycles, bytes.NewBufferString(""), nil, []string{
		"-o", "csv",
		"-f", "directory_coupling",
		"--component-strategy", "directory",
		"view", "components",
	})
	assert.NoError(t, err)

	var actualComponents []ComponentCSVRow
	stringOutputCycles := string(outputCycles.Bytes())
	t.Log("CSV Components Output:\n" + stringOutputCycles)
	err = csvutil.Unmarshal([]byte(stringOutputCycles), &actualComponents)
	assert.NoError(t, err)

	// Map of component name to cycle metrics
	componentsMap := make(map[string]ComponentCSVRow)
	for _, row := range actualComponents {
		name := strings.ReplaceAll(row.Name, "\\", "/")
		componentsMap[name] = row
	}

	// Validate circular dependency cycles for JS/TS button component
	buttonComponent, buttonExists := componentsMap["js_ts/button"]
	assert.True(t, buttonExists)
	assert.Equal(t, 2, buttonComponent.ShortCycleCount)
	assert.Equal(t, 2.0, buttonComponent.ShortCycleAvgSize)
	assert.Equal(t, 2, buttonComponent.ShortCycleMaxSize)

	// Validate circular dependency cycles for Python services component
	servicesComponent, servicesExists := componentsMap["python/services"]
	assert.True(t, servicesExists)
	assert.Equal(t, 2, servicesComponent.ShortCycleCount)
	assert.Equal(t, 2.0, servicesComponent.ShortCycleAvgSize)
	assert.Equal(t, 2, servicesComponent.ShortCycleMaxSize)
}

func Test_E2E_FallbackStrategy_RealFiles_Extensive(t *testing.T) {
	// Assert Fallback strategy direct connections
	output := bytes.NewBufferString("")
	err := cmd.Execute(output, bytes.NewBufferString(""), nil, []string{
		"-o", "csv",
		"-f", "directory_coupling",
		"--component-strategy", "fallback",
		"view", "component_connections_direct",
	})
	assert.NoError(t, err)

	var actualConnections []ComponentConnectionDirect
	stringOutput := string(output.Bytes())
	t.Log("CSV Connections Output (Fallback):\n" + stringOutput)
	err = csvutil.Unmarshal([]byte(stringOutput), &actualConnections)
	assert.NoError(t, err)

	// Normalize slashes for comparison
	var connections []string
	for _, conn := range actualConnections {
		from := strings.ReplaceAll(conn.From, "\\", "/")
		to := strings.ReplaceAll(conn.To, "\\", "/")
		connections = append(connections, from+" -> "+to)
	}

	// 1. Java declared package coupling
	assert.Contains(t, connections, "com.app -> com.app.services")

	// 2. Java declared component coupling to fallback JS/TS directory component
	// java/com/app/services/Auth.java imports js_ts.button.
	// Since js_ts.button is dot-separated and not declared, it falls back to directory:
	// js_ts/button is a directory containing snippets, so it matches!
	assert.Contains(t, connections, "com.app.services -> js_ts/button")

	// 3. Fallback JS/TS directory coupling
	assert.Contains(t, connections, "js_ts/button -> js_ts/layout")
}

type ComponentCSVRow struct {
	Name              string  `csv:"NAME"`
	ShortCycleCount   int     `csv:"CYCLES__SHORT__COUNT"`
	ShortCycleAvgSize float64 `csv:"CYCLES__SHORT__AVG"`
	ShortCycleMaxSize int     `csv:"CYCLES__SHORT__MAX"`
}
