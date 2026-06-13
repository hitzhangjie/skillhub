package enterprise

import (
	"archive/zip"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/hitzhangjie/skillhub/pkg/config"
	"github.com/hitzhangjie/skillhub/pkg/index"
)

type VerifyResponse struct {
	OrgID    int    `json:"orgId"`
	OrgOrgID string `json:"orgOrgId"`
	OrgSlug  string `json:"orgSlug"`
	OrgName  string `json:"orgName"`
}

func VerifyAPIKey(host, apiKey string) (*VerifyResponse, error) {
	apiURL := strings.TrimRight(host, "/") + "/api/v1/registry/verify"
	client := &http.Client{Timeout: 10 * time.Second}
	req, _ := http.NewRequest("POST", apiURL, nil)
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("User-Agent", config.CLIUserAgent())
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("cannot connect to %s: %w", host, err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode == 401 {
		var errResp struct {
			Error string `json:"error"`
		}
		json.Unmarshal(body, &errResp)
		msg := errResp.Error
		if msg == "" {
			msg = "invalid or expired API key"
		}
		return nil, fmt.Errorf("%s", msg)
	}
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("verify request failed (HTTP %d)", resp.StatusCode)
	}

	var vr VerifyResponse
	if err := json.Unmarshal(body, &vr); err != nil {
		return nil, fmt.Errorf("unexpected response from verify endpoint")
	}
	if vr.OrgID == 0 {
		return nil, fmt.Errorf("unexpected response from verify endpoint")
	}
	return &vr, nil
}

func Search(host string, orgID int, apiKey, query string, limit int) ([]index.SkillDescriptor, bool) {
	apiURL := fmt.Sprintf("%s/api/v1/orgs/%d/registry/search", strings.TrimRight(host, "/"), orgID)
	params := url.Values{}
	params.Set("q", query)
	params.Set("pageSize", fmt.Sprintf("%d", limit))
	fullURL := apiURL + "?" + params.Encode()

	client := &http.Client{Timeout: 3 * time.Second}
	req, _ := http.NewRequest("GET", fullURL, nil)
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("User-Agent", config.CLIUserAgent())
	req.Header.Set("Accept", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return nil, false
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)

	var raw map[string]interface{}
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, false
	}
	skills, ok := raw["skills"].([]interface{})
	if !ok {
		return nil, false
	}

	var out []index.SkillDescriptor
	for _, s := range skills {
		m, ok := s.(map[string]interface{})
		if !ok {
			continue
		}
		slug := strVal(m, "slug")
		if slug == "" {
			continue
		}
		name := strVal(m, "displayName")
		if name == "" {
			name = strVal(m, "name")
		}
		if name == "" {
			name = slug
		}
		out = append(out, index.SkillDescriptor{
			Slug:        slug,
			Name:        name,
			Description: strOrDefault(m, "summary", strVal(m, "description")),
			Summary:     strVal(m, "summary"),
			Version:     strVal(m, "version"),
		})
	}
	return out, true
}

func DownloadSkill(host string, orgID int, apiKey, slug, version, targetDir string, force bool) (string, error) {
	apiURL := fmt.Sprintf("%s/api/v1/orgs/%d/registry/skills/%s/download",
		strings.TrimRight(host, "/"), orgID, url.PathEscape(slug))
	if version != "" {
		apiURL += "?version=" + url.QueryEscape(version)
	}

	client := &http.Client{Timeout: 30 * time.Second}
	req, _ := http.NewRequest("GET", apiURL, nil)
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("User-Agent", config.CLIUserAgent())

	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("cannot connect to %s: %w", host, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == 404 {
		return "", nil
	}
	if resp.StatusCode >= 400 {
		return "", fmt.Errorf("download failed (HTTP %d)", resp.StatusCode)
	}

	content, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	detectedVersion := version
	if detectedVersion == "" {
		cd := resp.Header.Get("Content-Disposition")
		if cd != "" {
			re := regexp.MustCompile(`filename="?([^"]+)"?`)
			if m := re.FindStringSubmatch(cd); m != nil {
				fname := m[1]
				if strings.HasPrefix(fname, slug+"-") && strings.HasSuffix(fname, ".zip") {
					detectedVersion = strings.TrimSuffix(strings.TrimPrefix(fname, slug+"-"), ".zip")
				}
			}
		}
	}
	if detectedVersion == "" {
		detectedVersion = "latest"
	}

	if force {
		os.RemoveAll(targetDir)
	}
	os.MkdirAll(targetDir, 0755)

	tmpFile, err := os.CreateTemp("", "skillhub-ent-*.zip")
	if err != nil {
		return "", err
	}
	tmpPath := tmpFile.Name()
	tmpFile.Write(content)
	tmpFile.Close()
	defer os.Remove(tmpPath)

	if err := safeExtractZipInline(tmpPath, targetDir); err != nil {
		return "", err
	}
	return detectedVersion, nil
}

func safeExtractZipInline(zipPath, targetDir string) error {
	r, err := zip.OpenReader(zipPath)
	if err != nil {
		return fmt.Errorf("not a valid zip archive: %w", err)
	}
	defer r.Close()

	for _, f := range r.File {
		name := f.Name
		parts := strings.Split(name, "/")
		for _, p := range parts {
			if p == ".." {
				return fmt.Errorf("unsafe zip path entry: %s", name)
			}
		}
		if strings.HasPrefix(name, "/") {
			return fmt.Errorf("unsafe zip path entry: %s", name)
		}

		dest := filepath.Join(targetDir, filepath.FromSlash(name))
		if f.FileInfo().IsDir() {
			os.MkdirAll(dest, 0755)
			continue
		}
		os.MkdirAll(filepath.Dir(dest), 0755)
		rc, err := f.Open()
		if err != nil {
			return fmt.Errorf("failed to open zip entry %s: %w", name, err)
		}
		out, err := os.Create(dest)
		if err != nil {
			rc.Close()
			return fmt.Errorf("failed to create %s: %w", dest, err)
		}
		_, err = io.Copy(out, rc)
		rc.Close()
		out.Close()
		if err != nil {
			return fmt.Errorf("failed to extract %s: %w", name, err)
		}
	}
	return nil
}

func GetAuthMe(host, token string) (map[string]interface{}, error) {
	apiURL := strings.TrimRight(host, "/") + "/api/v1/auth/me"
	client := &http.Client{Timeout: 10 * time.Second}
	req, _ := http.NewRequest("GET", apiURL, nil)
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("network error: %w", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)

	var raw map[string]interface{}
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, fmt.Errorf("auth/me response not valid JSON: %w", err)
	}
	if user, ok := raw["user"].(map[string]interface{}); ok {
		return user, nil
	}
	return raw, nil
}

func strVal(m map[string]interface{}, key string) string {
	if v, ok := m[key].(string); ok {
		return strings.TrimSpace(v)
	}
	return ""
}

func strOrDefault(m map[string]interface{}, key, fallback string) string {
	v := strVal(m, key)
	if v == "" {
		return fallback
	}
	return v
}
