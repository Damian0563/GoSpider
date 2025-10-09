package cmd

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"

	"github.com/spf13/cobra"
)

type Page struct {
	url     string
	content string
}

func (page *Page) Split() []string {
	return strings.Split(page.content, " ")
}

func (page *Page) getLinks() {
	body := page.Split()
	for _, word := range body {
		if strings.HasPrefix(word, "href") {
			fmt.Println(word)
		}
	}
}

var command = &cobra.Command{
	Use:   "crawl",
	Short: "Crawl website for redirecting links",
	Long:  "Crawl website for redirecting links",
	Run: func(cmd *cobra.Command, args []string) {
		crawl(cmd, args[0])
	},
	Args: cobra.ExactArgs(1),
}

func crawl(cmd *cobra.Command, url string) {
	page := Page{url: url}
	resp, err := http.Get(url)
	if err != nil {
		log.Fatalln(err)
		return
	}
	defer resp.Body.Close()
	bodyByte, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Fatalln(err)
	}
	page.content = string(bodyByte)
	page.getLinks()
}

func init() {
	rootCmd.AddCommand(command)
}
