package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/hitzhangjie/skillhub/pkg/config"
	"github.com/hitzhangjie/skillhub/pkg/credentials"
	"github.com/hitzhangjie/skillhub/pkg/enterprise"
	"github.com/spf13/cobra"
)

var loginKey, loginHost string

var loginCmd = &cobra.Command{
	Use:   "login --key <api-key> [--host <url>]",
	Short: "Login to an enterprise source using API key",
	Long: `Login to an enterprise SkillHub registry using an API key (sk-ent-...).

The API key is verified against the registry, and credentials are stored in
~/.skillhub/credentials.json for subsequent use.`,
	Example: `  skillhub login --key sk-ent-abc123
  skillhub login --key sk-ent-abc123 --host https://custom.skillhub.cn`,
	RunE: runLogin,
}

func init() {
	loginCmd.Flags().StringVar(&loginKey, "key", "", "Enterprise API key (sk-ent-...)")
	loginCmd.MarkFlagRequired("key")
	loginCmd.Flags().StringVar(&loginHost, "host", "", "Enterprise host URL")
	rootCmd.AddCommand(loginCmd)
}

func runLogin(cmd *cobra.Command, args []string) error {
	key := strings.TrimSpace(loginKey)
	if key == "" {
		return fmt.Errorf("--key is required")
	}

	host := strings.TrimSpace(loginHost)
	if host == "" {
		host = config.DefaultEnterpriseHost
	}

	orgInfo, err := enterprise.VerifyAPIKey(host, key)
	if err != nil {
		return fmt.Errorf("login failed: %w", err)
	}

	oi := credentials.OrgInfo{
		OrgID:    orgInfo.OrgID,
		OrgOrgID: orgInfo.OrgOrgID,
		OrgSlug:  orgInfo.OrgSlug,
		OrgName:  orgInfo.OrgName,
		Host:     host,
		APIKey:   key,
	}

	// Check for old-format migration
	creds := credentials.Load()
	if orgInfo.OrgSlug != orgInfo.OrgOrgID {
		if _, exists := creds.Orgs[orgInfo.OrgSlug]; exists {
			delete(creds.Orgs, orgInfo.OrgSlug)
			fmt.Fprintf(os.Stderr, "Migrating credentials from @%s to @%s\n", orgInfo.OrgSlug, orgInfo.OrgOrgID)
		}
	}
	isFirst := len(creds.Orgs) == 0

	if err := credentials.SaveOrg(oi); err != nil {
		return fmt.Errorf("failed to save credentials: %w", err)
	}

	if isFirst {
		credPath := config.GetCredentialsPath()
		fmt.Fprintf(os.Stderr, "WARNING! Your API key is stored unencrypted in '%s'.\n", credPath)
		fmt.Fprintf(os.Stderr, "Configure a credential helper to remove this warning.\n")
	}

	fmt.Printf("✓ Logged in to %s (@%s) at %s\n", orgInfo.OrgName, orgInfo.OrgOrgID, host)
	return nil
}
