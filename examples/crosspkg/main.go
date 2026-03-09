package main

import (
	"log"
	"os"
)

func main() {
	if err := RenderHomePage(os.Stdout, "Cross-Package Components", "Imported GSX components resolve through normal Go imports."); err != nil {
		log.Fatal(err)
	}
}
