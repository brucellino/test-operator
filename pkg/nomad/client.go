// Package nomad interacts with the Nomad API, consuming events from the Nomad events stream.
package nomad

import (
	"context"
	"fmt"
	"time"

	"github.com/charmbracelet/log"
	"github.com/hashicorp/nomad/api"
)

// EventConsumer handles the event stream from Nomad
type EventConsumer struct {
	client  *api.Client
	onEvent func(eventType string, job *api.Job, alloc *api.Allocation)
	stop    func()
}

// ValidateNomadConnection checks that the Nomad client can interact with the Nomad API
func ValidateNomadConnection(client *api.Client) error {
	nodes, _, err := client.Nodes().List(&api.QueryOptions{})
	if err != nil {
		return fmt.Errorf("failed to list nodes: %w", err)
	}

	if len(nodes) == 0 {
		return fmt.Errorf("no nodes found - check if NOMAD_TOKEN has correct permissions or is set")
	}

	topics := map[api.Topic][]string{
		api.TopicAllocation: {"*"},
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
	defer cancel()

	eventsCh, err := client.EventStream().Stream(ctx, topics, 0, &api.QueryOptions{})
	if err != nil {
		return fmt.Errorf("failed to validate nomad event stream: %w", err)
	}

	select {
	case event := <-eventsCh:
		if event.Err != nil {
			return fmt.Errorf("failed to validate nomad event stream: %w", event.Err)
		}
	case <-ctx.Done():
		return fmt.Errorf("timeout waiting for event stream connection")
	}

	return nil
}

// NewEventConsumer creates a new event consumer
func NewEventConsumer(client *api.Client, onEvent func(eventType string, job *api.Job, alloc *api.Allocation)) *EventConsumer {
	return &EventConsumer{
		client:  client,
		onEvent: onEvent,
	}
}

// StopEventConsumer stops the event consumer
func (ec *EventConsumer) StopEventConsumer() {
	if ec.stop != nil {
		ec.stop()
	}
}

// StartEventConsumer starts consuming events
func (ec *EventConsumer) StartEventConsumer() {
	ctx := context.Background()
	ctx, ec.stop = context.WithCancel(ctx)
	go ec.consume(ctx)
}

// consume handles the event stream.
func (ec *EventConsumer) consume(ctx context.Context) error {
	log.Info("consuming events")
	var index uint64

	topics := map[api.Topic][]string{
		api.TopicAllocation: {"*"},
		api.TopicJob:        {"*"},
	}

	eventsClient := ec.client.EventStream()
	eventCh, err := eventsClient.Stream(ctx, topics, index, &api.QueryOptions{})
	if err != nil {
		return fmt.Errorf("failed to create event stream: %w", err)
	}

	lastEventTime := time.Now()
	heartbeatCheck := time.NewTicker(30 * time.Second)
	defer heartbeatCheck.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil

		case <-heartbeatCheck.C:
			if time.Since(lastEventTime) > 10*time.Minute {
				log.Warn("no events received in the last 10 minutes")
				continue
			}

		case event := <-eventCh:
			if event.Err != nil {
				log.Error("error receiving event", "error", event.Err)
				continue
			}

			lastEventTime = time.Now()

			for _, e := range event.Events {
				switch e.Topic {
				case "Allocation":
					alloc, err := e.Allocation()

					log.Infof("Allocation event happened: %s  -- ", alloc.Name)
					if err != nil {
						log.Error("failed to parse allocation", "error", err)
						continue
					}
					if alloc.Job != nil {
						log.Info("allocation completed",
							"job", *alloc.Job.ID,
							"alloc", alloc.ID,
							"status", alloc.ClientStatus)
						ec.onEvent("Allocation", alloc.Job, alloc)
					}

				case "Job":
					log.Info("Job event happened")
					job, err := e.Job()
					if err != nil {
						log.Error("failed to parse job", "error", err)
						continue
					}
					ec.onEvent("Job", job, nil)
				default:
					log.Infof("unknown topic %s", e.Topic)
				}
			}
		}
	}
}
