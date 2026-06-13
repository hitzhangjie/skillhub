package cmd

import (
	"fmt"

	"github.com/hitzhangjie/skillhub/pkg/upgrade"
	"github.com/spf13/cobra"
)

var (
	upgradeSlug      string
	upgradeCheckOnly bool
	upgradeTimeout   int
)

var upgradeCmd = &cobra.Command{
	Use:   "upgrade [slug]",
	Short: "Upgrade installed skills",
	Long: `Upgrade installed skills to their latest versions.

If a slug is provided, only that skill is checked for upgrades.
Without arguments, all installed skills are checked and upgraded.`,
	Example: `  skillhub upgrade
  skillhub upgrade my-skill
  skillhub upgrade --check-only
  skillhub upgrade my-skill --check-only`,
	RunE: runUpgrade,
}

func init() {
	upgradeCmd.Flags().BoolVar(&upgradeCheckOnly, "check-only", false, "Only check for upgrades without installing")
	upgradeCmd.Flags().IntVar(&upgradeTimeout, "timeout", 20, "Timeout in seconds for manifest fetch")
	rootCmd.AddCommand(upgradeCmd)
}

func runUpgrade(cmd *cobra.Command, args []string) error {
	slug := ""
	if len(args) > 0 {
		slug = args[0]
	}

	code, err := upgrade.Run(slug, installRoot, upgradeCheckOnly, upgradeTimeout)
	if err != nil {
		return err
	}
	if code != 0 {
		return fmt.Errorf("upgrade completed with errors")
	}
	return nil
}
