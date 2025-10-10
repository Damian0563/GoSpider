package cmd

import (
	"encoding/json"
	"fmt"
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
	seen sync.Map // thread-safe map of visited URLs
	sem  = make(chan struct{}, 20)
)

type Page struct {
	url     string
	content string
}

func (page *Page) Split() []string {
	return strings.Split(page.content, " ")
}

func (page *Page) getLinks(depth int) {
	body := page.Split()
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
			if _, loaded := seen.LoadOrStore(corrected, true); loaded {
				return
			}
			go execute(corrected, depth+1)

		}(word)
	}
	wg.Wait()
	all := []string{}
	seen.Range(func(key, _ any) bool {
		all = append(all, key.(string))
		return true
	})
	out, _ := json.MarshalIndent(all, "", "  ")
	os.WriteFile("storage.json", out, os.ModePerm)
	fmt.Println(string(out))
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

func execute(target string, depth int) {
	if depth > 2 {
		return
	}
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
	page := Page{url: target, content: string(bodyByte)}
	page.getLinks(depth)
}

func crawl(cmd *cobra.Command, startURL string) {
	execute(startURL, 0)
}

func init() {
	rootCmd.AddCommand(command)
}
