package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/brucellino/test-operator/pkg/nomad"
	"github.com/charmbracelet/log"
	"github.com/hashicorp/nomad/api"
)

func main() {
	if err := run(os.Args[:]); err != nil {
		log.Fatal(err)
	}
}

func run(args []string) error {
	// The run function starts a Nomad API client with relevant configuration.
	// We need the following environment variables in order to authenticate with the Nomad API:
	// - NOMAD_ADDR: The address of the Nomad server.
	// - NOMAD_TOKEN: The token to use for authentication.
	// - NOMAD_REGION: The region to use for the Nomad API.
	requireEnvVars := []string{"NOMAD_ADDR", "NOMAD_TOKEN"}

	for _, envVar := range requireEnvVars {
		if os.Getenv(envVar) == "" {
			log.Fatalf("environment variable %s is not set", envVar)
		}
	}

	// create the Nomad client
	client, err := api.NewClient(&api.Config{Address: os.Getenv("NOMAD_ADDR"), SecretID: os.Getenv("NOMAD_TOKEN")})

	log.Info("New Client created")
	if err != nil {
		return fmt.Errorf("failed to create Nomad client: %w", err)
	}

	if err := validateNomadConnection(client); err != nil {
		log.Errorf("Failed to validate Nomad connection: %v", err)
		return fmt.Errorf("failed to validate Nomad connection %w", err)
	}

	log.Info("Nomad connection valid.")

	log.Info("Starting Consumer")
	// operator was stepup
	operator, err := nomad.NewOperator(client)
	if err != nil {
		return fmt.Errorf("Failed to create operator %w", err)
	}
	// consumer := NewAllocationConsumer(client, operator.onAllocation())
	log.Infof("Operator is %v", *operator)

	// Make test parts - critical, Inner, outer, full

	// Make a new Job Consumer
	eventConsumer := nomad.NewEventConsumer(client, operator.OnEvent)
	// shutdown part
	signals := make(chan os.Signal, 1)
	signal.Notify(signals, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		// receive any signals
		s := <-signals
		log.Infof("Received signal %s. Stopping\n", s)
		eventConsumer.StopEventConsumer()
		os.Exit(0)
	}()
	eventConsumer.StartEventConsumer()
	return nil
} // run

// validateNomadConnection is a utility private function to check that the Nomad client can interact with the Nomad API.
func validateNomadConnection(client *api.Client) error {
	// try to list nodes - if there are no nodes, then the api connection is not valid.
	nodes, _, err := client.Nodes().List(&api.QueryOptions{})

	if err != nil {
		log.Errorf("Failed to list nodes: %v", err)
		return fmt.Errorf("Failed to list nodes: %w", err)
	}

	if len(nodes) == 0 {
		log.Error("No nodes found - check if NOMAD_TOKEN has correct permissions or is set.")
		return fmt.Errorf("no nodes found - check if NOMAD_TOKEN has correct permissions or is set")
	}

	// see if we can access all topics
	topics := map[api.Topic][]string{
		api.TopicNode: {"*"},
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
	defer cancel()

	eventsCh, err := client.EventStream().Stream(ctx, topics, 0, &api.QueryOptions{})

	if err != nil {
		log.Errorf("Failed to validate nomad event stream: %v", err)
		return fmt.Errorf("Failed to validate nomad nomad event stream: %w", err)
	}

	// wait for at least one event, even a heartbeat
	select {
	case event := <-eventsCh:
		if event.Err != nil {
			log.Errorf("Failed to validate nomad event stream: %v", err)
			return fmt.Errorf("Failed to validate nomad nomad event stream: %w", err)
		}

	case <-ctx.Done():
		log.Errorf("Timeout waiting for event stream connection")
		return fmt.Errorf("Timeout waiting for event stream connection")
	}

	return nil
}
