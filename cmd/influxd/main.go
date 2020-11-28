package main

import (
	"context"
	"fmt"
	_ "net/http/pprof"
	"os"
	"time"

	"github.com/influxdata/influxdb/v2"
	"github.com/influxdata/influxdb/v2/cmd/influxd/inspect"
	"github.com/influxdata/influxdb/v2/cmd/influxd/launcher"
	"github.com/influxdata/influxdb/v2/cmd/influxd/upgrade"
	_ "github.com/influxdata/influxdb/v2/tsdb/engine/tsm1"
	_ "github.com/influxdata/influxdb/v2/tsdb/index/tsi1"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	version = "dev"
	commit  = "none"
	date    = ""
)

func main() {
	if len(date) == 0 {
		date = time.Now().UTC().Format(time.RFC3339)
	}

	influxdb.SetBuildInfo(version, commit, date)

	v := viper.New()
	rootCmd := launcher.NewInfluxdCommand(context.Background(), v)
	// upgrade binds options to env variables, so it must be added after rootCmd is initialized
	rootCmd.AddCommand(upgrade.NewCommand(v))
	rootCmd.AddCommand(inspect.NewCommand())
	rootCmd.AddCommand(versionCmd())

	rootCmd.SilenceUsage = true
	if err := rootCmd.Execute(); err != nil {
		rootCmd.PrintErrf("See '%s -h' for help\n", rootCmd.CommandPath())
		os.Exit(1)
	}
}

func versionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print the influxd server version",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Printf("InfluxDB %s (git: %s) build_date: %s\n", version, commit, date)
		},
	}
}
