package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"github.com/gorilla/mux"
	"github.com/joho/godotenv"
	"log"
	"math"
	"net/http"
	"os"
	"regexp"
	"time"
)

// .env config struct
type Config struct {
	driversUrl       string
	directionsUrl    string
	driversApiKey    string
	directionsApiKey string
	port             string
}

// Driver struct, ID ommitted because it is not needed
type Driver struct {
	Name string
	Rate float64
}

// Journey information struct
type Journey struct {
	Start string
	End   string
}

// Google Directions API data struct
type Directions struct {
	Routes []struct {
		Legs []struct {
			Distance struct {
				Value float64 `json:"value"`
			} `json:"distance"`
			Steps []struct {
				Distance struct {
					Value float64 `json:"value"`
				} `json:"distance"`
				Html_instructions string `json:"html_instructions"`
			} `json:"steps"`
		} `json:"legs"`
	} `json:"routes"`
}

// Response struct
type Response struct {
	FinalRate float64
	Cost      float64
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
	config.driversUrl = GetEnvVar("DRIVERSURL")
	config.directionsUrl = GetEnvVar("DIRECTIONSURL")
	config.driversApiKey = GetEnvVar("DRIVERSAPIKEY")
	config.directionsApiKey = GetEnvVar("DIRECTIONSAPIKEY")
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
	router.HandleFunc("/esr_riders", RiderRequest).Methods("GET")
	log.Fatal(http.ListenAndServe(":5421", router))
}

// Handles GET requests, responds with the best driver and cost for the given journey
// The HTTP request must contain a 'Journey' structured JSON in the body with a start and end point
func RiderRequest(w http.ResponseWriter, r *http.Request) {
	decoder := json.NewDecoder(r.Body)
	var journey Journey
	var driver Driver
	var numDrivers int
	var cost float64
	var finalRate float64
	//Get journey data from request body
	if err := decoder.Decode(&journey); err == nil {
		if drv, num, err1 := FindBestDriver(); err1 == nil {
			if drv.Name != "" {
				driver = drv
				numDrivers = num
			} else {
				// if driver name is empty, then no drivers must be available as esr_drivers does not allow empty names
				w.Write([]byte("No drivers available at this time."))
				log.Print("Tried finding best driver, but none available.")
				return
			}
		} else {
			w.WriteHeader(http.StatusInternalServerError)
			log.Print(err1)
			return
		}
	} else {
		w.WriteHeader(http.StatusBadRequest)
		log.Print(err)
		return
	}
	// After getting the best driver, calculate the cost of the journey
	if cst, final, err := CalculateCost(journey, driver, numDrivers); err == nil {
		cost = cst
		finalRate = final
	} else {
		w.WriteHeader(http.StatusInternalServerError)
		log.Print(err)
		return
	}
	// create a response object
	response := Response{
		FinalRate: finalRate,
		Cost:      cost,
	}
	// respond
	if enc, err := json.Marshal(response); err == nil {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(enc))
	} else {
		w.WriteHeader(http.StatusInternalServerError)
		log.Print(err)
	}
}

// Finds the best driver currently available in the roster by requesting esr_drivers
// This func will use the 'driversurl' and 'driversApiKey' to send an HTTP request to the drivers microservice
// It will then return the driver with the lowest rate, or an empty driver object if no drivers are available
func FindBestDriver() (Driver, int, error) {
	client := &http.Client{}
	var driver Driver
	if req, err1 := http.NewRequest("GET", config.driversUrl, nil); err1 == nil {
		req.Header.Set("x-api-key", config.driversApiKey)
		if resp, err2 := client.Do(req); err2 == nil {
			var drivers map[string]Driver
			decoder := json.NewDecoder(resp.Body)
			if err3 := decoder.Decode(&drivers); err3 == nil {
				// Find lowest rate
				lowest := math.Inf(1)
				for _, curDriver := range drivers {
					if curDriver.Rate < lowest {
						lowest = curDriver.Rate
						driver = curDriver
					}
				}
				return driver, len(drivers), nil
			} else {
				return driver, 1, errors.New("Error: Response body decoding failed")
			}
		} else {
			return driver, 1, errors.New("Error: HTTP request sending gailed")
		}
	} else {
		return driver, 1, errors.New("Error: HTTP request creation gailed")
	}
}

// Calculates the cost of the given journey with a given driver
// This func will use the 'directionsurl' and 'directionsApiKey' to send an HTTP request to the directions microservice
// It returns the cost of the journey in GBP
func CalculateCost(journey Journey, driver Driver, numDrivers int) (float64, float64, error) {
	client := &http.Client{}
	var directions Directions
	// Get directions data from durections microservice
	if enc, err := json.Marshal(journey); err == nil {
		if req, err1 := http.NewRequest("GET", config.directionsUrl, bytes.NewBuffer(enc)); err1 == nil {
			req.Header.Set("x-api-key", config.directionsApiKey)
			if resp, err2 := client.Do(req); err2 == nil {
				decoder := json.NewDecoder(resp.Body)
				if err3 := decoder.Decode(&directions); err3 != nil {
					log.Print(err3)
					return 0, 0, errors.New("Error: Response body decoding failed")
				}
			} else {
				return 0, 0, errors.New("Error: HTTP request sending gailed")
			}
		} else {
			return 0, 0, errors.New("Error: HTTP request creation gailed")
		}
	} else {
		return 0, 0, errors.New("Error: Could not create JSON from journey")
	}
	if len(directions.Routes) == 0 || len(directions.Routes[0].Legs) == 0 {
		return 0, 0, errors.New("Error: No Route Found")
	}
	// calculate rate multiplier
	multiplier := 1.0
	// criteria 1
	aRoadTotal := 0.0
	for _, step := range directions.Routes[0].Legs[0].Steps {
		if matched, err := regexp.Match("A(\\d)+", []byte(step.Html_instructions)); matched && err == nil {
			aRoadTotal += step.Distance.Value
		} else if err != nil {
			return 0, 0, err
		}
	}
	if aRoadTotal > directions.Routes[0].Legs[0].Distance.Value/2 {
		multiplier *= 2
	}
	// criteria 2
	if numDrivers < 5 {
		multiplier *= 2
	}
	// criteria 3
	if hour, _, _ := time.Now().Clock(); hour > 22 || hour < 6 {
		multiplier *= 2
	}
	cost := directions.Routes[0].Legs[0].Distance.Value / 1000.0 * driver.Rate * multiplier
	log.Printf("Calculated journey. Driver: %s, Distance: %gm, Rate: £%g/km, Multiplier: %g, Final Rate: £%.2f/km, Cost: £%.2f",
		driver.Name, directions.Routes[0].Legs[0].Distance.Value, driver.Rate, multiplier, multiplier*driver.Rate, cost)
	return cost, multiplier * driver.Rate, nil
}
