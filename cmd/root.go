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
	Use: "aws-tui",
	Run: func(cmd *cobra.Command, args []string) {
		template.Init()

		app := internal.NewApplication(allowWrite)
		if err := app.Run(); err != nil {
			panic(err)
		}
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
