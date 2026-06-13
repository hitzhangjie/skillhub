package cmd

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	"github.com/hitzhangjie/skillhub/pkg/config"
	"github.com/hitzhangjie/skillhub/pkg/lockfile"
	"github.com/spf13/cobra"
)

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List locally installed skills",
	Long:  "List all skills installed in the local install root directory.",
	RunE:  runList,
}

func init() {
	rootCmd.AddCommand(listCmd)
}

func runList(cmd *cobra.Command, args []string) error {
	root := config.ExpandPath(installRoot)
	abs, err := filepath.Abs(root)
	if err != nil {
		return err
	}

	lock := lockfile.Load(abs)
	skills := lock.Skills
	if len(skills) == 0 {
		fmt.Println("No installed skills.")
		return nil
	}

	slugs := make([]string, 0, len(skills))
	for slug := range skills {
		slugs = append(slugs, slug)
	}
	sort.Strings(slugs)

	for _, slug := range slugs {
		meta := skills[slug]
		version := strings.TrimSpace(meta.Version)
		fmt.Printf("%s  %s\n", slug, version)
	}
	return nil
}
