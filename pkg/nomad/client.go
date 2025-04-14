package nomad

import (
	"context"
	"fmt"
	"time"

	"github.com/charmbracelet/log"
	"github.com/hashicorp/nomad/api"
)

// Operator is a struct which interacts with the Nomad API.
type Operator struct {
	client  *api.Client
	onEvent func(eventType string, job *api.Job)
}

// EventConsumer is a struct which contains a client, a function to handle events and a function to stop the consumer
type EventConsumer struct {
	client  *api.Client
	onEvent func(eventType string, job *api.Job)
	stop    func()
}

// validateNomadConnection is a utility private function to check that the Nomad client can interact with the Nomad API.
func ValidateNomadConnection(client *api.Client) error {
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

// NewOperator is a public function which returns a new Operator instance and optional error.
func NewOperator(client *api.Client) (*Operator, error) {
	return &Operator{
		client: client,
		onEvent: func(eventType string, job *api.Job) {
			log.Infof("EventConsumer: received event %s for job %s", eventType, job)
		},
	}, nil
}

// onAllocation is a function which takes an allocation event. It is invoked when new allocation events are received.
func (o *Operator) onAllocation(eventType string, allocation *api.Allocation) {
	// do nothing for now
	return
}

func NewEventConsumer(client *api.Client, onEvent func(eventType string, job *api.Job)) *EventConsumer {
	return &EventConsumer{
		client:  client,
		onEvent: onEvent,
	}
}

// StopEventConsumer is a public function which is used to stop the event consumer during shutdown.
func (ec *EventConsumer) StopEventConsumer() {
	if ec.stop != nil {
		ec.stop()
	}
}

// StartEventConsumer is a function which receives a EventConsumer type
// which starts the context in the background
func (ec *EventConsumer) StartEventConsumer() {
	ctx := context.Background()
	log.Info("EventConsumer: started context in background")
	ctx, ec.stop = context.WithCancel(ctx)
	ec.consume(ctx)
}

// consumer is a private function which is used to consumer the event. It receives the event payload from Nomad and performs business logic on it.
func (ec *EventConsumer) consume(ctx context.Context) error {
	log.Info("Consuming events")
	// this is the index of the event
	var index uint64 = 0
	// specify the topics we want to consume
	topics := map[api.Topic][]string{
		api.TopicJob: {"*"},
	}

	// eventsClient reads the event stream, which will later loop over
	log.Info("Establishing event stream")
	eventsClient := ec.client.EventStream()

	// create the channel for the events
	eventCh, err := eventsClient.Stream(ctx, topics, index, &api.QueryOptions{})
	if err != nil {
		log.Errorf("Failed to create event stream: %v", err)
		return fmt.Errorf("Failed to create event stream: %w", err)
	}

	// We start a ticker from now, in order to check if events are still coming.
	// We include heartbeat events in our check, but skip them in the event loop for processing later.
	lastEventTime := time.Now()
	heartbeatCheck := time.NewTicker(30 * time.Second)
	defer heartbeatCheck.Stop()

	// Loop over events in the stream
	for {
		select {
		// If the context is closed, return no error.
		case <-ctx.Done():
			return nil
		// If it's a heartbeat event, check when last we got an event.
		// if it's more than 10 minutes ago, log a warning
		// C is a ticker channel. This case deals with ticker events.
		case <-heartbeatCheck.C:
			if time.Since(lastEventTime) > 10*time.Minute {
				log.Warn("No events received in the last 10 minutes")
				// return fmt.Errorf("No events received in the last 10 minutes")
				continue
			}
		// eventCh is an event stream channel. This case deals with event stream events.
		case jobEvent := <-eventCh:
			if jobEvent.Err != nil {
				log.Errorf("Error receiving event: %v", jobEvent.Err)
				continue
			}
			// loop over the events received in the stream and handle them
			for _, event := range jobEvent.Events {
				log.Infof("Received event: %v", event.Topic)
				if event.Topic == "Job" {
					job, err := event.Job()
					if err != nil {
						log.Errorf("Error getting job: %v", err)
						continue
					}
					// If the job event is a complete event, construct the cdevent and submit the new batch job
					log.Infof("job name: %s", *job.Name)
				}
			}
		}
	}
}

func (o *Operator) OnEvent(eventType string, job *api.Job) {
	log.Infof("Node %s\n", job)
}
