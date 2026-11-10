package cmd

import (
	"fmt"
	"os"

	"github.com/bporter816/aws-tui/internal"
	"github.com/bporter816/aws-tui/internal/template"
	"github.com/spf13/cobra"
)

var allowWrite bool

var rootCmd = &cobra.Command{
	Use:   "aws-tui [service [view]]",
	Short: "Terminal UI for AWS",
	Long: `Terminal UI for AWS.

A service (e.g. s3, route53, ec2) opens that service right away; a view
(e.g. "aws-tui ec2 security groups") picks one of its views. Names are
case-insensitive and a unique prefix is enough.`,
	// Execute prints errors itself
	SilenceErrors: true,
	SilenceUsage:  true,
	RunE: func(cmd *cobra.Command, args []string) error {
		start, err := internal.ResolveStartView(args)
		if err != nil {
			return err
		}

		template.Init()

		app := internal.NewApplication(allowWrite)
		app.Open(start)
		if err := app.Run(); err != nil {
			panic(err)
		}
		return nil
	},
}

func init() {
	rootCmd.Flags().BoolVar(&allowWrite, "allow-write", false, "allow changing AWS resources (e.g. editing Route 53 records); read-only otherwise")
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
