package cmd

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/hitzhangjie/skillhub/pkg/config"
	"github.com/hitzhangjie/skillhub/pkg/credentials"
	"github.com/hitzhangjie/skillhub/pkg/metadata"
	"github.com/spf13/cobra"
)

var (
	pubVersion   string
	pubChangelog string
	pubDryRun    bool
	pubToken     string
	pubHost      string
	pubJSON      bool
)

var publishCmd = &cobra.Command{
	Use:   "publish <path>",
	Short: "Publish a skill to community registry",
	Long: `Publish a skill to the SkillHub community registry.

The path can be a skill directory or a .zip file. The directory/zip must
contain a SKILL.md file with YAML front matter defining slug, version,
displayName, and other metadata.`,
	Example: `  skillhub publish ./my-skill --dry-run
  skillhub publish ./my-skill --version 1.2.0 --changelog "bug fixes"
  skillhub publish ./my-skill.zip --dry-run
  skillhub publish ./my-skill --json
  skillhub publish ./my-skill --token skh_xxx`,
	Args: cobra.ExactArgs(1),
	RunE: runPublish,
}

func init() {
	publishCmd.Flags().StringVar(&pubVersion, "version", "", "Override version from SKILL.md")
	publishCmd.Flags().StringVar(&pubChangelog, "changelog", "", "Changelog for this version")
	publishCmd.Flags().BoolVar(&pubDryRun, "dry-run", false, "Validate metadata and packaging without uploading")
	publishCmd.Flags().StringVar(&pubToken, "token", "", "Override API token (skh_...)")
	publishCmd.Flags().StringVar(&pubHost, "host", "", "API host")
	publishCmd.Flags().BoolVar(&pubJSON, "json", false, "JSON output")
	rootCmd.AddCommand(publishCmd)
}

func runPublish(cmd *cobra.Command, args []string) error {
	pathArg := args[0]

	skillDir, cleanupDir, err := metadata.ResolveSkillInput(pathArg)
	if err != nil {
		return err
	}
	if cleanupDir != "" {
		defer os.RemoveAll(cleanupDir)
	}

	fm, err := metadata.ParseFrontmatter(skillDir + "/SKILL.md")
	if err != nil {
		return err
	}

	if strings.TrimSpace(pubVersion) != "" {
		fm["version"] = strings.TrimSpace(pubVersion)
	}

	if err := metadata.ValidateMetadata(fm); err != nil {
		return err
	}

	if pubDryRun {
		if pubJSON {
			data, _ := json.Marshal(map[string]interface{}{
				"dryRun":  true,
				"slug":    fm["slug"],
				"version": fm["version"],
			})
			fmt.Println(string(data))
		} else {
			fmt.Printf("✓ Dry-run passed: %s@%s\n", fm["slug"], fm["version"])
		}
		return nil
	}

	token, host := resolvePublishToken()
	if token == "" {
		return fmt.Errorf("not logged in. run: skillhub auth login --token skh_xxx")
	}

	skillFiles, err := metadata.CollectSkillFiles(skillDir)
	if err != nil {
		return fmt.Errorf("failed to collect skill files: %w", err)
	}

	payload := metadata.PublishPayload(fm, pubChangelog)
	status, body, err := postPublishMultipart(host, token, payload, skillFiles)
	if err != nil {
		return err
	}

	if status >= 200 && status < 300 {
		if pubJSON {
			data, _ := json.MarshalIndent(body, "", "  ")
			fmt.Println(string(data))
		} else {
			fmt.Printf("✓ Published: skillId=%v status=%v\n", body["skillId"], body["status"])
			if url, ok := body["publicUrl"].(string); ok && url != "" {
				fmt.Printf("  url: %s\n", url)
			}
		}
		return nil
	}

	errMsg := formatPublishError(status, body)
	if pubJSON {
		data, _ := json.Marshal(map[string]interface{}{"success": false, "status": status, "error": errMsg, "body": body})
		fmt.Println(string(data))
		return fmt.Errorf("%s", errMsg)
	}
	return fmt.Errorf("%s", errMsg)
}

func resolvePublishToken() (token, host string) {
	token = strings.TrimSpace(pubToken)
	if token == "" {
		token = os.Getenv(config.EnvToken)
	}
	host = strings.TrimSpace(pubHost)
	if host == "" {
		host = os.Getenv(config.EnvAPIBase)
	}
	if host == "" {
		if user := credentials.GetUser(); user != nil {
			if token == "" {
				token = user.Token
			}
			if host == "" {
				host = user.Host
			}
		}
	}
	if host == "" {
		host = config.DefaultEnterpriseHost
	}
	return
}

func postPublishMultipart(host, token string, payload map[string]interface{}, files []metadata.FileEntry) (int, map[string]interface{}, error) {
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)

	payloadJSON, _ := json.Marshal(payload)
	part, _ := w.CreateFormField("payload")
	part.Write(payloadJSON)
	// Force content-type for payload
	w.WriteField("payload", string(payloadJSON))

	// Rebuild with proper content-type
	buf.Reset()
	w2 := multipart.NewWriter(&buf)

	// Payload part with application/json
	h := make(map[string][]string)
	h["Content-Type"] = []string{"application/json"}
	pw, _ := w2.CreatePart(mapToMIMEHeader(map[string]string{
		"Content-Disposition": `form-data; name="payload"`,
		"Content-Type":        "application/json",
	}))
	pw.Write(payloadJSON)

	// File parts
	for _, fe := range files {
		ct := "application/octet-stream"
		if strings.HasSuffix(strings.ToLower(fe.RelPath), ".md") {
			ct = "text/markdown"
		}
		pw, _ := w2.CreatePart(mapToMIMEHeader(map[string]string{
			"Content-Disposition": fmt.Sprintf(`form-data; name="files"; filename="%s"`, fe.RelPath),
			"Content-Type":        ct,
		}))
		pw.Write(fe.Content)
	}
	w2.Close()

	apiURL := strings.TrimRight(host, "/") + "/api/v1/community/skills/publish"
	req, _ := http.NewRequest("POST", apiURL, &buf)
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", w2.FormDataContentType())
	req.Header.Set("Accept", "application/json")

	client := &http.Client{Timeout: 120 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return 0, nil, fmt.Errorf("network error: %w", err)
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(resp.Body)

	var parsed map[string]interface{}
	if err := json.Unmarshal(respBody, &parsed); err != nil {
		return resp.StatusCode, map[string]interface{}{"raw": string(respBody)}, nil
	}
	return resp.StatusCode, parsed, nil
}

func mapToMIMEHeader(m map[string]string) map[string][]string {
	out := make(map[string][]string)
	for k, v := range m {
		out[k] = []string{v}
	}
	return out
}

func formatPublishError(status int, body map[string]interface{}) string {
	msg := strValPublish(body, "error")
	if msg == "" {
		if raw, ok := body["raw"].(string); ok {
			msg = raw
		}
	}
	code := strValPublish(body, "code")
	switch status {
	case 401:
		return "please run: skillhub auth login --token skh_xxx"
	case 403:
		return fmt.Sprintf("permission denied: %s", msg)
	case 409:
		return fmt.Sprintf("slug conflict: %s", msg)
	case 429:
		rule := strValPublish(body, "rule")
		retryAfter := body["retryAfter"]
		hint := ""
		if retryAfter != nil {
			hint = fmt.Sprintf(", please retry after %v seconds", retryAfter)
		}
		if rule != "" {
			return fmt.Sprintf("rate limited (rule %s)%s: %s", rule, hint, msg)
		}
		return fmt.Sprintf("rate limited%s: %s", hint, msg)
	}
	if status >= 400 && status < 500 {
		if code != "" {
			return fmt.Sprintf("request failed (%d): %s (code=%s)", status, msg, code)
		}
		return fmt.Sprintf("request failed (%d): %s", status, msg)
	}
	return fmt.Sprintf("server error (%d): %s", status, msg)
}

func strValPublish(m map[string]interface{}, key string) string {
	if v, ok := m[key].(string); ok {
		return strings.TrimSpace(v)
	}
	return ""
}
