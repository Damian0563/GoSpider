package cmd

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/exec"
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
	var urls []string
	for _, result := range results {
		res, _ := bson.MarshalExtJSON(result, false, false)
		var jsonMap map[string]interface{}
		json.Unmarshal([]byte(res), &jsonMap)
		fmt.Println(jsonMap["index"])
		if urlValue, ok := jsonMap["url"].(string); ok {
			urls = append(urls, urlValue)
		}
	}

	return urls
}

func search(_ *cobra.Command, query string) {
	tokenized := standardize_input(query)
	results := query_database(tokenized)
	fmt.Println(results)
}
