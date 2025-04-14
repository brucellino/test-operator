package main

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"

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

	if err := nomad.ValidateNomadConnection(client); err != nil {
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

	// The initial critical tests are executed by StartCriticalTests()
	// eventConsumer.StartCriticalTests()

	// We then start an event consumer to watch for the job to finish
	eventConsumer.StartEventConsumer()
	return nil
} // run
