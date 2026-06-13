package credentials

import (
	"encoding/json"
	"os"
	"sort"
	"time"

	"github.com/hitzhangjie/skillhub/pkg/config"
)

type OrgInfo struct {
	Key        string `json:"-"`
	OrgID      int    `json:"orgId"`
	OrgOrgID   string `json:"orgOrgId"`
	OrgSlug    string `json:"orgSlug"`
	OrgName    string `json:"orgName"`
	Host       string `json:"host"`
	APIKey     string `json:"apiKey"`
	LoggedInAt string `json:"loggedInAt"`
}

type UserInfo struct {
	Host       string `json:"host"`
	Token      string `json:"token"`
	UserID     int    `json:"userId"`
	Handle     string `json:"handle"`
	LoggedInAt string `json:"loggedInAt"`
}

type Credentials struct {
	Version int                `json:"version"`
	Orgs    map[string]OrgInfo `json:"orgs"`
	User    *UserInfo          `json:"user,omitempty"`
}

func Load() *Credentials {
	c := &Credentials{Version: 1, Orgs: make(map[string]OrgInfo)}
	path := config.GetCredentialsPath()
	data, err := os.ReadFile(path)
	if err != nil {
		return c
	}
	var raw map[string]interface{}
	if err := json.Unmarshal(data, &raw); err != nil {
		return c
	}
	loadOrgs(raw, c)
	loadUser(raw, c)
	return c
}

func loadOrgs(raw map[string]interface{}, c *Credentials) {
	orgs, ok := raw["orgs"].(map[string]interface{})
	if !ok {
		return
	}
	for key, val := range orgs {
		m, ok := val.(map[string]interface{})
		if !ok {
			continue
		}
		oi := OrgInfo{Key: key}
		if v, ok := m["orgId"].(float64); ok {
			oi.OrgID = int(v)
		}
		if v, ok := m["orgOrgId"].(string); ok {
			oi.OrgOrgID = v
		}
		if v, ok := m["orgSlug"].(string); ok {
			oi.OrgSlug = v
		}
		if v, ok := m["orgName"].(string); ok {
			oi.OrgName = v
		}
		if v, ok := m["host"].(string); ok {
			oi.Host = v
		}
		if v, ok := m["apiKey"].(string); ok {
			oi.APIKey = v
		}
		if v, ok := m["loggedInAt"].(string); ok {
			oi.LoggedInAt = v
		}
		c.Orgs[key] = oi
	}
}

func loadUser(raw map[string]interface{}, c *Credentials) {
	u, ok := raw["user"].(map[string]interface{})
	if !ok {
		return
	}
	ui := &UserInfo{}
	if v, ok := u["host"].(string); ok {
		ui.Host = v
	}
	if v, ok := u["token"].(string); ok {
		ui.Token = v
	}
	if v, ok := u["userId"].(float64); ok {
		ui.UserID = int(v)
	}
	if v, ok := u["handle"].(string); ok {
		ui.Handle = v
	}
	if v, ok := u["loggedInAt"].(string); ok {
		ui.LoggedInAt = v
	}
	if ui.Token == "" {
		return
	}
	c.User = ui
}

func Save(c *Credentials) error {
	path := config.GetCredentialsPath()
	if err := os.MkdirAll(path[:len(path)-len(config.CredentialsFileName)-1], 0700); err == nil {
		// mkdir parent
		_ = os.MkdirAll(path[:len(path)-len(config.CredentialsFileName)-1], 0700)
	}
	tmp := path + ".tmp"
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(tmp, data, 0600); err != nil {
		// fallback direct write
		_ = os.WriteFile(path, data, 0600)
		return nil
	}
	return os.Rename(tmp, path)
}

func GetOrg(orgSlug string) *OrgInfo {
	c := Load()
	// Exact key match
	if oi, ok := c.Orgs[orgSlug]; ok {
		return &oi
	}
	// Fallback: match by OrgOrgID or OrgSlug field
	for _, oi := range c.Orgs {
		if oi.OrgOrgID == orgSlug || oi.OrgSlug == orgSlug {
			return &oi
		}
	}
	return nil
}

func GetAllOrgs() []OrgInfo {
	c := Load()
	orgs := make([]OrgInfo, 0, len(c.Orgs))
	for _, oi := range c.Orgs {
		orgs = append(orgs, oi)
	}
	sort.Slice(orgs, func(i, j int) bool {
		return orgs[i].LoggedInAt > orgs[j].LoggedInAt
	})
	return orgs
}

func GetUser() *UserInfo {
	c := Load()
	return c.User
}

func SaveUser(host, token string, userID int, handle string) error {
	c := Load()
	c.User = &UserInfo{
		Host:       host,
		Token:      token,
		UserID:     userID,
		Handle:     handle,
		LoggedInAt: time.Now().UTC().Format("2006-01-02T15:04:05Z"),
	}
	return Save(c)
}

func ClearUser() error {
	c := Load()
	c.User = nil
	return Save(c)
}

func SaveOrg(oi OrgInfo) error {
	c := Load()
	oi.LoggedInAt = time.Now().UTC().Format("2006-01-02T15:04:05Z")
	c.Orgs[oi.OrgOrgID] = oi
	return Save(c)
}

func DeleteOrg(orgSlug string) error {
	c := Load()
	// Exact key match
	if _, ok := c.Orgs[orgSlug]; ok {
		delete(c.Orgs, orgSlug)
		return Save(c)
	}
	// Fallback by field
	for key, oi := range c.Orgs {
		if oi.OrgOrgID == orgSlug || oi.OrgSlug == orgSlug {
			delete(c.Orgs, key)
			return Save(c)
		}
	}
	return nil
}

func ResolveFromEnv() *OrgInfo {
	orgSlug := os.Getenv(config.EnvOrg)
	apiKey := os.Getenv(config.EnvAPIKey)
	if orgSlug == "" || apiKey == "" {
		return nil
	}
	host := os.Getenv(config.EnvHost)
	if host == "" {
		host = config.DefaultEnterpriseHost
	}
	return &OrgInfo{
		OrgOrgID: orgSlug,
		OrgSlug:  orgSlug,
		Host:     host,
		APIKey:   apiKey,
	}
}

func MaskAPIKey(key string) string {
	if len(key) <= 15 {
		if len(key) > 8 {
			return key[:4] + "..." + key[len(key)-4:]
		}
		return "***"
	}
	return key[:11] + "..." + key[len(key)-4:]
}
