package cmd

import "github.com/spf13/cobra"

var command = &cobra.Command{
	Use:   "crawl",
	Short: "Crawl website for redirecting links",
	Long:  "Crawl website for redirecting links",
	Run:   crawl,
}

func crawl(cmd *cobra.Command, args []string) {

}
