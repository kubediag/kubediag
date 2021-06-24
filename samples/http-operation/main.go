package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"time"

	"github.com/gorilla/mux"
)

var (
	cache = map[string]string{
		"a": "1",
		"b": "2",
		"c": "3",
		"d": "4",
	}
)

func main() {
	address := flag.String("address", "0.0.0.0", "The address on which to advertise.")
	port := flag.String("port", "80", "The port to serve on.")
	flag.Parse()

	r := mux.NewRouter()
	r.HandleFunc("/", handler)
	srv := &http.Server{
		Handler:      r,
		Addr:         fmt.Sprintf("%s:%s", *address, *port),
		WriteTimeout: 30 * time.Second,
		ReadTimeout:  30 * time.Second,
	}

	log.Fatal(srv.ListenAndServe())
}

func handler(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case "POST":
		// Parse the request payload into a map[string]string.
		body, err := ioutil.ReadAll(r.Body)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		parameters := make(map[string]string)
		err = json.Unmarshal(body, &parameters)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		// Update cache with parameters.
		for key, value := range parameters {
			log.Printf("Update cache with %s:%s", key, value)
			cache[key] = value
		}
		data, err := json.Marshal(cache)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		// Response with cache.
		w.Header().Set("Content-Type", "application/json")
		w.Write(data)
	default:
		http.Error(w, fmt.Sprintf("method %s is not supported", r.Method), http.StatusMethodNotAllowed)
	}
}
