package api

import (
	"encoding/json"
	"github.com/gorilla/mux"
	"net/http"
)

// StartAPI initializes and starts the HTTP API server
func StartAPI() {
	r := mux.NewRouter()

	r.HandleFunc("/posts", getPostsHandler).Methods("GET")
	r.HandleFunc("/bookmark", bookmarkPostHandler).Methods("POST")

	http.ListenAndServe(":8080", r)
}

func getPostsHandler(w http.ResponseWriter, r *http.Request) {
	// Authentication and fetching posts logic here
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode([]string{"Post 1", "Post 2"})
}

func bookmarkPostHandler(w http.ResponseWriter, r *http.Request) {
	// Authentication and bookmarking logic here
	w.WriteHeader(http.StatusCreated)
}
