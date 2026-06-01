package assert

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/archstats/archstats/cmd/common"
	"github.com/archstats/archstats/cmd/export/sqlite"
	"github.com/archstats/archstats/core"
	_ "github.com/mattn/go-sqlite3"
	"github.com/ryanuber/columnize"
	"github.com/samber/lo"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

type Assertion struct {
	Name        string `yaml:"name"`
	Description string `yaml:"description"`
	Query       string `yaml:"query"`
	Expect      int    `yaml:"expect"`
}

type Config struct {
	Assertions []*Assertion `yaml:"assertions"`
}

func Cmd() *cobra.Command {
	var rulesPath string
	cmd := &cobra.Command{
		Use:          "assert",
		Short:        "Run SQL-based architectural assertions against the codebase",
		Long:         `Run SQL-based architectural assertions against the codebase`,
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			if rulesPath == "" {
				return fmt.Errorf("rules file must be specified with --rules or -r")
			}

			// Load rules
			cfg, err := loadRulesFile(rulesPath)
			if err != nil {
				return err
			}

			// Analyze codebase
			results, err := common.Analyze(cmd)
			if err != nil {
				return err
			}

			// Render all views
			possibleViews := lo.Map(results.GetViewFactories(), func(vf *core.ViewFactory, index int) string {
				return vf.Name
			})
			var views []*core.View
			for _, viewName := range possibleViews {
				v, err := results.RenderView(viewName)
				if err != nil {
					return err
				}
				views = append(views, v)
			}

			// Create a temporary database file in the workspace
			tempDbPath := filepath.Join(".", "temp_assertions.db")

			// Remove any pre-existing temp file
			_ = os.Remove(tempDbPath)
			defer os.Remove(tempDbPath)

			// Save all views to SQLite DB
			err = sqlite.SaveToDB(&sqlite.SqlOptions{
				DatabaseName: tempDbPath,
				ReportId:     "assertions_run",
				ScanTime:     time.Now(),
			}, results, views)
			if err != nil {
				return fmt.Errorf("failed to export views to temp database: %w", err)
			}

			// Open DB and run queries
			db, err := sql.Open("sqlite3", tempDbPath)
			if err != nil {
				return fmt.Errorf("failed to open assertion database: %w", err)
			}
			defer db.Close()

			failures := 0
			for _, assertion := range cfg.Assertions {
				rows, err := db.Query(assertion.Query)
				if err != nil {
					fmt.Fprintf(cmd.OutOrStdout(), "❌ Assertion '%s' failed to execute: %v\n", assertion.Name, err)
					failures++
					continue
				}

				cols, err := rows.Columns()
				if err != nil {
					rows.Close()
					return err
				}

				var violations [][]string
				for rows.Next() {
					rowVals := make([]interface{}, len(cols))
					rowValStrings := make([]string, len(cols))
					for i := range rowVals {
						rowVals[i] = &rowValStrings[i]
					}
					err = rows.Scan(rowVals...)
					if err != nil {
						rows.Close()
						return err
					}
					violations = append(violations, rowValStrings)
				}
				rows.Close()

				if len(violations) != assertion.Expect {
					failures++
					fmt.Fprintf(cmd.OutOrStdout(), "\n❌ RULE VIOLATED: %s\n", assertion.Name)
					if assertion.Description != "" {
						fmt.Fprintf(cmd.OutOrStdout(), "   Description: %s\n", assertion.Description)
					}
					fmt.Fprintf(cmd.OutOrStdout(), "   Expected: %d violations, got %d:\n\n", assertion.Expect, len(violations))

					// Pretty-print the table
					var tableRows []string
					upperCols := lo.Map(cols, func(c string, _ int) string {
						return strings.ToUpper(c)
					})
					tableRows = append(tableRows, strings.Join(upperCols, "|"))
					for _, v := range violations {
						tableRows = append(tableRows, strings.Join(v, "|"))
					}
					fmt.Fprintln(cmd.OutOrStdout(), columnize.SimpleFormat(tableRows))
					fmt.Fprintln(cmd.OutOrStdout())
				} else {
					fmt.Fprintf(cmd.OutOrStdout(), "✅ %s: Passed\n", assertion.Name)
				}
			}

			if failures > 0 {
				return fmt.Errorf("architectural assertions failed with %d violations", failures)
			}
			fmt.Fprintln(cmd.OutOrStdout(), "\n🎉 All assertions passed successfully!")
			return nil
		},
	}

	cmd.Flags().StringVarP(&rulesPath, "rules", "r", "", "Path to the YAML rules assertion file")
	return cmd
}

func loadRulesFile(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read rules file: %w", err)
	}
	var cfg Config
	err = yaml.Unmarshal(data, &cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to parse rules YAML: %w", err)
	}
	return &cfg, nil
}
