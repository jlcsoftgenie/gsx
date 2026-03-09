package main

import (
	"log"
	"os"
)

type ProfileData struct {
	Title       string
	Description string
	Email       string
	Role        string
}

func main() {
	data := ProfileData{
		Title:       "Account Overview",
		Description: "Named slots and nested layouts compiled ahead of time.",
		Email:       "operator@example.com",
		Role:        "Administrator",
	}
	if err := RenderProfilePage(os.Stdout, data); err != nil {
		log.Fatal(err)
	}
}
