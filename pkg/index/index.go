package index

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/hitzhangjie/skillhub/pkg/config"
)

type SkillDescriptor struct {
	Slug        string   `json:"slug"`
	Name        string   `json:"name,omitempty"`
	DisplayName string   `json:"displayName,omitempty"`
	Description string   `json:"description,omitempty"`
	Summary     string   `json:"summary,omitempty"`
	Version     string   `json:"version,omitempty"`
	Tags        []string `json:"tags,omitempty"`
	Categories  []string `json:"categories,omitempty"`
	ZipURL      string   `json:"zip_url,omitempty"`
	SHA256      string   `json:"sha256,omitempty"`
	Homepage    string   `json:"homepage,omitempty"`
	Source      string   `json:"_source,omitempty"`
	Org         string   `json:"_org,omitempty"`
}

type Index struct {
	Skills []SkillDescriptor `json:"skills"`
}

func Load(uri string, timeout int) (*Index, error) {
	data, err := ReadJSONFromURI(uri, timeout)
	if err != nil {
		return nil, err
	}
	return normalizePayload(data)
}

func LoadLocal(path string) (*Index, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var raw interface{}
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("invalid JSON in %s: %w", path, err)
	}
	return normalizePayload(raw)
}

func normalizePayload(data interface{}) (*Index, error) {
	idx := &Index{}
	switch v := data.(type) {
	case map[string]interface{}:
		if skills, ok := v["skills"].([]interface{}); ok {
			for _, s := range skills {
				if desc := mapToDescriptor(s); desc != nil {
					idx.Skills = append(idx.Skills, *desc)
				}
			}
		}
	case []interface{}:
		for _, s := range v {
			if desc := mapToDescriptor(s); desc != nil {
				idx.Skills = append(idx.Skills, *desc)
			}
		}
	default:
		return nil, fmt.Errorf("index JSON must be an object or array")
	}
	return idx, nil
}

func mapToDescriptor(item interface{}) *SkillDescriptor {
	m, ok := item.(map[string]interface{})
	if !ok {
		return nil
	}
	d := &SkillDescriptor{}
	if v, ok := m["slug"].(string); ok {
		d.Slug = v
	}
	if v, ok := m["name"].(string); ok {
		d.Name = v
	}
	if v, ok := m["displayName"].(string); ok {
		d.DisplayName = v
	}
	if v, ok := m["description"].(string); ok {
		d.Description = v
	}
	if v, ok := m["summary"].(string); ok {
		d.Summary = v
	}
	if v, ok := m["version"].(string); ok {
		d.Version = v
	}
	if tags, ok := m["tags"].([]interface{}); ok {
		for _, t := range tags {
			if s, ok := t.(string); ok {
				d.Tags = append(d.Tags, s)
			}
		}
	}
	if cats, ok := m["categories"].([]interface{}); ok {
		for _, c := range cats {
			if s, ok := c.(string); ok {
				d.Categories = append(d.Categories, s)
			}
		}
	}
	if v, ok := m["zip_url"].(string); ok {
		d.ZipURL = v
	}
	if v, ok := m["sha256"].(string); ok {
		d.SHA256 = v
	}
	if v, ok := m["homepage"].(string); ok {
		d.Homepage = v
	}
	if v, ok := m["_source"].(string); ok {
		d.Source = v
	}
	if v, ok := m["_org"].(string); ok {
		d.Org = v
	}
	return d
}

func Find(idx *Index, slug string) *SkillDescriptor {
	for i := range idx.Skills {
		if strings.TrimSpace(idx.Skills[i].Slug) == slug {
			return &idx.Skills[i]
		}
	}
	return nil
}

func SearchText(d *SkillDescriptor) string {
	parts := []string{
		d.Slug, d.Name, d.Description, d.Summary, d.Version,
		strings.Join(d.Tags, " "),
		strings.Join(d.Categories, " "),
	}
	return strings.ToLower(strings.Join(parts, " "))
}

func SkillZipURI(skill *SkillDescriptor, slug, filesBaseURI, downloadURLTemplate string, indexPath string) string {
	// Try filesBaseURI first
	if strings.TrimSpace(filesBaseURI) != "" {
		u := appendSlugZip(filesBaseURI, slug)
		if u != "" {
			return u
		}
	}
	// Try sibling files/ dir if local index
	if indexPath != "" {
		sibling := filepath.Join(filepath.Dir(indexPath), "files", slug+".zip")
		if _, err := os.Stat(sibling); err == nil {
			return "file://" + sibling
		}
	}
	// Try skill's own zip_url
	for _, key := range []string{skill.ZipURL} {
		if key != "" {
			if parsed, err := url.Parse(key); err == nil && parsed.Scheme != "" {
				return key
			}
			abs, _ := filepath.Abs(key)
			return "file://" + abs
		}
	}
	// Fallback to download URL template
	if strings.TrimSpace(downloadURLTemplate) != "" {
		return appendSlugZip(downloadURLTemplate, slug)
	}
	return ""
}

func appendSlugZip(base, slug string) string {
	base = strings.TrimSpace(base)
	if base == "" {
		return ""
	}
	if strings.Contains(base, "{slug}") {
		return strings.ReplaceAll(base, "{slug}", url.PathEscape(slug))
	}
	parsed, err := url.Parse(base)
	if err != nil {
		return ""
	}
	if parsed.Scheme == "http" || parsed.Scheme == "https" {
		return strings.TrimRight(base, "/") + "/" + url.PathEscape(slug) + ".zip"
	}
	return "file://" + filepath.Join(base, slug+".zip")
}

func FillSlugTemplate(tmpl, slug string) string {
	tmpl = strings.TrimSpace(tmpl)
	if tmpl == "" || !strings.Contains(tmpl, "{slug}") {
		return tmpl
	}
	return strings.ReplaceAll(tmpl, "{slug}", url.PathEscape(slug))
}

func ReadJSONFromURI(uri string, timeout int) (interface{}, error) {
	parsed, err := url.Parse(uri)
	if err != nil {
		return nil, fmt.Errorf("invalid URI: %s", uri)
	}
	switch parsed.Scheme {
	case "", "file":
		path := uri
		if parsed.Scheme == "file" {
			path = parsed.Path
		}
		path = config.ExpandPath(path)
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("JSON source not found: %s", path)
		}
		var raw interface{}
		if err := json.Unmarshal(data, &raw); err != nil {
			return nil, fmt.Errorf("invalid JSON in %s: %w", path, err)
		}
		return raw, nil
	case "http", "https":
		client := &http.Client{Timeout: time.Duration(timeout) * time.Second}
		req, _ := http.NewRequest("GET", uri, nil)
		req.Header.Set("User-Agent", config.CLIUserAgent())
		req.Header.Set("Accept", "application/json")
		resp, err := client.Do(req)
		if err != nil {
			return nil, fmt.Errorf("failed to fetch JSON from %s: %w", uri, err)
		}
		defer resp.Body.Close()
		body, _ := io.ReadAll(resp.Body)
		if resp.StatusCode >= 400 {
			return nil, fmt.Errorf("failed to fetch JSON (%d) from %s", resp.StatusCode, uri)
		}
		var raw interface{}
		if err := json.Unmarshal(body, &raw); err != nil {
			return nil, fmt.Errorf("invalid JSON from %s: %w", uri, err)
		}
		return raw, nil
	default:
		return nil, fmt.Errorf("unsupported URI scheme: %s", uri)
	}
}
