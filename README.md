# Test Operator

This repo contains a model implementation for a Nomad operator intended to control the lifecycle of a test suite for a complex application.

## Architecture

The operator is deployed via the Nomad API by an external API listener which is triggered on a CDEvent.
The operator then launches tests in an expanding scope, based on the payload of the CDEvent, so that services involved in the event are tested from the inside out.
