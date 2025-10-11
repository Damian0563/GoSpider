package cmd

import (
	"encoding/json"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"

	"github.com/spf13/cobra"
)

var (
	sem = make(chan struct{}, 20)
)

type Page struct {
	url     string
	content string
	seen    map[string][]string
	mu      sync.Mutex
}

func (page *Page) Split() []string {
	return strings.Split(page.content, " ")
}

func (page *Page) getLinks() {
	body := page.Split()
	page.seen[page.url] = []string{}
	var wg sync.WaitGroup
	for _, word := range body {
		if !strings.HasPrefix(word, "href=\"") {
			continue
		}
		wg.Add(1)
		go func(word string) {
			defer wg.Done()
			link, found := strings.CutPrefix(word, "href=\"")
			if !found {
				return
			}
			end := strings.Index(link, "\"")
			if end == -1 {
				return
			}
			link = link[:end]
			if strings.HasSuffix(link, ".js") ||
				strings.HasSuffix(link, ".svg") ||
				strings.HasSuffix(link, ".css") ||
				strings.HasPrefix(link, "#") {
				return
			}
			base, _ := url.Parse(page.url)
			href, _ := url.Parse(link)
			corrected := base.ResolveReference(href).String()
			if validateLink(corrected) {
				page.mu.Lock()
				page.seen[page.url] = append(page.seen[page.url], corrected)
				page.mu.Unlock()
			}
		}(word)
	}
	wg.Wait()
	out, _ := json.MarshalIndent(page.seen, "", "  ")
	os.WriteFile("storage.json", out, os.ModePerm)
}

func validateLink(link string) bool {
	resp, err := http.Get(link)
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	return resp.StatusCode < 400
}

var command = &cobra.Command{
	Use:   "crawl",
	Short: "Crawl website for redirecting links",
	Long:  "Crawl website for redirecting links recursively with concurrency limits.",
	Run: func(cmd *cobra.Command, args []string) {
		crawl(cmd, args[0])
	},
	Args: cobra.ExactArgs(1),
}

func execute(target string) {
	sem <- struct{}{}
	defer func() { <-sem }()
	resp, err := http.Get(target)
	if err != nil {
		log.Println("Request failed:", err)
		return
	}
	defer resp.Body.Close()
	bodyByte, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Println("Read failed:", err)
		return
	}
	page := Page{url: target, content: string(bodyByte), seen: make(map[string][]string)}
	page.getLinks()
}

func crawl(cmd *cobra.Command, startURL string) {
	execute(startURL)
}

func init() {
	rootCmd.AddCommand(command)
}
