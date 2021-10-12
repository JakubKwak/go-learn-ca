package main

import (
	"encoding/json"
	"github.com/gorilla/mux"
	"github.com/joho/godotenv"
	"io/ioutil"
	"log"
	"net/http"
	"os"
)

// .env config struct
type Config struct {
	apiKey  string
	authKey string
	authUrl string
	port    string
}

// Driver struct
type Driver struct {
	Id   string
	Name string
	Rate float64
}

// Drivers map to hold the current roster of drivers
var drivers map[string]Driver

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
	drivers = make(map[string]Driver)
	handleRequests()
}

// Gets environment variables and store them in config struct
func GetEnv() {
	config.apiKey = GetEnvVar("APIKEY")
	config.authKey = GetEnvVar("AUTHKEY")
	config.authUrl = GetEnvVar("AUTHURL")
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
	router.HandleFunc("/esr_drivers", Create).Methods("POST")
	router.HandleFunc("/esr_drivers/{user}", Read).Methods("GET")
	router.HandleFunc("/esr_drivers", ReadAll).Methods("GET")
	router.HandleFunc("/esr_drivers", Update).Methods("PUT")
	router.HandleFunc("/esr_drivers/{user}", Delete).Methods("DELETE")
	log.Fatal(http.ListenAndServe(":"+config.port, router))
}

// Handles POST requests, creates a driver and adds them to the roster
// Request must have a valid x-api-key in header which corresponds to an existing user,
// and a 'Driver' structured JSOn in the body
// Responds with StatusCreated if successful
func Create(w http.ResponseWriter, r *http.Request) {
	// user Api Key authentication
	var userId string
	if id, err := Authenticate(r.Header.Get("x-api-key")); err == nil {
		if id == "" {
			w.WriteHeader(http.StatusBadRequest)
			log.Print("Error: Invalid user API key")
			return
		}
		userId = id
	} else {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	var driver Driver
	decoder := json.NewDecoder(r.Body)
	if err := decoder.Decode(&driver); err == nil {
		if driver.Name != "" {
			driver.Id = userId
			index := driver.Name
			if _, ok := drivers[index]; !ok {
				w.WriteHeader(http.StatusCreated)
				drivers[index] = driver
				log.Print("Created driver: " + index)
			} else {
				w.WriteHeader(http.StatusBadRequest)
			}
		} else {
			w.WriteHeader(http.StatusBadRequest)
		}
	} else {
		w.WriteHeader(http.StatusBadRequest)
	}
}

// Handles some GET requests, responding with a specific driver
// Request must have a valid 'x-api-key' in header
// Responds with the driver corresponding with drivername in request url
func Read(w http.ResponseWriter, r *http.Request) {
	// service Api Key authentication
	if r.Header.Get("x-api-key") != config.apiKey {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	vars := mux.Vars(r)
	user := vars["user"]
	if driver, ok := drivers[user]; ok {
		if enc, err := json.Marshal(driver); err == nil {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(enc))
			log.Print("Read driver: " + user)
		} else {
			w.WriteHeader(http.StatusInternalServerError)
		}
	} else {
		w.WriteHeader(http.StatusNotFound)
	}
}

// Handles some GET requests, responding with an array of all drivers
// Request must have a valid 'x-api-key' in header
// Responds with an array of all drivers currently in the roster
func ReadAll(w http.ResponseWriter, r *http.Request) {
	// service Api Key authentication
	if r.Header.Get("x-api-key") != config.apiKey {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	if enc, err := json.Marshal(drivers); err == nil {
		w.Write([]byte(enc))
		log.Print("Read all drivers")
	} else {
		w.WriteHeader(http.StatusInternalServerError)
	}
}

// Handles PUT requests, updating driver rates
// Request must have a 'Driver' structured Json in the body
// Only performs the update if the user id from the key matches the id of the
// driver being edited.a
// Responds with StatusOK if successful
func Update(w http.ResponseWriter, r *http.Request) {
	// user Api Key authentication
	var userId string
	if id, err := Authenticate(r.Header.Get("x-api-key")); err == nil {
		if id == "" {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		userId = id
	} else {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	var driver Driver
	decoder := json.NewDecoder(r.Body)
	if err := decoder.Decode(&driver); err == nil {
		driver.Id = userId
		index := driver.Name
		if drv, ok := drivers[index]; ok && drv.Id == userId {
			w.WriteHeader(http.StatusOK)
			drivers[index] = driver
			log.Print("Updated driver: " + index)
		} else {
			w.WriteHeader(http.StatusBadRequest)
		}
	} else {
		w.WriteHeader(http.StatusBadRequest)
	}
}

// Handles DELETE requests, deleting a driver from the roster
// The request must have a valid 'x-api-key' in the header and a valid driver name in url
// Only performs the request if the api key corresponds to the user who created the driver
// Reponds with StatusOK if successful
func Delete(w http.ResponseWriter, r *http.Request) {
	// user Api Key authentication
	var userId string
	if id, err := Authenticate(r.Header.Get("x-api-key")); err == nil {
		if id == "" {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		userId = id
	} else {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	vars := mux.Vars(r)
	user := vars["user"]
	if driver, ok := drivers[user]; ok && driver.Id == userId {
		w.WriteHeader(http.StatusOK)
		delete(drivers, user)
		log.Print("Deleted driver: " + user)
	} else {
		w.WriteHeader(http.StatusBadRequest)
	}
}

// Sends request to esr_auth service and returns user ID which matches the key
// Returns empty string if no users match the key
func Authenticate(key string) (string, error) {
	client := &http.Client{}
	url := config.authUrl + "/" + key
	if req, err1 := http.NewRequest("GET", url, nil); err1 == nil {
		req.Header.Set("x-api-key", config.authKey)
		if resp, err2 := client.Do(req); err2 == nil {
			if body, err3 := ioutil.ReadAll(resp.Body); err3 == nil {
				return string(body), nil
			} else {
				return "", err3
			}
		} else {
			return "", err2
		}
	} else {
		return "", err1
	}
}
