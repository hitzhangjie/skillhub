package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/hitzhangjie/skillhub/pkg/community"
	"github.com/hitzhangjie/skillhub/pkg/config"
	"github.com/hitzhangjie/skillhub/pkg/credentials"
	"github.com/hitzhangjie/skillhub/pkg/enterprise"
	"github.com/hitzhangjie/skillhub/pkg/index"
	"github.com/spf13/cobra"
)

var (
	searchURL     string
	searchLimit   int
	searchTimeout int
	searchJSON    bool
	searchOrg     string
)

var searchCmd = &cobra.Command{
	Use:   "search [query...]",
	Short: "Search skills by keyword",
	Long: `Search for skills across community and enterprise registries.

Without arguments, lists all skills from the local index.
With a query, searches community and enterprise registries and merges results.`,
	Example: `  skillhub search
  skillhub search code review
  skillhub search --json "code review"
  skillhub search --org my-team deploy`,
	RunE: runSearch,
}

func init() {
	searchCmd.Flags().StringVar(&searchURL, "search-url", config.Metadata.SkillsSearchURL,
		"Remote search API URL")
	searchCmd.Flags().IntVar(&searchLimit, "search-limit", 20, "Remote search limit")
	searchCmd.Flags().IntVar(&searchTimeout, "search-timeout", 6, "Remote search timeout in seconds")
	searchCmd.Flags().BoolVar(&searchJSON, "json", false, "Print search results as JSON")
	searchCmd.Flags().StringVar(&searchOrg, "org", "", "Only search specified enterprise source (org slug)")
	rootCmd.AddCommand(searchCmd)
}

func runSearch(cmd *cobra.Command, args []string) error {
	query := strings.TrimSpace(strings.Join(args, " "))
	scopeOrg := strings.TrimSpace(searchOrg)

	if query == "" && scopeOrg == "" {
		return searchLocalIndex(query)
	}
	return searchRemote(query, scopeOrg)
}

func searchLocalIndex(query string) error {
	idx, err := index.Load(indexURI, 20)
	if err != nil {
		return fmt.Errorf("failed to load index: %w", err)
	}

	matches := idx.Skills
	if query != "" {
		query = strings.ToLower(query)
		var filtered []index.SkillDescriptor
		for _, s := range matches {
			if strings.Contains(index.SearchText(&s), query) {
				filtered = append(filtered, s)
			}
		}
		matches = filtered
	}

	if len(matches) == 0 {
		fmt.Println("No skills found.")
		return nil
	}

	if searchJSON {
		return printSearchJSON(matches, query, nil)
	}

	fmt.Println(`You can use "skillhub install [skill]" to install.`)
	for _, s := range matches {
		slug := s.Slug
		name := s.Name
		if name == "" {
			name = s.DisplayName
		}
		if name == "" {
			name = slug
		}
		desc := s.Description
		if desc == "" {
			desc = s.Summary
		}
		fmt.Printf("  %s  %s\n", slug, name)
		if desc != "" {
			fmt.Printf("    - %s\n", desc)
		}
		if s.Version != "" {
			fmt.Printf("    - version: %s\n", s.Version)
		}
		if s.ZipURL != "" {
			fmt.Printf("    - %s\n", s.ZipURL)
		}
		if s.Homepage != "" {
			fmt.Printf("    - %s\n", s.Homepage)
		}
	}
	return nil
}

func searchRemote(query, scopeOrg string) error {
	var warnings []string
	var enterpriseResults []index.SkillDescriptor
	var communityResults []index.SkillDescriptor

	// Search enterprise sources
	if scopeOrg == "" || scopeOrg != "community" {
		orgs := credentials.GetAllOrgs()
		if scopeOrg != "" {
			orgs = filterOrgs(orgs, scopeOrg)
			if len(orgs) == 0 {
				warnings = append(warnings, fmt.Sprintf("@%s: not logged in, skipped", scopeOrg))
			}
		}
		for _, cred := range orgs {
			host := cred.Host
			if host == "" {
				host = config.DefaultEnterpriseHost
			}
			if cred.OrgID == 0 || cred.APIKey == "" {
				continue
			}
			results, ok := enterprise.Search(host, cred.OrgID, cred.APIKey, query, searchLimit)
			if !ok {
				warnings = append(warnings, fmt.Sprintf("@%s: request timed out or failed, results omitted", cred.OrgSlug))
			} else {
				for _, r := range results {
					r.Source = fmt.Sprintf("@%s", cred.OrgSlug)
					r.Org = cred.OrgSlug
					enterpriseResults = append(enterpriseResults, r)
				}
			}
		}
	}

	// Search community
	if scopeOrg == "" || scopeOrg == "community" {
		comm, ok := community.Search(searchURL, query, searchLimit, searchTimeout)
		if ok {
			for _, r := range comm {
				r.Source = "community"
				communityResults = append(communityResults, r)
			}
		} else if scopeOrg == "" {
			warnings = append(warnings, "community: search request failed")
		}
	}

	allResults := append(enterpriseResults, communityResults...)

	for _, w := range warnings {
		fmt.Fprintf(os.Stderr, "⚠️  %s\n", w)
	}

	if len(allResults) == 0 {
		fmt.Println("No skills found.")
		return nil
	}

	if searchJSON {
		return printSearchJSON(allResults, query, warnings)
	}

	fmt.Println(`You can use "skillhub install [skill]" to install.`)
	for _, s := range allResults {
		displaySlug := s.Slug
		if s.Source != "" && !strings.HasPrefix(s.Source, "com") {
			displaySlug = fmt.Sprintf("@%s/%s", s.Org, s.Slug)
		}
		name := s.Name
		if name == "" {
			name = s.DisplayName
		}
		if name == "" {
			name = s.Slug
		}
		fmt.Printf("  %s  %s\n", displaySlug, name)
		if s.Description != "" {
			fmt.Printf("    - %s\n", s.Description)
		}
		if s.Version != "" {
			fmt.Printf("    - version: %s\n", s.Version)
		}
	}
	return nil
}

func printSearchJSON(results []index.SkillDescriptor, query string, warnings []string) error {
	type resultJSON struct {
		Slug        string `json:"slug"`
		Name        string `json:"name"`
		Description string `json:"description"`
		Version     string `json:"version"`
		Source      string `json:"source"`
	}
	var out []resultJSON
	for _, s := range results {
		displaySlug := s.Slug
		if s.Source != "" && !strings.HasPrefix(s.Source, "com") {
			displaySlug = fmt.Sprintf("@%s/%s", s.Org, s.Slug)
		}
		name := s.Name
		if name == "" {
			name = s.DisplayName
		}
		if name == "" {
			name = s.Slug
		}
		desc := s.Description
		if desc == "" {
			desc = s.Summary
		}
		out = append(out, resultJSON{
			Slug:        displaySlug,
			Name:        name,
			Description: desc,
			Version:     s.Version,
			Source:      s.Source,
		})
	}
	data, _ := json.MarshalIndent(map[string]interface{}{
		"query":    query,
		"count":    len(out),
		"results":  out,
		"warnings": warnings,
	}, "", "  ")
	fmt.Println(string(data))
	return nil
}

func filterOrgs(orgs []credentials.OrgInfo, scope string) []credentials.OrgInfo {
	var filtered []credentials.OrgInfo
	for _, o := range orgs {
		if o.OrgOrgID == scope || o.OrgSlug == scope {
			filtered = append(filtered, o)
		}
	}
	return filtered
}
