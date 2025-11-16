package cmd

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/gocolly/colly/v2"
	"github.com/joho/godotenv"
	"github.com/spf13/cobra"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

var (
	sem    = make(chan struct{}, 20)
	update bool
	cache  sync.Map
)

type Page struct {
	url     string
	content string
	seen    map[string][]string
	mu      sync.Mutex
}

func check_exsistence(client *mongo.Client, url string) bool {
	filter := bson.D{
		bson.E{Key: "url", Value: url},
	}
	count, err := client.Database("crawler").Collection("links").CountDocuments(context.TODO(), filter)
	if err != nil {
		return false
	}
	return count > 0
}

func (page *Page) Split() []string {
	var result []string
	c := colly.NewCollector()
	c.OnHTML("a[href]", func(e *colly.HTMLElement) {
		result = append(result, e.Attr("href"))
	})
	c.Visit(page.url)
	c.Wait()
	return result
}

func (page *Page) is_duplicate(url string) bool {
	page.mu.Lock()
	defer page.mu.Unlock()
	return slices.Contains(page.seen[page.url], url)
}

func get_references(url string, ch chan []string, client *mongo.Client) {
	filter := bson.D{
		{Key: "seen", Value: url},
	}
	coll := client.Database("crawler").Collection("links")
	cursor, err := coll.Find(context.TODO(), filter)
	if err != nil {
		log.Fatal(err)
	}
	defer cursor.Close(context.TODO())
	var results []Document
	if err := cursor.All(context.TODO(), &results); err != nil {
		log.Fatal(err)
	}
	var list_ref []string
	for _, result := range results {
		res, _ := bson.MarshalExtJSON(result, false, false)
		var jsonMap map[string]interface{}
		json.Unmarshal([]byte(res), &jsonMap)
		if url, ok := jsonMap["url"].(string); ok {
			list_ref = append(list_ref, url)
		}
	}
	ch <- list_ref
}

func (page *Page) getLinks(client *mongo.Client, wg *sync.WaitGroup) {
	body := page.Split()
	defer wg.Done()
	var innerWG sync.WaitGroup
	page.mu.Lock()
	page.seen[page.url] = []string{}
	page.mu.Unlock()
	ch := make(chan map[string]int, 1)
	innerWG.Add(1)
	go func() {
		defer innerWG.Done()
		indexCollector := colly.NewCollector()
		nlpIndex(indexCollector, ch, page.url)
	}()
	ref_chan := make(chan []string, 1)
	innerWG.Add(1)
	go func() {
		defer innerWG.Done()
		get_references(page.url, ref_chan, client)
	}()
	var references []string = <-ref_chan
	for _, word := range body {
		innerWG.Add(1)
		go func(word string) {
			defer innerWG.Done()
			if strings.HasSuffix(word, ".js") ||
				strings.HasSuffix(word, ".svg") ||
				strings.HasSuffix(word, ".css") ||
				strings.HasSuffix(word, ".json") ||
				strings.HasPrefix(word, "#") ||
				!validateLink(word) {
				return
			}
			_, ok := cache.Load(word)
			if !ok && !check_exsistence(client, word) && !page.is_duplicate(word) {
				page.mu.Lock()
				page.seen[page.url] = append(page.seen[page.url], word)
				page.mu.Unlock()
				cache.Store(word, true)
			}
		}(word)
	}
	innerWG.Wait()
	index := <-ch
	doc := map[string]any{
		"url":        page.url,
		"seen":       page.seen[page.url],
		"time":       time.Now().Format(time.DateOnly),
		"index":      index,
		"references": references,
	}
	fmt.Println("Related pages: ")
	page.mu.Lock()
	for i := 0; i < len(page.seen[page.url]); i++ {
		fmt.Printf("%d. %s\n", i+1, page.seen[page.url][i])
	}
	page.mu.Unlock()
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
	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Get(link)
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	return resp.StatusCode < 400
}

var crawl_command = &cobra.Command{
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
	var results []map[string]any
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
	crawl_command.Flags().BoolVarP(&update, "update", "u", false, "Update existing links")

	rootCmd.AddCommand(searchCommand)
	rootCmd.AddCommand(crawl_command)
}
