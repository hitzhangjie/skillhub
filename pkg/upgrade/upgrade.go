package upgrade

import (
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/hitzhangjie/skillhub/pkg/config"
	"github.com/hitzhangjie/skillhub/pkg/index"
	"github.com/hitzhangjie/skillhub/pkg/install"
	"github.com/hitzhangjie/skillhub/pkg/lockfile"
	"github.com/hitzhangjie/skillhub/pkg/version"
)

func Run(slug, installRoot string, checkOnly bool, timeout int) (int, error) {
	absRoot, err := filepath.Abs(config.ExpandPath(installRoot))
	if err != nil {
		return 1, err
	}

	lock := lockfile.Load(absRoot)
	skills := lock.Skills
	if skills == nil {
		skills = make(map[string]lockfile.SkillMeta)
	}

	var targets []string
	if slug != "" {
		targets = []string{slug}
	} else {
		for s := range skills {
			targets = append(targets, s)
		}
		if len(targets) == 0 {
			return 0, fmt.Errorf("no installed skills in lockfile: %s", filepath.Join(absRoot, config.LockfileName))
		}
	}

	checked := 0
	upgraded := 0
	skipped := 0
	failed := 0

	for _, s := range targets {
		checked++
		targetDir := filepath.Join(absRoot, s)
		if _, err := os.Stat(targetDir); os.IsNotExist(err) {
			fmt.Printf("[%s] skip: skill directory not found: %s\n", s, targetDir)
			skipped++
			continue
		}

		lockMeta, _ := skills[s]
		configPath := filepath.Join(targetDir, config.SkillConfigName)
		if _, err := os.Stat(configPath); os.IsNotExist(err) {
			fmt.Printf("[%s] skip: %s not found\n", s, config.SkillConfigName)
			skipped++
			continue
		}

		rawConfigData, err := os.ReadFile(configPath)
		if err != nil {
			fmt.Printf("[%s] fail: cannot read %s: %v\n", s, config.SkillConfigName, err)
			failed++
			continue
		}
		preservedConfigText := string(rawConfigData)

		var rawConfig map[string]interface{}
		if err := json.Unmarshal(rawConfigData, &rawConfig); err != nil {
			fmt.Printf("[%s] fail: invalid %s: %v\n", s, config.SkillConfigName, err)
			failed++
			continue
		}

		updateURL := extractUpdateURL(rawConfig, targetDir)
		if updateURL == "" {
			fmt.Printf("[%s] skip: missing update URL in %s\n", s, config.SkillConfigName)
			skipped++
			continue
		}

		manifest, err := index.ReadJSONFromURI(updateURL, timeout)
		if err != nil {
			fmt.Printf("[%s] fail: %v\n", s, err)
			failed++
			continue
		}
		manifestMap, ok := manifest.(map[string]interface{})
		if !ok {
			fmt.Printf("[%s] fail: update manifest must be a JSON object: %s\n", s, updateURL)
			failed++
			continue
		}

		latestVersion, packageURI, expectedSHA := extractUpdateManifestInfo(manifestMap)
		if latestVersion == "" {
			fmt.Printf("[%s] fail: update manifest missing version: %s\n", s, updateURL)
			failed++
			continue
		}
		if packageURI == "" {
			fmt.Printf("[%s] fail: update manifest missing package URL: %s\n", s, updateURL)
			failed++
			continue
		}

		currentVersion := readInstalledVersion(targetDir, lockMeta)
		if !version.IsNewer(latestVersion, currentVersion) {
			fmt.Printf("[%s] up-to-date: current=%s latest=%s\n", s, orUnknown(currentVersion), latestVersion)
			skipped++
			continue
		}

		packageURI = resolveURIWithBase(packageURI, targetDir)
		if checkOnly {
			fmt.Printf("[%s] upgrade available: current=%s latest=%s package=%s\n",
				s, orUnknown(currentVersion), latestVersion, packageURI)
			continue
		}

		if err := install.InstallZipToTarget(s, packageURI, targetDir, true, expectedSHA); err != nil {
			fmt.Printf("[%s] fail: %v\n", s, err)
			failed++
			continue
		}

		// Restore config.json if overwritten
		restoredConfigPath := filepath.Join(targetDir, config.SkillConfigName)
		if _, err := os.Stat(restoredConfigPath); os.IsNotExist(err) {
			os.WriteFile(restoredConfigPath, []byte(preservedConfigText), 0644)
		}

		lockMeta.Version = latestVersion
		lockMeta.ZipURL = packageURI
		lockMeta.UpdateURL = updateURL
		if lockMeta.Name == "" {
			lockMeta.Name = s
		}
		if lockMeta.Source == "" {
			lockMeta.Source = "unknown"
		}
		skills[s] = lockMeta
		upgraded++
		fmt.Printf("[%s] upgraded: %s -> %s\n", s, orUnknown(currentVersion), latestVersion)
	}

	lock.Skills = skills
	lockfile.Save(absRoot, lock)
	fmt.Printf("upgrade done: checked=%d upgraded=%d skipped=%d failed=%d dir=%s\n",
		checked, upgraded, skipped, failed, absRoot)
	if failed > 0 {
		return 2, nil
	}
	return 0, nil
}

func orUnknown(v string) string {
	if v == "" {
		return "<unknown>"
	}
	return v
}

func extractUpdateURL(config map[string]interface{}, skillDir string) string {
	for _, key := range []string{"update_url", "updateUrl", "upgrade_url", "upgradeUrl", "manifest_url", "manifestUrl"} {
		if v := strVal(config, key); v != "" {
			return resolveURIWithBase(v, skillDir)
		}
	}
	for _, containerKey := range []string{"update", "upgrade", "autoupdate"} {
		if nested, ok := config[containerKey].(map[string]interface{}); ok {
			for _, key := range []string{"url", "uri", "manifest", "manifest_url"} {
				if v := strVal(nested, key); v != "" {
					return resolveURIWithBase(v, skillDir)
				}
			}
		}
	}
	return ""
}

func extractUpdateManifestInfo(manifest map[string]interface{}) (version, packageURI, sha256 string) {
	candidates := []map[string]interface{}{manifest}
	for _, key := range []string{"latest", "release", "data", "skill", "package"} {
		if nested, ok := manifest[key].(map[string]interface{}); ok {
			candidates = append(candidates, nested)
		}
	}
	for _, item := range candidates {
		if version == "" {
			version = firstNonEmpty(item, "version", "latest_version", "latestVersion")
		}
		if packageURI == "" {
			packageURI = firstNonEmpty(item, "zip_url", "zipUrl", "download_url", "downloadUrl", "package_url", "packageUrl", "url")
		}
		if sha256 == "" {
			sha256 = firstNonEmpty(item, "sha256", "sha_256", "checksum")
		}
	}
	return version, packageURI, strings.ToLower(sha256)
}

func readInstalledVersion(skillDir string, lockMeta lockfile.SkillMeta) string {
	if lockMeta.Version != "" {
		return lockMeta.Version
	}
	metaPath := filepath.Join(skillDir, config.SkillMetaName)
	if data, err := os.ReadFile(metaPath); err == nil {
		var raw map[string]interface{}
		if json.Unmarshal(data, &raw) == nil {
			if v := strVal(raw, "version"); v != "" {
				return v
			}
		}
	}
	return ""
}

func resolveURIWithBase(raw string, baseDir string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	parsed, err := url.Parse(raw)
	if err != nil {
		return ""
	}
	if parsed.Scheme == "http" || parsed.Scheme == "https" {
		return raw
	}
	if parsed.Scheme == "file" {
		abs, _ := filepath.Abs(parsed.Path)
		return "file://" + abs
	}
	abs, _ := filepath.Abs(filepath.Join(baseDir, raw))
	return "file://" + abs
}

func strVal(m map[string]interface{}, key string) string {
	if v, ok := m[key].(string); ok {
		return strings.TrimSpace(v)
	}
	return ""
}

func firstNonEmpty(m map[string]interface{}, keys ...string) string {
	for _, k := range keys {
		if v := strVal(m, k); v != "" {
			return v
		}
	}
	return ""
}
