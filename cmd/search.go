package cmd

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/exec"
	"sort"
	"strings"

	"github.com/joho/godotenv"
	"github.com/spf13/cobra"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.mongodb.org/mongo-driver/v2/bson"
)

var searchCommand = &cobra.Command{
	Use:   "search",
	Short: "Crawl website for redirecting links",
	Long:  "Crawl website for redirecting links recursively with concurrency limits.",
	Run: func(cmd *cobra.Command, args []string) {
		search(cmd, args[0])
	},
	Args: cobra.ExactArgs(1),
}

type Document struct {
	ID         bson.ObjectID  `bson:"id"`
	URL        string         `bson:"url"`
	Seen       []string       `bson:"seen"`
	Time       string         `bson:"time"`
	References []string       `bson:"references"`
	Index      map[string]int `bson:"index"`
}

func standardizeInput(query string) []string {
	jsonData, _ := json.Marshal(strings.Fields(query))
	var pythonOut bytes.Buffer
	var pythonErr bytes.Buffer
	var pythonRes []string
	cmd := exec.Command("python", "cmd/standardize.py")
	cmd.Stdin = bytes.NewReader(jsonData)
	cmd.Stderr = &pythonErr
	cmd.Stdout = &pythonOut
	if err := cmd.Run(); err != nil {
		fmt.Printf("Python STDERR:\n%s\n", pythonErr.String())
		return strings.Fields(query)
	}
	trimmedOutput := bytes.TrimSpace(pythonOut.Bytes())
	if err := json.Unmarshal(trimmedOutput, &pythonRes); err != nil {
		return strings.Fields(query)
	}
	return pythonRes
}

func Contains(slice []string, val string) bool {
	for _, item := range slice {
		if item == val {
			return true
		}
	}
	return false
}

func Count(slice []string, words map[string]any) int {
	result := 0
	for _, item := range slice {
		// fmt.Println(item, words[item])
		if value, ok := words[item]; ok {
			if num, ok := value.(float64); ok {
				result += int(num)
			}
		}
	}
	return result
}

func sortSimilarities(urls map[string]int) []string {
	var keys []string
	for k := range urls {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var result []string
	maxMatches := 0
	for _, k := range keys {
		result = append(result, k)
		maxMatches++
		if maxMatches > 10 {
			break
		}
	}
	return result
}

func queryDatabase(tokenized []string) []string {
	if err := godotenv.Load(); err != nil {
		log.Println(err)
		log.Println("No .env file found — using system environment variables")
	}
	uri := os.Getenv("MONGO_URI")
	client, err := mongo.Connect(context.TODO(), options.Client().ApplyURI(uri))
	if err != nil {
		log.Fatal("Failed to connect to MongoDB: ", err)
		return nil // Return empty results
	}
	defer client.Disconnect(context.TODO())
	coll := client.Database("crawler").Collection("links")
	var results []Document
	filter := bson.M{}
	cursor, err := coll.Find(context.TODO(), filter)
	if err != nil {
		log.Fatal("Failed to execute Find operation: ", err)
		return nil // Return empty results
	}
	defer cursor.Close(context.TODO())
	if err := cursor.All(context.TODO(), &results); err != nil {
		panic(err)
	}
	urls := make(map[string]int, 10)
	for _, result := range results {
		res, _ := bson.MarshalExtJSON(result, false, false)
		var jsonMap map[string]any
		if err := json.Unmarshal([]byte(res), &jsonMap); err == nil {
			if url, ok := jsonMap["url"].(string); ok {
				simmilarity := 0
				if words, ok := jsonMap["index"].(map[string]any); ok {
					simmilarity += Count(tokenized, words)
					if simmilarity != 0 {
						if list, ok := jsonMap["references"].([]string); ok {
							references := len(list)
							urls[url] = simmilarity + references
						} else {
							urls[url] = simmilarity
						}
					}
				}
			}
		}

	}
	mostSimilar := sortSimilarities(urls) // get top 10
	// fmt.Println(mostSimilar)
	return mostSimilar
}

func search(_ *cobra.Command, query string) {
	tokenized := standardizeInput(query)
	results := queryDatabase(tokenized)
	if len(results) > 0 {
		fmt.Println("----------- Search Results -----------")
		for _, url := range results {
			fmt.Println("\t", url)
		}
	} else {
		fmt.Println("----------- No search results found :( -----------")
	}
}
