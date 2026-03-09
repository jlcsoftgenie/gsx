package main

import (
	"io"
	"log"
	"net/http"
)

type User struct {
	ID    int
	Name  string
	Email string
	Role  string
}

func main() {
	users := seedUsers()
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		writeHTML(w, func(dst io.Writer) error {
			return RenderHomePage(dst, "GSX Webserver Example", "A small multi-route net/http app rendered with compiled GSX templates.", users)
		})
	})
	mux.HandleFunc("/users", func(w http.ResponseWriter, r *http.Request) {
		writeHTML(w, func(dst io.Writer) error {
			return RenderUsersPage(dst, "User Directory", users)
		})
	})
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		_, _ = io.WriteString(w, "ok\n")
	})
	log.Println("webserver example listening on :8080")
	log.Fatal(http.ListenAndServe(":8080", mux))
}

func writeHTML(w http.ResponseWriter, render func(io.Writer) error) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := render(w); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func seedUsers() []User {
	return []User{
		{ID: 1, Name: "Ada Lovelace", Email: "ada@example.com", Role: "Founder"},
		{ID: 2, Name: "Grace Hopper", Email: "grace@example.com", Role: "Compiler Engineer"},
		{ID: 3, Name: "Margaret Hamilton", Email: "margaret@example.com", Role: "Flight Software"},
		{ID: 4, Name: "Barbara Liskov", Email: "barbara@example.com", Role: "Language Design"},
	}
}
