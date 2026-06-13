package community

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/hitzhangjie/skillhub/pkg/config"
	"github.com/hitzhangjie/skillhub/pkg/index"
)

func Search(searchURL, query string, limit, timeout int) ([]index.SkillDescriptor, bool) {
	base := strings.TrimSpace(searchURL)
	q := strings.TrimSpace(query)
	if base == "" || q == "" {
		return nil, false
	}
	parsed, err := url.Parse(base)
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		return nil, false
	}

	params := url.Values{}
	params.Set("q", q)
	params.Set("limit", fmt.Sprintf("%d", max(1, limit)))
	parsed.RawQuery = params.Encode()
	fullURL := parsed.String()

	client := &http.Client{Timeout: time.Duration(max(1, timeout)) * time.Second}
	req, _ := http.NewRequest("GET", fullURL, nil)
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
	results, ok := raw["results"].([]interface{})
	if !ok {
		return nil, false
	}

	var out []index.SkillDescriptor
	for _, item := range results {
		m, ok := item.(map[string]interface{})
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
			Source:      "community",
		})
	}
	return out, true
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
