package spork_test

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/sporkops/spork-go"
)

func ExampleNewClient() {
	client := spork.NewClient(
		spork.WithAPIKey(os.Getenv("SPORK_API_KEY")),
	)

	account, err := client.GetAccount(context.Background())
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("Logged in as %s (%s plan)\n", account.Email, account.Plan)
}

func ExampleClient_CreateMonitor() {
	client := spork.NewClient(spork.WithAPIKey("sk_live_..."))

	monitor, err := client.CreateMonitor(context.Background(), &spork.Monitor{
		Name:           "API Health",
		Target:         "https://api.example.com/health",
		Interval:       60,
		ExpectedStatus: 200,
		Regions:        []string{"us-central1", "europe-west1"},
	})
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("Created monitor %s: %s\n", monitor.ID, monitor.Name)
}

func ExampleClient_ListMonitors() {
	client := spork.NewClient(spork.WithAPIKey("sk_live_..."))

	monitors, err := client.ListMonitors(context.Background())
	if err != nil {
		log.Fatal(err)
	}
	for _, m := range monitors {
		fmt.Printf("%s: %s (%s)\n", m.ID, m.Name, m.Status)
	}
}

func ExampleClient_CreateAlertChannel() {
	client := spork.NewClient(spork.WithAPIKey("sk_live_..."))

	channel, err := client.CreateAlertChannel(context.Background(), &spork.AlertChannel{
		Name: "On-Call Email",
		Type: "email",
		Config: map[string]string{
			"to": "oncall@example.com",
		},
	})
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("Created channel %s\n", channel.ID)
}

func ExampleClient_CreateStatusPage() {
	client := spork.NewClient(spork.WithAPIKey("sk_live_..."))

	page, err := client.CreateStatusPage(context.Background(), &spork.StatusPage{
		Name:     "Acme Status",
		Slug:     "acme-status",
		IsPublic: true,
		Theme:    "light",
		Components: []spork.StatusComponent{
			{MonitorID: "mon_abc", DisplayName: "API", Order: 0},
			{MonitorID: "mon_def", DisplayName: "Website", Order: 1},
		},
	})
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("Status page: https://%s.status.sporkops.com\n", page.Slug)
}

func ExampleIsNotFound() {
	client := spork.NewClient(spork.WithAPIKey("sk_live_..."))

	_, err := client.GetMonitor(context.Background(), "mon_nonexistent")
	if spork.IsNotFound(err) {
		fmt.Println("Monitor does not exist")
	}
}
