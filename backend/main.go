package main
import (
	"fmt"
	"os"
	"log"
	"net/http"
	"github.com/joho/godotenv"
)

func homepage(w http.ResponseWriter, r *http.Request){
	fmt.Fprintf(w, "Hi there, I love %s!", r.URL.Path[1:])
}

func main(){
	if err := godotenv.Load(); err != nil {
		log.Println(err)
		log.Println("No .env file found — using system environment variables")
	}
	uri := os.Getenv("MONGO_URI")
	fmt.Println(uri)
	http.HandleFunc("/",homepage)
	log.Fatal(http.ListenAndServe(":8080",nil))


}
