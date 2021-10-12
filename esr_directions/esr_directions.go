package main

import (
	"encoding/json"
	"github.com/gorilla/mux"
	"github.com/joho/godotenv"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"strings"
)

// .env config struct
type Config struct {
	googleKey string
	googleUrl string
	apiKey    string
	port      string
}

// Journey information struct
type Journey struct {
	Start string
	End   string
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

// Listens to and handles incoming HTTP requests
func handleRequests() {
	router := mux.NewRouter().StrictSlash(true)
	router.HandleFunc("/esr_directions", GetDirections).Methods("GET")
	log.Fatal(http.ListenAndServe(":"+config.port, router))
}

// Gets environment variables and store them in config struct
func GetEnv() {
	config.googleKey = GetEnvVar("GOOGLEKEY")
	config.googleUrl = GetEnvVar("GOOGLEURL")
	config.apiKey = GetEnvVar("APIKEY")
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

// Handles GET requests, responding with directions
// Request must have a valid 'x-api-key' in header and a 'Journey' structured JSON in body
// This func uses the 'googleURL' and 'googleKey' env vars to send an HTTP GET request to Google's
// Directions API. It then responds with the received JSON.
func GetDirections(w http.ResponseWriter, r *http.Request) {
	if r.Header.Get("x-api-key") != config.apiKey {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	decoder := json.NewDecoder(r.Body)
	var journey Journey
	if err := decoder.Decode(&journey); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	// remove any spaces in the location names
	start := strings.ReplaceAll(journey.Start, " ", "_")
	end := strings.ReplaceAll(journey.End, " ", "_")
	// get directions
	url := config.googleUrl + "?origin=" + start + "&destination=" + end + "&key=" + config.googleKey
	client := &http.Client{}
	if req, err1 := http.NewRequest("GET", url, nil); err1 == nil {
		if resp, err2 := client.Do(req); err2 == nil {
			if body, err3 := ioutil.ReadAll(resp.Body); err3 == nil {
				w.Write([]byte(body))
				log.Print("Directions request complete: " + start + " to " + end)
			} else {
				w.WriteHeader(http.StatusInternalServerError)
			}
		} else {
			w.WriteHeader(http.StatusInternalServerError)
		}
	} else {
		w.WriteHeader(http.StatusInternalServerError)
	}
}
