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

	"cloud.google.com/go/vertexai/genai"
	"github.com/joho/godotenv"
	"github.com/spf13/cobra"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

var INPUT_PROMPT string = "Create a concise and informative brief summary for the following HTML content: "

var (
	sem       = make(chan struct{}, 20)
	mu        sync.Mutex
	update    bool
	recursive bool
	cache     sync.Map
)

type Page struct {
	url     string
	content string
	seen    map[string][]string
	mu      sync.Mutex
}

func summarizePage(html_content string) any {
	ctx := context.Background()
	client, err := genai.NewClient(ctx, os.Getenv("PROJECT_ID"), os.Getenv("REGION"))
	if err != nil {
		return ""
	}
	defer client.Close()
	prompt := genai.Text(INPUT_PROMPT + html_content)
	response, err := client.GenerativeModel(os.Getenv("MODEL")).GenerateContent(ctx, prompt)
	if err != nil {
		return ""
	}
	return response.Candidates[0].Content.Parts[0]
}

func check_exsistence(client *mongo.Client, url string) bool {
	filter := bson.D{
		bson.E{Key: "url", Value: url},
	}
	count, err := client.Database("crawler").Collection("links").CountDocuments(context.TODO(), filter)
	if err != nil {
		return true
	}
	return count > 0
}

func (page *Page) Split() []string {
	return strings.Split(page.content, " ")
}

func (page *Page) getLinks(client *mongo.Client, wg *sync.WaitGroup) {
	body := page.Split()
	defer wg.Done()
	page.seen[page.url] = []string{}
	for _, word := range body {
		if !strings.HasPrefix(word, "href=\"") {
			continue
		}
		wg.Add(1)
		go func(word string) {
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
				if recursive {
					_, ok := cache.Load(page.url)
					if !ok && !check_exsistence(client, corrected) {
						cache.Store(page.url, true)
						wg.Add(1)
						go execute(corrected, client, wg)
					}
				}
			}
		}(word)
	}
	//out, _ := json.MarshalIndent(page.seen, "", "  ")
	//fmt.Print(string(out))
	doc := map[string]any{
		"url":     page.url,
		"seen":    page.seen[page.url],
		"time":    time.Now().Format(time.DateOnly),
		"summary": summarizePage(page.content),
	}
	fmt.Print(doc["summary"])
	fmt.Println("Related pages: ")
	for i := 0; i < len(page.seen[page.url]); i++ {
		fmt.Printf("%d. %s\n", i+1, page.seen[page.url][i])
	}
	mu.Lock()
	defer mu.Unlock()
	if !update {
		coll := client.Database("crawler").Collection("links")
		_, err := coll.InsertOne(context.TODO(), doc)
		if err != nil {
			log.Fatal(err)
		}
	} else {
		filter := bson.D{bson.E{Key: "url", Value: page.url}}
		update := bson.D{
			{Key: "$set", Value: bson.D{
				{Key: "seen", Value: page.seen[page.url]},
				{Key: "time", Value: time.Now().Format(time.DateOnly)},
			}},
		}
		coll := client.Database("crawler").Collection("links")
		_, err := coll.UpdateOne(context.TODO(), filter, update)
		if err != nil {
			log.Fatal(err)
		}
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

func execute(target string, client *mongo.Client, wg *sync.WaitGroup) {
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
	page.getLinks(client, wg)
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
			urlStr, ok := results[0]["url"].(string)
			if !ok {
				log.Println("Type assertion for url failed")
				return
			}
			data, err := bson.MarshalExtJSON(bson.M{urlStr: results[0]["seen"]}, false, false)
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
				update = true
			}
		}
	}
	var wg sync.WaitGroup
	wg.Add(1)
	go execute(startURL, client, &wg)
	wg.Wait()
}

func init() {
	command.Flags().BoolVarP(&update, "update", "u", false, "Update existing links")
	command.Flags().BoolVarP(&recursive, "recursive", "r", true, "Recursively crawl links")
	rootCmd.AddCommand(command)
}
