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

type EventConsumer struct {
	client  *api.Client
	onEvent func(eventType string, job *api.Job)
	stop    func()
}

// NewOperator is a public function which returns a new Operator instance and optional error
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

func (ec *EventConsumer) consume(ctx context.Context) error {
	log.Info("Consuming events")
	// this is the index of the event
	var index uint64 = 0
	// specify the topics we want to consume
	topics := map[api.Topic][]string{
		api.TopicJob: {"*"},
	}

	// start the event stream
	eventsClient := ec.client.EventStream()

	log.Info("Establishing event stream")
	// create the channel for the events
	eventCh, err := eventsClient.Stream(ctx, topics, index, &api.QueryOptions{})
	if err != nil {
		log.Errorf("Failed to create event stream: %v", err)
		return fmt.Errorf("Failed to create event stream: %w", err)
	}

	lastEventTime := time.Now()
	heartbeatCheck := time.NewTicker(30 * time.Second)

	defer heartbeatCheck.Stop()

	// Loop over events in the stream
	for {
		select {
		// If the context is closed, return no error.
		case <-ctx.Done():
			return nil
		// If it's a heartbeat event, check when last we got
		case <-heartbeatCheck.C:
			if time.Since(lastEventTime) > 10*time.Minute {
				log.Warn("No events received in the last 10 minutes")
				return fmt.Errorf("No events received in the last 10 minutes")
			}
		case jobEvent := <-eventCh:
			if jobEvent.Err != nil {
				log.Errorf("Error receiving event: %v", jobEvent.Err)
				continue
			}
			log.Infof("Received job event: %v", jobEvent)
		}
	}
}

func (o *Operator) OnEvent(eventType string, job *api.Job) {
	log.Infof("Node %s\n", job)
}
