package main

import (
	"github.com/bongani-m/hardhatdb/go/store"
	"github.com/spf13/cobra"
)

func restoreCommand() *cobra.Command {
	var from, data string
	cmd := &cobra.Command{
		Use:   "restore --from <backup-dir> --data <data-dir>",
		Short: "load a HardhatDB backup into an empty data directory",
		RunE: func(cmd *cobra.Command, args []string) error {
			return store.RestoreBackup(from, data)
		},
	}
	cmd.Flags().StringVar(&from, "from", "", "backup directory")
	cmd.Flags().StringVar(&data, "data", "", "empty data directory")
	_ = cmd.MarkFlagRequired("from")
	_ = cmd.MarkFlagRequired("data")
	cmd.SilenceErrors = true
	cmd.SilenceUsage = true
	cmd.DisableFlagsInUseLine = true
	cmd.DisableSuggestions = true
	return cmd
}
