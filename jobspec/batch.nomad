job "robot-test-[[ .TicketID ]]-[[ .TestPhase ]]-[[ .EventId ]]" {
  datacenters = ["dc1"]
  type = "batch"
  group "test" {
    count = 1
    task "test-runner" {
      driver = "docker"
      config {
        image = "alpine:latest"
        command = "/bin/echo"
        args = [
          "Hello, World!"
        ]
      }
      template {
        data = <<EOH
#!/bin/sh
TEST_SUITE_NAME="${test_phase}"
TEST_SUITE_URI="https://github.com/tests/suite"
TRIGGER="[[ .TicketID]]"
TEST_ARGUMENTS="[[ .TestArguments ]]"
EOH
        destination = "local/env.sh"
        change_mode = "noop"
      }

      resources {
        cpu    = 500
        memory = 512
      }
    }
  }
}
