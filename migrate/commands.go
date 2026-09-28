package migrate

import (
	"os"
	"time"

	"github.com/spf13/cobra"
)

const commandUsage = `Usage:
  {{.UseLine}}

{{if .HasAvailableLocalFlags}}Flags:
{{.LocalFlags.FlagUsages | trimTrailingWhitespaces}}
{{end}}`

// Commands returns the migration subcommands.
func Commands() []*cobra.Command {
	cmds := []*cobra.Command{
		newLoad(),
		newReplicate(),
		newStatus(),
		newCutover(),
	}
	for _, cmd := range cmds {
		cmd.SilenceErrors = true
		cmd.SilenceUsage = true
		cmd.DisableFlagsInUseLine = true
		cmd.DisableSuggestions = true
		cmd.SetUsageTemplate(commandUsage)
	}
	return cmds
}

func newLoad() *cobra.Command {
	return &cobra.Command{
		Use:   "load SRC DST DATABASE...",
		Short: "copy schema and rows from MySQL into HardhatDB",
		Args:  cobra.MinimumNArgs(3),
		RunE: func(cmd *cobra.Command, args []string) error {
			return Load(cmd.Context(), args[0], args[1], args[2:], os.Stdout, os.Stderr)
		},
	}
}

func newReplicate() *cobra.Command {
	return &cobra.Command{
		Use:   "replicate SRC DST DATABASE...",
		Short: "start replication from a MySQL binlog",
		Args:  cobra.MinimumNArgs(3),
		RunE: func(cmd *cobra.Command, args []string) error {
			return Replicate(cmd.Context(), args[0], args[1], args[2:], os.Stdout)
		},
	}
}

func newStatus() *cobra.Command {
	return &cobra.Command{
		Use:   "status DST",
		Short: "show replica status",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return Status(cmd.Context(), args[0], os.Stdout)
		},
	}
}

func newCutover() *cobra.Command {
	var timeout time.Duration
	var interval time.Duration
	cmd := &cobra.Command{
		Use:   "cutover DST",
		Short: "wait until the replica has caught up, then stop it",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return Cutover(cmd.Context(), args[0], timeout, interval, os.Stdout)
		},
	}
	cmd.Flags().DurationVar(&timeout, "timeout", 5*time.Minute, "how long to wait for the replica to catch up")
	cmd.Flags().DurationVar(&interval, "interval", time.Second, "how often to read replica status")
	return cmd
}
