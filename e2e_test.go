package main

import (
	"encoding/json"
	"fmt"
	"gopkg.in/yaml.v3"
	"os"
	"os/exec"
	"strings"
	"testing"
)

func TestCodebases(t *testing.T) {
	os.Mkdir("./testdata", 0755)
	defer os.RemoveAll("./testdata")

	file, _ := os.ReadFile("e2e_input.yaml")

	config := &Config{}
	yaml.Unmarshal(file, config)

	for _, theCase := range config.Cases {

		dirName := fmt.Sprintf("./testdata/%s", theCase.Name)
		err := exec.Command("git", "clone", theCase.Repo, dirName).Run()
		if err != nil {
			fmt.Println(err)
		}
		err = exec.Command("git", "-C", dirName, "reset", "--hard", theCase.Commit).Run()
		if err != nil {
			fmt.Println(err)
		}

		os.WriteFile(fmt.Sprintf("%s/%s", dirName, ".archstatsignore"), []byte(theCase.Ignore), 0644)
		options := strings.Fields(theCase.OptionArgs)
		mustOptions := []string{"--output-format", "json"}
		output, _ := RunArchstats(append([]string{dirName}, append(options, mustOptions...)...))

		rows := parseJsonRows(output)
		for _, row := range rows {

			fmt.Println(row)
		}
		fmt.Println(rows)
	}
}

func parseJsonRows(input string) []map[string]interface{} {
	var rows []map[string]interface{}
	json.Unmarshal([]byte(input), &rows)
	return rows
}

type Config struct {
	Cases []Case `yaml:"cases"`
}

type Case struct {
	Name               string      `yaml:"name"`
	Repo               string      `yaml:"repo"`
	Commit             string      `yaml:"commit"`
	OptionArgs         string      `yaml:"options"`
	Ignore             string      `yaml:"ignore"`
	ExpectedOutputFile interface{} `yaml:"expectedOutputFile"`
}
