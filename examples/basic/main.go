package main

import (
	"log"
	"os"
)

type User struct {
	Name      string
	Email     string
	AvatarURL string
}

func main() {
	users := []User{
		{Name: "Ada Lovelace", Email: "ada@example.com", AvatarURL: "https://example.com/ada.png"},
		{Name: "Grace Hopper", Email: "grace@example.com", AvatarURL: "https://example.com/grace.png"},
	}
	if err := RenderUsersPage(os.Stdout, "Team Directory", users); err != nil {
		log.Fatal(err)
	}
}
