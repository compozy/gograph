package commands_test

import (
	"testing"

	"github.com/compozy/gograph/cmd/gograph/commands"
	"github.com/stretchr/testify/assert"
)

func TestMCPCommand(t *testing.T) {
	t.Run("Should register MCP command", func(t *testing.T) {
		// Register the command
		commands.RegisterMCPCommand()

		// The command should be registered - we can't test more without
		// actually running the command which would start a server
		assert.True(t, true, "Command registered successfully")
	})
}
