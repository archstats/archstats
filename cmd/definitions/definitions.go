package definitions

import (
	"encoding/json"
	"fmt"
	"github.com/archstats/archstats/cmd/common"
	definitions2 "github.com/archstats/archstats/core/definitions"
	"github.com/ryanuber/columnize"
	"github.com/spf13/cobra"
	"io"
	"sort"
	"strings"
)

func Cmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "definitions [metric-id]",
		Short: "List or describe registered metric definitions",
		Long:  `List or describe registered metric definitions`,
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			results, err := common.Analyze(cmd)
			if err != nil {
				return err
			}

			verbose, _ := cmd.Flags().GetBool("verbose")
			outputFormat, _ := cmd.Flags().GetString("output-format")
			noHeader, _ := cmd.Flags().GetBool("no-header")

			allDefs := results.GetDefinitions()

			// If a single metric-id is provided, show its full details
			if len(args) == 1 {
				metricId := args[0]
				def, exists := allDefs[metricId]
				if !exists {
					// Fallback to name match or case-insensitive ID match
					for _, d := range allDefs {
						if strings.EqualFold(d.Id, metricId) || strings.EqualFold(d.Name, metricId) {
							def = d
							exists = true
							break
						}
					}
				}

				if !exists {
					return fmt.Errorf("no metric definition found for '%s'", metricId)
				}

				if outputFormat == "json" {
					bytes, _ := json.MarshalIndent(def, "", "  ")
					_, err = io.WriteString(cmd.OutOrStdout(), string(bytes)+"\n")
					return err
				}

				// Standard readable output for single metric
				var sb strings.Builder
				sb.WriteString(fmt.Sprintf("ID:                %s\n", def.Id))
				sb.WriteString(fmt.Sprintf("Name:              %s\n", def.Name))
				sb.WriteString(fmt.Sprintf("Short Description: %s\n", def.ShortDescription))
				if def.LongDescription != "" {
					sb.WriteString(fmt.Sprintf("Long Description:\n%s\n", def.LongDescription))
				}
				_, err = io.WriteString(cmd.OutOrStdout(), sb.String())
				return err
			}

			// Otherwise, list all definitions
			var defSlice []*definitions2.Definition
			for _, d := range allDefs {
				defSlice = append(defSlice, d)
			}

			// Sort by ID for stable output
			sort.Slice(defSlice, func(i, j int) bool {
				return defSlice[i].Id < defSlice[j].Id
			})

			// Render list based on output format
			switch outputFormat {
			case "json":
				bytes, _ := json.Marshal(defSlice)
				_, err = io.WriteString(cmd.OutOrStdout(), string(bytes)+"\n")
				return err
			case "csv":
				var rows []string
				if !noHeader {
					if verbose {
						rows = append(rows, "ID,NAME,SHORT_DESCRIPTION,LONG_DESCRIPTION")
					} else {
						rows = append(rows, "ID,NAME,SHORT_DESCRIPTION")
					}
				}
				for _, d := range defSlice {
					if verbose {
						rows = append(rows, fmt.Sprintf("%q,%q,%q,%q", d.Id, d.Name, d.ShortDescription, d.LongDescription))
					} else {
						rows = append(rows, fmt.Sprintf("%q,%q,%q", d.Id, d.Name, d.ShortDescription))
					}
				}
				_, err = io.WriteString(cmd.OutOrStdout(), strings.Join(rows, "\n")+"\n")
				return err
			case "tsv":
				var rows []string
				if !noHeader {
					if verbose {
						rows = append(rows, "ID\tNAME\tSHORT_DESCRIPTION\tLONG_DESCRIPTION")
					} else {
						rows = append(rows, "ID\tNAME\tSHORT_DESCRIPTION")
					}
				}
				for _, d := range defSlice {
					cleanShort := strings.ReplaceAll(strings.ReplaceAll(d.ShortDescription, "\n", " "), "\t", " ")
					cleanLong := strings.ReplaceAll(strings.ReplaceAll(d.LongDescription, "\n", " "), "\t", " ")
					if verbose {
						rows = append(rows, fmt.Sprintf("%s\t%s\t%s\t%s", d.Id, d.Name, cleanShort, cleanLong))
					} else {
						rows = append(rows, fmt.Sprintf("%s\t%s\t%s", d.Id, d.Name, cleanShort))
					}
				}
				_, err = io.WriteString(cmd.OutOrStdout(), strings.Join(rows, "\n")+"\n")
				return err
			default: // table
				var rows []string
				if !noHeader {
					if verbose {
						rows = append(rows, "ID|NAME|SHORT DESCRIPTION|LONG DESCRIPTION")
					} else {
						rows = append(rows, "ID|NAME|SHORT DESCRIPTION")
					}
				}
				for _, d := range defSlice {
					cleanShort := strings.ReplaceAll(d.ShortDescription, "\n", " ")
					cleanLong := strings.ReplaceAll(d.LongDescription, "\n", " ")
					if verbose {
						rows = append(rows, fmt.Sprintf("%s|%s|%s|%s", d.Id, d.Name, cleanShort, cleanLong))
					} else {
						rows = append(rows, fmt.Sprintf("%s|%s|%s", d.Id, d.Name, cleanShort))
					}
				}
				outputStr := columnize.SimpleFormat(rows)
				_, err = io.WriteString(cmd.OutOrStdout(), outputStr+"\n")
				return err
			}
		},
	}

	cmd.Flags().Bool("no-header", false, "No header (only applicable for csv, tsv, table)")
	cmd.Flags().StringP("output-format", "o", "table", "Output format (table, csv, tsv, json)")
	return cmd
}
