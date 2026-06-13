package cmd

import (
	"fmt"

	"github.com/hitzhangjie/skillhub/pkg/config"
	"github.com/spf13/cobra"
)

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print the CLI version",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println(config.Version)
	},
}

func init() {
	rootCmd.AddCommand(versionCmd)
}
