package cmd

import (
	"os"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "gospider",
	Short: "Simple webcrawler collecting all redirecting links",
	Long: `A web crawler fetching all redirecting links to help build a tree and a good 
	understanding of what a page contents might be.`,
}

func Execute() {
	err := rootCmd.Execute()
	if err != nil {
		os.Exit(1)
	}
}

func init() {

	rootCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}
