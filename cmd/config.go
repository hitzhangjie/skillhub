package cmd

import (
	"fmt"

	"github.com/hitzhangjie/skillhub/pkg/config"
	"github.com/hitzhangjie/skillhub/pkg/credentials"
	"github.com/spf13/cobra"
)

var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Manage CLI configuration",
	Long:  "View and manage SkillHub CLI configuration, including enterprise sources.",
	RunE:  runConfigList,
}

var configListCmd = &cobra.Command{
	Use:   "list",
	Short: "List configured enterprise sources",
	RunE:  runConfigList,
}

func init() {
	configCmd.AddCommand(configListCmd)
	rootCmd.AddCommand(configCmd)
}

func runConfigList(cmd *cobra.Command, args []string) error {
	orgs := credentials.GetAllOrgs()
	if len(orgs) == 0 {
		fmt.Println("No enterprise sources configured.")
		fmt.Println(`Run "skillhub login --key <api-key>" to add one.`)
		return nil
	}

	fmt.Println("Enterprise Sources:")
	for _, info := range orgs {
		host := info.Host
		if host == "" {
			host = config.DefaultEnterpriseHost
		}
		// @key uses the map key (same as Python's for key, info in orgs.items())
		fmt.Printf("  @%s\n", info.Key)
		if info.OrgName != "" {
			fmt.Printf("    Name:      %s\n", info.OrgName)
		}
		if info.OrgID == 0 {
			fmt.Printf("    Org ID:    ?\n")
		} else {
			fmt.Printf("    Org ID:    %d\n", info.OrgID)
		}
		// Team ID falls back to map key if orgOrgId is empty (matching Python)
		teamID := info.OrgOrgID
		if teamID == "" {
			teamID = info.Key
		}
		fmt.Printf("    Team ID:   %s\n", teamID)
		if info.OrgSlug != teamID {
			fmt.Printf("    Slug:      %s\n", info.OrgSlug)
		}
		fmt.Printf("    Host:      %s\n", host)
		fmt.Printf("    API Key:   %s\n", credentials.MaskAPIKey(info.APIKey))
		fmt.Printf("    Logged in: %s\n", info.LoggedInAt)
		fmt.Println()
	}
	return nil
}
