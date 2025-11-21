package main
import (
	"fmt"
	"os"
	"log"
	"github.com/joho/godotenv"
)


func main(){
	fmt.Println("test")
	if err := godotenv.Load(); err != nil {
		log.Println(err)
		log.Println("No .env file found — using system environment variables")
	}
	uri := os.Getenv("MONGO_URI")
	fmt.Println(uri)
}
