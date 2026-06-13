package cmd

import (
	"fmt"
	"os"

	"github.com/hitzhangjie/skillhub/pkg/config"
	"github.com/spf13/cobra"
)

var (
	indexURI    string
	installRoot string
)

var rootCmd = &cobra.Command{
	Use:   "skillhub",
	Short: "SkillHub CLI - Install and manage AI skills",
	Long: `SkillHub CLI is a command-line tool for discovering, installing, and managing
AI skills from the SkillHub community and enterprise registries.

Examples:
  skillhub search "code review"
  skillhub install my-skill
  skillhub install @my-org/private-skill
  skillhub list
  skillhub upgrade
  skillhub publish ./my-skill --dry-run

Shell completion:
  source <(skillhub completion bash)
  source <(skillhub completion zsh)
  skillhub completion fish | source`,
	Version:      config.Version,
	SilenceUsage: true,
}

func init() {
	rootCmd.PersistentFlags().StringVar(&indexURI, "index", config.Metadata.SkillsIndexURL,
		"Skills index JSON path/URI (supports http://, https://, file://, or local paths)")
	rootCmd.PersistentFlags().StringVar(&installRoot, "dir", config.DefaultInstallRoot,
		"Install root directory")

}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
