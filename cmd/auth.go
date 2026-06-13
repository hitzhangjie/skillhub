package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/hitzhangjie/skillhub/pkg/config"
	"github.com/hitzhangjie/skillhub/pkg/credentials"
	"github.com/hitzhangjie/skillhub/pkg/enterprise"
	"github.com/spf13/cobra"
)

var authCmd = &cobra.Command{
	Use:   "auth",
	Short: "Manage personal API token (skh_)",
	Long: `Manage your personal SkillHub API token for community registry publishing.

Subcommands:
  login   - Login with an API token
  logout  - Clear stored API token
  whoami  - Show current user identity
  token   - Print current token to stdout (for CI use)`,
}

var (
	authToken string
	authHost  string
	authJSON  bool
)

var authLoginCmd = &cobra.Command{
	Use:   "login --token <skh_xxx>",
	Short: "Login with a personal API token",
	Example: `  skillhub auth login --token skh_abc123
  skillhub auth login --token skh_abc123 --host https://custom.api.com`,
	RunE: runAuthLogin,
}

var authLogoutCmd = &cobra.Command{
	Use:   "logout",
	Short: "Clear stored API token",
	RunE:  runAuthLogout,
}

var authWhoamiCmd = &cobra.Command{
	Use:   "whoami",
	Short: "Show current user identity",
	RunE:  runAuthWhoami,
}

var authTokenCmd = &cobra.Command{
	Use:   "token",
	Short: "Print current token to stdout (for CI)",
	RunE:  runAuthToken,
}

func init() {
	authLoginCmd.Flags().StringVar(&authToken, "token", "", "Personal API token (skh_...)")
	authLoginCmd.MarkFlagRequired("token")
	authLoginCmd.Flags().StringVar(&authHost, "host", "", "API host")
	authCmd.AddCommand(authLoginCmd)

	authWhoamiCmd.Flags().StringVar(&authHost, "host", "", "API host (overrides stored host)")
	authWhoamiCmd.Flags().BoolVar(&authJSON, "json", false, "JSON output")
	authCmd.AddCommand(authWhoamiCmd)

	authCmd.AddCommand(authLogoutCmd)
	authCmd.AddCommand(authTokenCmd)
	rootCmd.AddCommand(authCmd)
}

func runAuthLogin(cmd *cobra.Command, args []string) error {
	token := strings.TrimSpace(authToken)
	if token == "" {
		return fmt.Errorf("--token is required")
	}
	if !strings.HasPrefix(token, "skh_") {
		return fmt.Errorf("token must start with skh_")
	}

	host := strings.TrimSpace(authHost)
	if host == "" {
		host = config.DefaultEnterpriseHost
	}

	info, err := enterprise.GetAuthMe(host, token)
	if err != nil {
		return fmt.Errorf("login failed: %w", err)
	}

	userID := 0
	if v, ok := info["id"].(float64); ok {
		userID = int(v)
	}
	if userID == 0 {
		if v, ok := info["userId"].(float64); ok {
			userID = int(v)
		}
	}
	handle := strVal(info, "handle")

	if userID == 0 {
		return fmt.Errorf("auth/me response missing id/userId field")
	}

	if err := credentials.SaveUser(host, token, userID, handle); err != nil {
		return fmt.Errorf("failed to save credentials: %w", err)
	}
	fmt.Printf("✓ Logged in as @%s (userId=%d)\n", handle, userID)
	return nil
}

func runAuthLogout(cmd *cobra.Command, args []string) error {
	user := credentials.GetUser()
	if user == nil {
		fmt.Println("Already logged out")
		return nil
	}
	if err := credentials.ClearUser(); err != nil {
		return err
	}
	fmt.Println("✓ Logged out")
	return nil
}

func runAuthWhoami(cmd *cobra.Command, args []string) error {
	user := credentials.GetUser()
	if user == nil {
		return fmt.Errorf("not logged in. run: skillhub auth login --token skh_xxx")
	}

	host := strings.TrimSpace(authHost)
	if host == "" {
		host = user.Host
	}
	if host == "" {
		host = config.DefaultEnterpriseHost
	}

	info, err := enterprise.GetAuthMe(host, user.Token)
	if err != nil {
		return fmt.Errorf("whoami failed: %w", err)
	}

	if authJSON {
		data, _ := json.MarshalIndent(info, "", "  ")
		fmt.Println(string(data))
		return nil
	}

	id := 0
	if v, ok := info["id"].(float64); ok {
		id = int(v)
	}
	if id == 0 {
		if v, ok := info["userId"].(float64); ok {
			id = int(v)
		}
	}
	fmt.Printf("userId : %d\n", id)
	fmt.Printf("handle : %s\n", strVal(info, "handle"))
	if role := strVal(info, "role"); role != "" {
		fmt.Printf("role   : %s\n", role)
	}
	return nil
}

func runAuthToken(cmd *cobra.Command, args []string) error {
	user := credentials.GetUser()
	if user == nil {
		return fmt.Errorf("not logged in")
	}
	os.Stdout.WriteString(user.Token)
	return nil
}

func strVal(m map[string]interface{}, key string) string {
	if v, ok := m[key].(string); ok {
		return strings.TrimSpace(v)
	}
	return ""
}
