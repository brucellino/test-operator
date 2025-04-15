/*
Package operator submits a nomad job and then subsequent jobs, based on events in Nomad.
*/
package main // cmd/package.go

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/brucellino/test-operator/pkg/config"
	"github.com/brucellino/test-operator/pkg/nomad"
	"github.com/charmbracelet/log"
	"github.com/hashicorp/nomad/api"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		log.Fatal(err)
	}
}

func run(args []string) error {
	// The run function starts a Nomad API client with relevant configuration.
	// We need the following environment variables in order to authenticate with the Nomad API:
	// - NOMAD_ADDR: The address of the Nomad server.
	// - NOMAD_TOKEN: The token to use for authentication.
	// - NOMAD_REGION: The region to use for the Nomad API.
	log.Debugf("Launched with arguments: %v", args)

	if len(args) < 5 {
		return fmt.Errorf("usage: %s <job-template-path> <services-config-path> <ticket-id> <primary-service> <affected-services...>", os.Args[0])
	}

	templatePath := args[0]
	servicesConfigPath := args[1]
	ticketID := args[2]
	primaryService := nomad.Service(args[3])
	var affectedServices []nomad.Service
	for _, s := range args[4:] {
		affectedServices = append(affectedServices, nomad.Service(s))
	}

	// Load services configuration
	servicesConfig, err := config.LoadServices(servicesConfigPath)
	if err != nil {
		return fmt.Errorf("failed to load services configuration: %w", err)
	}

	testConfig := &nomad.TestConfig{
		Phase:            nomad.Critical,
		PrimaryService:   primaryService,
		AffectedServices: affectedServices,
		ServicesConfig:   servicesConfig,
	}
	if _, err := os.Stat(templatePath); os.IsNotExist(err) {
		return fmt.Errorf("template file does not exist: %s", templatePath)
	}

	requireEnvVars := []string{"NOMAD_ADDR", "NOMAD_TOKEN"}
	for _, envVar := range requireEnvVars {
		if os.Getenv(envVar) == "" {
			log.Fatalf("environment variable %s is not set", envVar)
		}
	}

	// create the Nomad client
	client, err := api.NewClient(&api.Config{Address: os.Getenv("NOMAD_ADDR"), SecretID: os.Getenv("NOMAD_TOKEN")})
	if err != nil {
		return fmt.Errorf("failed to create Nomad client: %w", err)
	}
	log.Info("New Client created")

	if err := nomad.ValidateNomadConnection(client); err != nil {
		log.Errorf("Failed to validate Nomad connection: %v", err)
		return fmt.Errorf("failed to validate Nomad connection: %w", err)
	}
	log.Info("Nomad connection valid")

	operator, err := nomad.NewOperator(client, templatePath, testConfig, ticketID)
	if err != nil {
		return fmt.Errorf("failed to create operator: %w", err)
	}
	log.Info("Created operator")
	// Start event consumer to monitor job events
	eventConsumer := nomad.NewEventConsumer(client, operator.OnEvent)
	log.Info("Created Event Consumer")

	//
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
	// Submit the initial critical tests
	resp, err := operator.SubmitJobFromTemplate()
	if err != nil {
		return fmt.Errorf("failed to submit job: %w", err)
	}
	log.Infof("Initial test job submitted successfully: %s", resp.EvalID)
	// Create a channel to signal completion
	done := make(chan struct{})
	operator.SetDoneChannel(done)

	// Wait for either completion or interrupt
	select {
	case <-signals:
		log.Info("Shutting down gracefully...")
		eventConsumer.StopEventConsumer()
		return nil
	case <-done:
		log.Info("All test phases completed successfully")
		eventConsumer.StopEventConsumer()
		return nil
	}
} // run
