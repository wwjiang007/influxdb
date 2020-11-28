package inspect

import (
	"github.com/spf13/cobra"
)

// NewCommand creates the new command.
func NewCommand() *cobra.Command {
	base := &cobra.Command{
		Use:   "inspect",
		Short: "Commands for inspecting on-disk database data",
		Args:  cobra.NoArgs,
		Run: func(cmd *cobra.Command, args []string) {
			cmd.PrintErrf("See '%s -h' for help\n", cmd.CommandPath())
		},
	}

	// List of available sub-commands
	// If a new sub-command is created, it must be added here
	subCommands := []*cobra.Command{
		//NewBuildTSICommand(),
		//NewCompactSeriesFileCommand(),
		//NewExportBlocksCommand(),
		NewExportIndexCommand(),
		//NewReportTSMCommand(),
		//NewVerifyTSMCommand(),
		//NewVerifyWALCommand(),
		//NewReportTSICommand(),
		//NewVerifySeriesFileCommand(),
		//NewDumpWALCommand(),
		//NewDumpTSICommand(),
	}

	base.AddCommand(subCommands...)

	return base
}
