// Package nomad provides the operator necessary for controlling the execution of test phases
package nomad

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/log"
	"github.com/google/uuid"
	"github.com/hashicorp/nomad/api"
)

// Operator is a struct which interacts with the Nomad API.
type Operator struct {
	client       *api.Client
	eventID      string
	testConfig   *TestConfig
	templatePath string
	currentPhase TestPhase
	evaluationID string
	allocationID string
	jobID        string
	ticketID     string
	done         chan struct{}
}

// NewOperator is a public function which returns a new Operator instance and optional error.
func NewOperator(client *api.Client, templatePath string, config *TestConfig, ticketID string) (*Operator, error) {
	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("invalid test configuration: %w", err)
	}

	return &Operator{
		client:       client,
		testConfig:   config,
		templatePath: templatePath,
		eventID:      uuid.New().String(),
		currentPhase: Critical, // Start with critical tests
		ticketID:     ticketID,
	}, nil
}

// SetDoneChannel sets the channel used to signal completion of all test phases
func (o *Operator) SetDoneChannel(done chan struct{}) {
	o.done = done
}

// SubmitJobFromTemplate loads a job template from the given path and submits it to Nomad
func (o *Operator) SubmitJobFromTemplate() (*api.JobRegisterResponse, error) {
	vars := JobTemplateVars{
		EventID:       o.eventID,
		TicketID:      o.ticketID,
		TestPhase:     o.currentPhase,
		TestArguments: o.testConfig.BuildRobotTags(),
	}

	job, err := LoadJobTemplate(o.templatePath, o.client, vars)
	if err != nil {
		return nil, fmt.Errorf("failed to load job template: %w", err)
	}

	// Register the job
	resp, _, err := o.client.Jobs().Register(job, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to submit job: %w", err)
	}

	o.evaluationID = resp.EvalID
	if job.ID != nil {
		o.jobID = *job.ID
	}

	return resp, nil
}

// OnEvent handles job events from Nomad
func (o *Operator) OnEvent(eventType string, job *api.Job, alloc *api.Allocation) {
	log.Infof("Received event: %s", eventType)
	log.Infof("Job: %s -- %s", *job.Name, *job.Status)
	log.Infof("allocation: %s", alloc.Name)
	if alloc == nil {
		log.Warn("allocation is nil")
	} else if alloc.ClientStatus == "failed" {
		log.Error("job allocation failed",
			"job", *job.ID,
			"alloc", alloc.ID,
			"description", alloc.TaskStates)
	}

	if eventType == "Job" {
		if *job.Status == "dead" && strings.Contains(*job.Name, o.eventID) {
			log.Infof("job %s is dead, moving to next phase", *job.Name)
			o.progressToNextPhase()
			return
		} else if *job.Status == "running" {
			log.Infof("job %s is running", *job.Name)
			return
		}
		log.Infof("job %s is in unknown state", *job.Name)
	}
}

func (o *Operator) progressToNextPhase() {
	switch o.currentPhase {
	case Critical:
		o.currentPhase = Inner
		if _, err := o.SubmitJobFromTemplate(); err != nil {
			log.Error("failed to submit inner phase job", "error", err)
		}
	case Inner:
		o.currentPhase = Outer
		if _, err := o.SubmitJobFromTemplate(); err != nil {
			log.Error("failed to submit outer phase job", "error", err)
		}
	case Outer:
		log.Info("all test phases complete")
		if o.done != nil {
			close(o.done)
		}
		return
	}

	log.Info("progressing to next phase", "phase", o.currentPhase)
}
