package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/hitzhangjie/skillhub/pkg/config"
	"github.com/hitzhangjie/skillhub/pkg/credentials"
	"github.com/hitzhangjie/skillhub/pkg/enterprise"
	"github.com/hitzhangjie/skillhub/pkg/index"
	"github.com/hitzhangjie/skillhub/pkg/install"
	"github.com/hitzhangjie/skillhub/pkg/lockfile"
	"github.com/spf13/cobra"
)

var (
	installFilesBaseURI        string
	installDownloadURLTemplate string
	installPrimaryDownloadURL  string
	installSearchURL           string
	installSearchLimit         int
	installSearchTimeout       int
	installForce               bool
	installSecret              string
	installJSON                bool
)

var installCmd = &cobra.Command{
	Use:   "install <slug|@org/slug[@version]>",
	Short: "Install a skill by slug",
	Long: `Install a skill from the community or enterprise registry.

Supports three formats:
  skillhub install my-skill               # Community skill
  skillhub install @org/my-skill           # Enterprise skill (requires login)
  skillhub install @org/my-skill@1.0.0     # Specific enterprise skill version
  skillhub install my-skill --secret sk-ent-xxx  # Enterprise direct download`,
	Example: `  skillhub install my-skill
  skillhub install @tencent/my-skill
  skillhub install @tencent/my-skill@1.0.0
  skillhub install my-skill --secret sk-ent-abc123
  skillhub install --json my-skill
  skillhub install --force my-skill`,
	Args: cobra.ExactArgs(1),
	RunE: runInstall,
}

func init() {
	installCmd.Flags().StringVar(&installFilesBaseURI, "files-base-uri", "", "Base URI for local archives")
	installCmd.Flags().StringVar(&installDownloadURLTemplate, "download-url-template", config.Metadata.SkillsDownloadURLTpl, "Fallback download URL template")
	installCmd.Flags().StringVar(&installPrimaryDownloadURL, "primary-download-url-template", config.Metadata.SkillsPrimaryDownloadURLTpl, "Primary download URL template")
	installCmd.Flags().StringVar(&installSearchURL, "search-url", config.Metadata.SkillsSearchURL, "Remote search API URL")
	installCmd.Flags().IntVar(&installSearchLimit, "search-limit", 20, "Remote search limit")
	installCmd.Flags().IntVar(&installSearchTimeout, "search-timeout", 6, "Remote search timeout in seconds")
	installCmd.Flags().BoolVar(&installForce, "force", false, "Overwrite existing target directory")
	installCmd.Flags().StringVar(&installSecret, "secret", "", "Enterprise API key for direct download")
	installCmd.Flags().BoolVar(&installJSON, "json", false, "JSON output")
	rootCmd.AddCommand(installCmd)
}

func runInstall(cmd *cobra.Command, args []string) error {
	rawSlug := strings.TrimSpace(args[0])
	if installSecret == "" {
		installSecret = os.Getenv(config.EnvSecret)
	}

	// Parse @org/slug@version
	org, slug, version := parseSkillRef(rawSlug)

	if org != "" {
		return installEnterpriseSkill(org, slug, version)
	}
	return installCommunitySkill(slug)
}

func parseSkillRef(ref string) (org, slug, version string) {
	if strings.HasPrefix(ref, "@") {
		parts := strings.SplitN(ref[1:], "/", 2)
		if len(parts) == 2 {
			org = parts[0]
			ref = parts[1]
		}
	}
	if idx := strings.LastIndex(ref, "@"); idx > 0 {
		version = ref[idx+1:]
		slug = ref[:idx]
	} else {
		slug = ref
	}
	return
}

func installCommunitySkill(slug string) error {
	root := config.ExpandPath(installRoot)
	absRoot, _ := filepath.Abs(root)
	targetDir := filepath.Join(absRoot, slug)

	// Try loading from index
	var skill *index.SkillDescriptor
	idx, err := index.Load(indexURI, 20)
	if err == nil {
		skill = index.Find(idx, slug)
	}
	if skill == nil {
		// Fallback: remote search
		fromSearch := fmt.Sprintf(`info: "%s" not in index, trying direct download by slug`, slug)
		skill = &index.SkillDescriptor{Slug: slug, Name: slug, Source: "community"}
		fmt.Fprintln(os.Stderr, fromSearch)
	}

	primaryZipURL := index.FillSlugTemplate(installPrimaryDownloadURL, slug)
	if primaryZipURL == "" {
		errMsg := fmt.Sprintf("primary download URL template resolved empty URL for %s", slug)
		if installJSON {
			data, _ := json.Marshal(map[string]interface{}{"success": false, "slug": slug, "error": errMsg})
			fmt.Println(string(data))
			return fmt.Errorf("%s", errMsg)
		}
		return fmt.Errorf("%s", errMsg)
	}

	// Conflict detection
	lock := lockfile.Load(absRoot)
	skills := lock.Skills
	if skills == nil {
		skills = make(map[string]lockfile.SkillMeta)
	}
	existing, exists := skills[slug]
	force := installForce
	if exists && strings.HasPrefix(existing.Source, "@") {
		fmt.Fprintf(os.Stderr, "⚠️  Replacing existing skill \"%s\" (%s v%s) with community %s\n",
			slug, existing.Source, existing.Version, slug)
		force = true
	}

	expectedSHA := strings.TrimSpace(strings.ToLower(skill.SHA256))

	var savedStderr *os.File
	if installJSON {
		savedStderr = os.Stderr
		os.Stderr, _ = os.Open(os.DevNull)
	}

	err = install.InstallZipToTargetWithFallback(
		slug,
		[]string{primaryZipURL},
		targetDir,
		force,
		expectedSHA,
		installJSON,
	)

	if installJSON {
		if savedStderr != nil {
			os.Stderr.Close()
			os.Stderr = savedStderr
		}
	}

	if err != nil {
		if installJSON {
			data, _ := json.Marshal(map[string]interface{}{"success": false, "slug": slug, "error": err.Error()})
			fmt.Println(string(data))
		}
		return err
	}

	installedAt := time.Now().UTC().Format("2006-01-02T15:04:05Z")
	skillVersion := strings.TrimSpace(skill.Version)
	skills[slug] = lockfile.SkillMeta{
		Name:        skill.Name,
		ZipURL:      primaryZipURL,
		Source:      "community",
		Version:     skillVersion,
		InstalledAt: installedAt,
	}
	lock.Skills = skills
	lockfile.Save(absRoot, lock)

	if installJSON {
		data, _ := json.Marshal(map[string]interface{}{
			"success":     true,
			"slug":        slug,
			"name":        skill.Name,
			"version":     skillVersion,
			"source":      "community",
			"installedAt": installedAt,
			"targetDir":   targetDir,
		})
		fmt.Println(string(data))
	} else {
		fmt.Printf("✓ Installed: %s -> %s\n", slug, targetDir)
	}
	return nil
}

func installEnterpriseSkill(orgSlug, slug, version string) error {
	root := config.ExpandPath(installRoot)
	absRoot, _ := filepath.Abs(root)
	targetDir := filepath.Join(absRoot, slug)

	var host string
	var orgID int
	var apiKey string

	if installSecret != "" {
		if !strings.HasPrefix(installSecret, "sk-ent-") {
			errMsg := "--secret must be an enterprise API key (sk-ent-...)"
			if installJSON {
				data, _ := json.Marshal(map[string]interface{}{"success": false, "slug": slug, "error": errMsg})
				fmt.Println(string(data))
			}
			return fmt.Errorf("%s", errMsg)
		}
		host = os.Getenv(config.EnvHost)
		if host == "" {
			host = config.DefaultEnterpriseHost
		}
		orgInfo, err := enterprise.VerifyAPIKey(host, installSecret)
		if err != nil {
			if installJSON {
				data, _ := json.Marshal(map[string]interface{}{"success": false, "slug": slug, "error": err.Error()})
				fmt.Println(string(data))
			}
			return err
		}
		orgID = orgInfo.OrgID
		apiKey = installSecret
	} else {
		cred := credentials.GetOrg(orgSlug)
		if cred == nil {
			errMsg := fmt.Sprintf("not logged in to @%s. run: skillhub login --key <your-api-key>", orgSlug)
			if installJSON {
				data, _ := json.Marshal(map[string]interface{}{"success": false, "slug": slug, "error": errMsg})
				fmt.Println(string(data))
			}
			return fmt.Errorf("%s", errMsg)
		}
		host = cred.Host
		if host == "" {
			host = config.DefaultEnterpriseHost
		}
		orgID = cred.OrgID
		apiKey = cred.APIKey
	}

	if orgID == 0 || apiKey == "" {
		errMsg := fmt.Sprintf("invalid credentials for @%s. run: skillhub login --key <your-api-key>", orgSlug)
		if installJSON {
			data, _ := json.Marshal(map[string]interface{}{"success": false, "slug": slug, "error": errMsg})
			fmt.Println(string(data))
		}
		return fmt.Errorf("%s", errMsg)
	}

	// Conflict detection
	lock := lockfile.Load(absRoot)
	skills := lock.Skills
	if skills == nil {
		skills = make(map[string]lockfile.SkillMeta)
	}
	force := installForce
	if existing, exists := skills[slug]; exists {
		if existing.Source != fmt.Sprintf("@%s", orgSlug) {
			fmt.Fprintf(os.Stderr, "⚠️  Replacing existing skill \"%s\" (%s v%s) with @%s/%s\n",
				slug, existing.Source, existing.Version, orgSlug, slug)
			force = true
		}
	}

	installedVersion, err := enterprise.DownloadSkill(host, orgID, apiKey, slug, version, targetDir, force)
	if err != nil {
		if installJSON {
			data, _ := json.Marshal(map[string]interface{}{"success": false, "slug": slug, "error": err.Error()})
			fmt.Println(string(data))
		}
		return err
	}
	if installedVersion == "" {
		errMsg := fmt.Sprintf("skill \"%s\" not found in @%s", slug, orgSlug)
		if installJSON {
			data, _ := json.Marshal(map[string]interface{}{"success": false, "slug": slug, "error": errMsg})
			fmt.Println(string(data))
		}
		return fmt.Errorf("%s", errMsg)
	}

	installedAt := time.Now().UTC().Format("2006-01-02T15:04:05Z")
	skills[slug] = lockfile.SkillMeta{
		Name:        slug,
		Source:      fmt.Sprintf("@%s", orgSlug),
		Org:         orgSlug,
		Host:        host,
		Version:     installedVersion,
		InstalledAt: installedAt,
	}
	lock.Skills = skills
	lockfile.Save(absRoot, lock)

	versionDisplay := ""
	if installedVersion != "latest" {
		versionDisplay = fmt.Sprintf("@%s", installedVersion)
	}

	if installJSON {
		data, _ := json.Marshal(map[string]interface{}{
			"success":     true,
			"slug":        slug,
			"name":        slug,
			"version":     installedVersion,
			"source":      fmt.Sprintf("@%s", orgSlug),
			"org":         orgSlug,
			"installedAt": installedAt,
			"targetDir":   targetDir,
		})
		fmt.Println(string(data))
	} else {
		fmt.Printf("✓ Installed: @%s/%s%s -> %s\n", orgSlug, slug, versionDisplay, targetDir)
	}
	return nil
}
