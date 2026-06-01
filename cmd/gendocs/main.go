package main

import (
	"context"
	"fmt"
	"github.com/archstats/archstats/cmd"
	"github.com/archstats/archstats/cmd/common"
	definitions2 "github.com/archstats/archstats/core/definitions"
	"os"
	"sort"
	"strings"
)

func main() {
	rootCmd, err := cmd.Cmd()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error creating root command: %v\n", err)
		os.Exit(1)
	}

	rootCmd.SetContext(context.Background())

	// We pass the CLI arguments manually to rootCmd flags
	// For gendocs, we enable optional extensions to collect all definition files.
	rootCmd.SetArgs([]string{"view", "definitions", "--extension", "java", "--extension", "csharp", "--extension", "kotlin", "--extension", "cycles"})

	// To prevent executing the "view definitions" action, we'll parse the flags manually 
	// and run common.Analyze directly!
	// Execute the parse command to set all flags on rootCmd
	err = rootCmd.ParseFlags(os.Args[1:])
	if err != nil {
		// If ParseFlags fails or we are not passing normal flags, just set default args
	}

	// Make sure extension flags are populated so common.GetEnabledExtensions works
	rootCmd.Flags().Set("working-dir", ".")
	rootCmd.Flags().Set("extension", "java,csharp,kotlin,cycles")

	results, err := common.Analyze(rootCmd)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error running analysis: %v\n", err)
		os.Exit(1)
	}

	allDefs := results.GetDefinitions()
	var defSlice []*definitions2.Definition
	for _, d := range allDefs {
		defSlice = append(defSlice, d)
	}

	sort.Slice(defSlice, func(i, j int) bool {
		return defSlice[i].Id < defSlice[j].Id
	})

	var sb strings.Builder
	sb.WriteString("# Archstats Metric Reference\n\n")
	sb.WriteString("This document lists all metric definitions registered in `archstats`.\n\n")

	// Group definitions by their Category field
	groups := make(map[string][]*definitions2.Definition)
	for _, def := range defSlice {
		category := def.Category
		if category == "" || category == "Other" {
			category = "Other"
		}
		groups[category] = append(groups[category], def)
	}

	// Gather all unique group names
	var groupNames []string
	for gName := range groups {
		groupNames = append(groupNames, gName)
	}
	sort.Strings(groupNames)

	// Move "Other" to the end if it exists
	if _, hasOther := groups["Other"]; hasOther {
		var sortedWithoutOther []string
		for _, name := range groupNames {
			if name != "Other" {
				sortedWithoutOther = append(sortedWithoutOther, name)
			}
		}
		groupNames = append(sortedWithoutOther, "Other")
	}

	for _, groupName := range groupNames {
		defsInGroup := groups[groupName]
		if len(defsInGroup) == 0 {
			continue
		}
		sb.WriteString(fmt.Sprintf("## %s\n\n", groupName))
		for _, def := range defsInGroup {
			sb.WriteString(fmt.Sprintf("### `%s` (%s)\n\n", def.Id, def.Name))
			
			// Make sure short description is clean
			shortDesc := strings.TrimSpace(def.ShortDescription)
			sb.WriteString(fmt.Sprintf("**Description:** %s\n\n", shortDesc))
			
			if def.LongDescription != "" {
				sb.WriteString(fmt.Sprintf("**Details:**\n%s\n\n", strings.TrimSpace(def.LongDescription)))
			}
			sb.WriteString("---\n\n")
		}
	}

	err = os.WriteFile("docs/metrics.md", []byte(sb.String()), 0644)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error writing markdown: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("Successfully generated docs/metrics.md!")
}
