package main

import (
	"context"
	"fmt"
	"os"

	"github.com/PagerDuty/go-pagerduty"
)

type Client struct {
	client *pagerduty.Client
	ctx    context.Context
}

func New(authToken string, ctx context.Context) *Client {
	return &Client{
		client: pagerduty.NewClient(authToken),
		ctx:    ctx,
	}
}

func (p *Client) ListSchedule() (string, error) {
	var lo pagerduty.ListSchedulesOptions
	schedules, err := p.client.ListSchedulesWithContext(p.ctx, lo)
	if err != nil {
		return "", err
	}

	var msg string
	for _, sched := range schedules.Schedules {
		msg += fmt.Sprintf("name: %s, ID: %s\n", sched.Name, sched.ID)
	}

	return msg, nil
}

func (p *Client) GetSchedule(id string) (string, error) {
	schedule, err := p.client.GetScheduleWithContext(p.ctx, id, pagerduty.GetScheduleOptions{})
	if err != nil {
		return "", err
	}

	var msg string
	for _, layer := range schedule.ScheduleLayers {
		msg += fmt.Sprintf("name: %s, ID: %s\n", layer.Name, layer.ID)
	}

	return msg, nil
}

func something() {
	pdToken := os.Getenv("PD_TOKEN")
	if pdToken == "" {
		fmt.Fprintf(os.Stderr, "PD_TOKEN must be set.\n")
		os.Exit(1)
	}

	ctx := context.Background()
	client := New(pdToken, ctx)

	fmt.Println(client.ListSchedule())
	fmt.Println(client.GetSchedule("PSC0CIG"))

}
