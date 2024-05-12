package main

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"
)

func main() {
	version := getLatestTag()
	commit := getCommit()

	versionFile := `// Code generated by go generate; DO NOT EDIT.
package main

var (
	version = "%s"
	commit  = "%s"
)
`
	// Write the generated code to version.go in the parent directory
	outputFile, _ := os.Create("./version.go")
	defer outputFile.Close()

	fmt.Fprintf(outputFile, versionFile, version, commit)
}

func getLatestTag() string {
	cmd := exec.Command("git", "describe", "--tags", "--abbrev=0")
	output, err := cmd.Output()
	if err != nil {
		return "unknown"
	}
	return strings.TrimSpace(string(output))
}

func getCommit() string {
	cmd := exec.Command("git", "rev-parse", "HEAD")
	output, err := cmd.Output()
	if err != nil {
		return "unknown"
	}
	return strings.TrimSpace(string(output))
}

func getDate() string {
	return time.Now().Format("2006-01-02")
}
