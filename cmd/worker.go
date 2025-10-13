package cmd

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/joho/godotenv"
	"github.com/spf13/cobra"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

var (
	sem    = make(chan struct{}, 20)
	mu     sync.Mutex
	update bool
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

func (page *Page) getLinks(client *mongo.Client) {
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
	fmt.Print(string(out))
	//os.WriteFile("storage.json", out, os.ModePerm)
	doc := map[string]any{
		"url":  page.url,
		"seen": page.seen[page.url],
		"time": time.Now().Format(time.DateOnly),
	}
	mu.Lock()
	defer mu.Unlock()
	coll := client.Database("crawler").Collection("links")
	_, err := coll.InsertOne(context.TODO(), doc)
	if err != nil {
		log.Fatal(err)
	}

}

func validateLink(link string) bool {
	resp, err := http.Head(link)
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

func execute(target string, client *mongo.Client) {
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
	page.getLinks(client)
}

func crawl(_ *cobra.Command, startURL string) {
	if err := godotenv.Load(); err != nil {
		log.Println(err)
		log.Println("No .env file found — using system environment variables")
	}
	uri := os.Getenv("MONGO_URI")
	client, err := mongo.Connect(context.TODO(), options.Client().ApplyURI(uri))
	if err != nil {
		panic(err)
	}
	defer func(ctx context.Context) {
		client.Disconnect(ctx)
	}(context.TODO())
	filter := bson.D{
		bson.E{Key: "url", Value: startURL},
	}
	cursor, err := client.Database("crawler").Collection("links").Find(context.TODO(), filter)
	if err != nil {
		log.Fatal(err)
	}
	results := []map[string]any{
		{
			"url":  startURL,
			"seen": []string{},
		},
	}
	if err = cursor.All(context.TODO(), &results); err != nil {
		panic(err)
	}
	if !update {
		if len(results) > 0 {
			data, err := bson.MarshalExtJSON(bson.M{"results": results[0]["seen"]}, false, false)
			if err != nil {
				log.Println("BSON marshal error:", err)
				return
			}
			date := results[0]["time"]
			fmt.Printf("Entry already exists, it was created on %v. Do you wish to update it?\n[y/n] ", date)
			res, err := bufio.NewReader(os.Stdin).ReadString('\n')
			if err != nil {
				log.Println("Input error:", err)
				return
			} else {
				res = strings.TrimSpace(res)
				if res != "y" && res != "Y" && res != "yes" && res != "YES" {
					formatted, err := json.MarshalIndent(string(data), "", "  ")
					if err != nil {
						log.Println("JSON marshal error:", err)
						return
					}
					fmt.Print(string(formatted))
					return
				}
			}
		}
	}
	execute(startURL, client)
}

func init() {
	command.Flags().BoolVarP(&update, "update", "u", false, "Update existing links")
	rootCmd.AddCommand(command)
}
