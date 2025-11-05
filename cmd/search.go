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

var search_command = &cobra.Command{
	Use:   "search",
	Short: "Crawl website for redirecting links",
	Long:  "Crawl website for redirecting links recursively with concurrency limits.",
	Run: func(cmd *cobra.Command, args []string) {
		search(cmd, args[0])
	},
	Args: cobra.ExactArgs(1),
}

type Document struct {
	ID    bson.ObjectID  `bson:"id"`
	URL   string         `bson:"url"`
	Seen  []string       `bson:"seen"`
	Time  string         `bson:"time"`
	Index map[string]int `bson:"index"`
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

func Contains(slice []string, val string) bool {
	for _, item := range slice {
		if item == val {
			return true
		}
	}
	return false
}

func Count(slice []string, val string) int {
	result := 0
	for _, item := range slice {
		if item == val {
			result++
		}
	}
	return result
}

func sort_similarities(urls map[string]int) []string {
	keys := make([]string, 0, len(urls))
	for k := range urls {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var result []string
	max_matches := 0
	for _, k := range keys {
		result = append(result, k)
		max_matches++
		if max_matches > 10 {
			break
		}
	}
	return result
}

func query_database(tokenized []string) []string {
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
		var jsonMap map[string]interface{}
		json.Unmarshal([]byte(res), &jsonMap)
		if url, ok := jsonMap["url"].(string); ok {
			simmilarity := 0
			if urlValue, ok := jsonMap["index"].(string); ok {
				simmilarity += Count(tokenized, urlValue)
			}
			urls[url] = simmilarity
		}
	}
	most_similar := sort_similarities(urls) //get top 10
	return most_similar
}

func search(_ *cobra.Command, query string) {
	tokenized := standardize_input(query)
	results := query_database(tokenized)
	fmt.Println("-----------Search Results-----------")
	for url := range results {
		fmt.Println("\t", url)
	}
}
