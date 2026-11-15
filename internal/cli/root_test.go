package cli

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCLIHelpDoesNotRequireDatabase(t *testing.T) {
	for _, args := range [][]string{{"--help"}, {"serve", "--help"}, {"db", "--help"}, {"db", "seed", "--help"}, {"admin", "--help"}} {
		cmd := NewRootCommand()
		var out bytes.Buffer
		cmd.SetOut(&out)
		cmd.SetErr(&out)
		cmd.SetArgs(args)
		require.NoError(t, cmd.Execute())
		assert.Contains(t, out.String(), "Usage:")
	}
}
func TestCLIRejectsUnknownCommandsAndExtraArguments(t *testing.T) {
	for _, args := range [][]string{{"unknown"}, {"serve", "unexpected"}, {"db", "up", "unexpected"}, {"db", "seed", "unexpected"}, {"db", "new"}} {
		cmd := NewRootCommand()
		cmd.SetArgs(args)
		require.Error(t, cmd.Execute())
	}
}
