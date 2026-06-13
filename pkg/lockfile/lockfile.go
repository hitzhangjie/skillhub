package lockfile

import (
	"encoding/json"
	"os"
	"path/filepath"

	"github.com/hitzhangjie/skillhub/pkg/config"
)

type SkillMeta struct {
	Name        string `json:"name,omitempty"`
	ZipURL      string `json:"zip_url,omitempty"`
	Source      string `json:"source,omitempty"`
	Org         string `json:"org,omitempty"`
	Host        string `json:"host,omitempty"`
	Version     string `json:"version,omitempty"`
	UpdateURL   string `json:"update_url,omitempty"`
	InstalledAt string `json:"installedAt,omitempty"`
}

type Lockfile struct {
	Version int                  `json:"version"`
	Skills  map[string]SkillMeta `json:"skills"`
}

func Load(installRoot string) *Lockfile {
	lf := &Lockfile{Version: 1, Skills: make(map[string]SkillMeta)}
	path := filepath.Join(installRoot, config.LockfileName)
	data, err := os.ReadFile(path)
	if err != nil {
		return lf
	}
	var raw map[string]interface{}
	if err := json.Unmarshal(data, &raw); err != nil {
		return lf
	}
	if v, ok := raw["version"].(float64); ok {
		lf.Version = int(v)
	}
	if skills, ok := raw["skills"].(map[string]interface{}); ok {
		for slug, val := range skills {
			if m, ok := val.(map[string]interface{}); ok {
				sm := SkillMeta{}
				if v, ok := m["name"].(string); ok {
					sm.Name = v
				}
				if v, ok := m["zip_url"].(string); ok {
					sm.ZipURL = v
				}
				if v, ok := m["source"].(string); ok {
					sm.Source = v
				}
				if v, ok := m["org"].(string); ok {
					sm.Org = v
				}
				if v, ok := m["host"].(string); ok {
					sm.Host = v
				}
				if v, ok := m["version"].(string); ok {
					sm.Version = v
				}
				if v, ok := m["update_url"].(string); ok {
					sm.UpdateURL = v
				}
				if v, ok := m["installedAt"].(string); ok {
					sm.InstalledAt = v
				}
				lf.Skills[slug] = sm
			}
		}
	}
	return lf
}

func Save(installRoot string, lf *Lockfile) error {
	if err := os.MkdirAll(installRoot, 0755); err != nil {
		return err
	}
	path := filepath.Join(installRoot, config.LockfileName)
	data, err := json.MarshalIndent(lf, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}
