package main

//TODO
//import (
//	"fmt"
//	"github.com/stretchr/testify/assert"
//	"gopkg.in/yaml.v3"
//	"log"
//	"os"
//	"os/exec"
//	"strings"
//	"testing"
//)
//
//const TestDataPath = "./temp_testdata"
//
//func Test_E2E(t *testing.T) {
//
//	os.Mkdir(TestDataPath, 0755)
//	//defer os.RemoveAll(TestDataPath) TODO: Should not remove all in the case of mutation testing
//
//	file, _ := os.ReadFile("e2e_input.yaml")
//
//	config := &Config{}
//	yaml.Unmarshal(file, config)
//
//	for _, theCase := range config.Cases {
//		testCase(t, theCase)
//	}
//}
//
//func testCase(t *testing.T, theCase Case) {
//	log.Println("Testing case:", theCase.Name)
//
//	repoDirName := strings.ReplaceAll(fmt.Sprintf("%s/%s", TestDataPath, theCase.Repo[strings.LastIndex(theCase.Repo, "/"):]), ".git", "")
//	if _, err := os.Stat(repoDirName); os.IsNotExist(err) {
//		log.Println("Cloning repo:", theCase.Repo)
//		err := exec.Command("git", "clone", theCase.Repo, repoDirName).Run()
//		if err != nil {
//			assert.Fail(t, "Failed to clone repo: {}", theCase.Repo)
//		}
//	} else {
//		log.Println("Repo already exists, skipping clone...")
//	}
//
//	err := exec.Command("git", "-C", repoDirName, "reset", "--hard", theCase.Commit).Run()
//	if err != nil {
//		assert.Fail(t, "Failed to reset repo (%s) to commit '%s'", theCase.Repo, theCase.Commit)
//	}
//
//	err = os.WriteFile(fmt.Sprintf("%s/%s", repoDirName, ".archstatsignore"), []byte(theCase.Ignore), 0644)
//	defer os.Remove(fmt.Sprintf("%s/%s", repoDirName, ".archstatsignore"))
//	if err != nil {
//		assert.Fail(t, "Failed to write .archstatsignore file")
//	}
//	allArgs := append([]string{repoDirName}, strings.Fields(theCase.OptionArgs)...)
//
//	actualOutput, err := RunArchstats(allArgs)
//
//	if err != nil {
//		assert.Fail(t, "Failed to run archstats: %s", err)
//	}
//
//	expectedOutputBytes, err := os.ReadFile(theCase.ExpectedOutputFile)
//	if err != nil {
//		assert.Fail(t, "Failed to read expected output file: %s", theCase.ExpectedOutputFile)
//	}
//
//	options, _ := getOptions(append([]string{repoDirName}, allArgs...))
//
//	expectedOutput := strings.TrimSpace(string(expectedOutputBytes))
//	actualOutput = strings.TrimSpace(actualOutput)
//	var passed bool
//
//	if options.SortedBy == "" {
//		expectedOutputLines := strings.Split(expectedOutput, "\n")
//		actualOutputLines := strings.Split(actualOutput, "\n")
//		passed = assert.ElementsMatch(t, expectedOutputLines, actualOutputLines, "Actual output does not match expected output")
//	} else {
//		passed = assert.Equal(t, expectedOutput, actualOutput, "Actual output does not match expected output")
//	}
//
//	if passed {
//		log.Println("Case passed:", theCase.Name)
//		log.Println()
//	} else {
//		log.Println("Expected output:")
//		log.Println(expectedOutput)
//		log.Println("Actual output:")
//		log.Println(actualOutput)
//	}
//}
//
//type Config struct {
//	Cases []Case `yaml:"cases"`
//}
//
//type Case struct {
//	Name               string `yaml:"name"`
//	Repo               string `yaml:"repo"`
//	Commit             string `yaml:"commit"`
//	OptionArgs         string `yaml:"options"`
//	Ignore             string `yaml:"ignore"`
//	ExpectedOutputFile string `yaml:"expectedOutputFile"`
//}
