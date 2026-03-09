package main

import (
	"log"
	"os"
)

type Metric struct {
	Label string
	Value string
}

type DashboardData struct {
	Title   string
	Metrics []Metric
}

func main() {
	data := DashboardData{
		Title: "Operations Dashboard",
		Metrics: []Metric{
			{Label: "Active Sessions", Value: "128"},
			{Label: "Pending Jobs", Value: "17"},
			{Label: "Error Rate", Value: "0.02%"},
		},
	}
	if err := RenderDashboardPage(os.Stdout, data); err != nil {
		log.Fatal(err)
	}
}
