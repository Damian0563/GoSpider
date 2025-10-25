package cmd

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"

	"github.com/spf13/cobra"
)

var search_command = &cobra.Command{
	Use:   "crawl",
	Short: "Crawl website for redirecting links",
	Long:  "Crawl website for redirecting links recursively with concurrency limits.",
	Run: func(cmd *cobra.Command, args []string) {
		search(cmd, args[0])
	},
	Args: cobra.ExactArgs(1),
}

func standardize_input(query string) []string {
	jsonData, _ := json.Marshal(strings.Fields(query))
	var python_out bytes.Buffer
	var python_err bytes.Buffer
	var python_result []string
	cmd := exec.Command("python", "cmd/standardize.py")
	cmd.Stdin = bytes.NewReader(jsonData)
	cmd.Stderr = &python_err
	cmd.Stdout = &python_out
	if err := cmd.Run(); err != nil {
		fmt.Printf("Python STDERR:\n%s\n", python_err.String())
		return strings.Fields(query)
	}
	trimmedOutput := bytes.TrimSpace(python_out.Bytes())
	if err := json.Unmarshal(trimmedOutput, &python_result); err != nil {
		return strings.Fields(query)
	}
	return python_result
}

func search(_ *cobra.Command, query string) {
	tokenized := standardize_input(query)
	fmt.Println(tokenized)
}
