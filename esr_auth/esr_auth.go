package main

import (
	"encoding/json"
	"github.com/gorilla/mux"
	"github.com/joho/godotenv"
	"log"
	"net/http"
	"os"
)

// .env config struct
type Config struct {
	apiKey    string
	usersPath string
	port      string
}

// User struct for loading users from json
type User struct {
	Id  string
	Key string
}

// config struct to hold URLs and API keys
var config Config

// invoked before main
func init() {
	if err := godotenv.Load(); err != nil {
		log.Fatal("No .env file found")
	}
}

func main() {
	GetEnv()
	handleRequests()
}

// Gets environment variables and store them in config struct
func GetEnv() {
	config.apiKey = GetEnvVar("APIKEY")
	config.usersPath = GetEnvVar("USERSPATH")
	config.port = GetEnvVar("PORT")
}

// Gets a specific var from env, or log fatal if missing
func GetEnvVar(varName string) string {
	if value, exists := os.LookupEnv(varName); exists {
		return value
	} else {
		log.Fatal("Error: " + varName + " missing in config")
		return ""
	}
}

// Listens to and handles incoming HTTP requests
func handleRequests() {
	router := mux.NewRouter().StrictSlash(true)
	router.HandleFunc("/esr_auth/{key}", Authenticate).Methods("GET")
	log.Fatal(http.ListenAndServe(":"+config.port, router))
}

// Responds to HTTP GET requests, responding with the user ID which matches the given key
// The HTTP request must contain the key to authenticate in the address
// It would be more appropiate to store user data in a database rather than a JSON file, but that is outside
// of the scope of this CA, and a JSON file is sufficient to demonstrate the functionality of this service.
func Authenticate(w http.ResponseWriter, r *http.Request) {
	if r.Header.Get("x-api-key") != config.apiKey {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	vars := mux.Vars(r)
	key := vars["key"]
	usersFile, err := os.Open(config.usersPath)
	if err != nil {
		log.Print(err)
		w.WriteHeader(http.StatusInternalServerError)
		w.Write(nil)
		return
	}
	var users []User
	usersDecoder := json.NewDecoder(usersFile)
	if err = usersDecoder.Decode(&users); err != nil {
		log.Print(err)
		w.WriteHeader(http.StatusInternalServerError)
		w.Write(nil)
		return
	}
	var id string
	for _, user := range users {
		if user.Key == key {
			id = user.Id
			break
		}
	}
	if id != "" {
		log.Print("User authenticated: " + id)
	}
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(id))
}
