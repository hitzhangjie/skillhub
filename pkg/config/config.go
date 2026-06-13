package config

import (
	_ "embed"
	"os"
	"path/filepath"
	"strings"
)

// Env var names
const (
	EnvSkipWorkspaceSkills    = "SKILLHUB_SKIP_WORKSPACE_SKILLS"
	EnvOrg                    = "SKILLHUB_ORG"
	EnvAPIKey                 = "SKILLHUB_API_KEY"
	EnvHost                   = "SKILLHUB_HOST"
	EnvSecret                 = "SKILLHUB_SECRET"
	EnvToken                  = "SKILLHUB_TOKEN"
	EnvAPIBase                = "SKILLHUB_API_BASE"
	EnvSearchURL              = "SKILLHUB_SEARCH_URL"
	EnvPrimaryDownloadURLTmpl = "SKILLHUB_PRIMARY_DOWNLOAD_URL_TEMPLATE"
	EnvConfigPath             = "SKILLHUB_CONFIG_PATH"
	EnvVerboseLog             = "LOG"
)

// Defaults
const (
	DefaultCLIHome        = "~/.skillhub"
	DefaultInstallRoot    = "~/.skillhub/skills"
	DefaultEnterpriseHost = "https://api.skillhub.cn"

	LockfileName        = ".skills_store_lock.json"
	SkillConfigName     = "config.json"
	SkillMetaName       = "_meta.json"
	CLIConfigName       = "config.json"
	CredentialsFileName = "credentials.json"

	PostUpgradeSkillMigrationMinVersionMajor = 3
	PostUpgradeSkillMigrationMinVersionMinor = 13

	FindSkillsSlug         = "find-skills"
	SkillhubPreferenceSlug = "skillhub-preference"
)

type CLIMetadata struct {
	SkillsIndexURL              string `json:"skills_index_url"`
	SkillsSearchURL             string `json:"skills_search_url"`
	SkillsPrimaryDownloadURLTpl string `json:"skills_primary_download_url_template"`
	SkillsDownloadURLTpl        string `json:"skills_download_url_template"`
}

var (
	Version  = "dev" // set via ldflags
	Metadata *CLIMetadata
)

func init() {
	const (
		defaultIndexURIFallback               = "https://skillhub-1388575217.cos.ap-guangzhou.myqcloud.com/skills.json"
		defaultSearchURLFallback              = "https://api.skillhub.cn/api/v1/search"
		defaultSkillsDownloadURLTmplFallback  = "https://skillhub-1388575217.cos.ap-guangzhou.myqcloud.com/skills/{slug}.zip"
		defaultPrimaryDownloadURLTmplFallback = "https://api.skillhub.cn/api/v1/download?slug={slug}"
	)

	Metadata = &CLIMetadata{
		SkillsIndexURL:              defaultIndexURIFallback,
		SkillsSearchURL:             defaultSearchURLFallback,
		SkillsPrimaryDownloadURLTpl: defaultPrimaryDownloadURLTmplFallback,
		SkillsDownloadURLTpl:        defaultSkillsDownloadURLTmplFallback,
	}
}

func CLIUserAgent() string {
	return "skills-store-cli/" + Version
}

func ExpandPath(p string) string {
	if strings.HasPrefix(p, "~/") {
		home, _ := os.UserHomeDir()
		return filepath.Join(home, p[2:])
	}
	return p
}

func GetCLIHome() string {
	return ExpandPath(DefaultCLIHome)
}

func GetConfigPath() string {
	if v := os.Getenv(EnvConfigPath); v != "" {
		return ExpandPath(v)
	}
	return filepath.Join(GetCLIHome(), CLIConfigName)
}

func GetCredentialsPath() string {
	return filepath.Join(GetCLIHome(), CredentialsFileName)
}

func ParseBoolLike(v string) (bool, bool) {
	v = strings.TrimSpace(strings.ToLower(v))
	switch v {
	case "1", "true", "yes", "on":
		return true, true
	case "0", "false", "no", "off":
		return false, true
	}
	return false, false
}

func VerboseEnabled() bool {
	v, _ := ParseBoolLike(os.Getenv(EnvVerboseLog))
	return v
}
