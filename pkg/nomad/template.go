// Package nomad provides the types  and functions for reading nomad job templates, rendering them, and running them via the Nomad API
package nomad

import (
	"fmt"
	"os"
	"strings"
	"text/template"

	"github.com/hashicorp/nomad/api"
)

// JobTemplateVars contains the variables needed to render the job template
type JobTemplateVars struct {
	EventID       string
	TicketID      string
	TestPhase     TestPhase
	TestArguments string
}

// LoadJobTemplate reads an HCL template file and returns a Nomad job
func LoadJobTemplate(templatePath string, client *api.Client, vars JobTemplateVars) (*api.Job, error) {
	// Read the template file
	content, err := os.ReadFile(templatePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read template file: %w", err)
	}

	// Create a new template and parse the job file
	tmpl, err := template.New("job").Delims("[[", "]]").Parse(string(content))
	if err != nil {
		return nil, fmt.Errorf("failed to parse template: %w", err)
	}

	// Create a buffer to store our rendered template
	var rendered strings.Builder
	err = tmpl.Execute(&rendered, struct {
		EventID       string
		TicketID      string
		TestPhase     string
		TestArguments string
	}{
		EventID:       vars.EventID,
		TicketID:      vars.TicketID,
		TestPhase:     string(vars.TestPhase),
		TestArguments: vars.TestArguments,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to render template: %w", err)
	}

	// Parse the rendered template as a Nomad job
	jobs := client.Jobs()
	job, err := jobs.ParseHCL(rendered.String(), true)
	if err != nil {
		return nil, fmt.Errorf("failed to parse job spec: %w", err)
	}

	return job, nil
}
