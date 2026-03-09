package main

import (
	"log"
	"net/http"
)

type User struct {
	ID    int
	Name  string
	Email string
}

func main() {
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		if err := RenderUsersPage(w, seedUsers()); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	})
	mux.HandleFunc("/users", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		if err := RenderUsersTable(w, seedUsers()); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	})
	log.Println("listening on :8080")
	log.Fatal(http.ListenAndServe(":8080", mux))
}

func seedUsers() []User {
	return []User{
		{ID: 1, Name: "Ada Lovelace", Email: "ada@example.com"},
		{ID: 2, Name: "Grace Hopper", Email: "grace@example.com"},
		{ID: 3, Name: "Margaret Hamilton", Email: "margaret@example.com"},
	}
}
