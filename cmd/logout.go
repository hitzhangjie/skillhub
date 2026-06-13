package cmd

import (
	"fmt"
	"strings"

	"github.com/hitzhangjie/skillhub/pkg/credentials"
	"github.com/spf13/cobra"
)

var logoutOrg string

var logoutCmd = &cobra.Command{
	Use:   "logout [--org <org-slug>]",
	Short: "Logout from an enterprise source",
	Long: `Logout from a configured enterprise source, removing stored credentials.

If only one enterprise source is configured, --org is optional.
Otherwise, you must specify which org to logout from.`,
	Example: `  skillhub logout
  skillhub logout --org my-org`,
	RunE: runLogout,
}

func init() {
	logoutCmd.Flags().StringVar(&logoutOrg, "org", "", "Organization slug to logout from")
	rootCmd.AddCommand(logoutCmd)
}

func runLogout(cmd *cobra.Command, args []string) error {
	creds := credentials.Load()
	orgs := creds.Orgs

	orgInput := strings.TrimPrefix(strings.TrimSpace(logoutOrg), "@")

	if orgInput == "" {
		if len(orgs) == 1 {
			for k := range orgs {
				orgInput = k
				break
			}
		} else if len(orgs) == 0 {
			return fmt.Errorf("no enterprise sources configured. nothing to logout")
		} else {
			names := make([]string, 0, len(orgs))
			for k := range orgs {
				names = append(names, "@"+k)
			}
			return fmt.Errorf("multiple enterprise sources configured (%s). please specify --org", strings.Join(names, ", "))
		}
	}

	// Find matching org
	matchedKey := ""
	if _, ok := orgs[orgInput]; ok {
		matchedKey = orgInput
	} else {
		for key, info := range orgs {
			if info.OrgOrgID == orgInput || info.OrgSlug == orgInput {
				matchedKey = key
				break
			}
		}
	}
	if matchedKey == "" {
		return fmt.Errorf("not logged in to @%s", orgInput)
	}

	if err := credentials.DeleteOrg(matchedKey); err != nil {
		return err
	}
	fmt.Printf("✓ Logged out from @%s\n", matchedKey)
	return nil
}
