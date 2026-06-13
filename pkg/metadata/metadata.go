package metadata

import (
	"archive/zip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/hitzhangjie/skillhub/pkg/config"
)

type FileEntry struct {
	RelPath string
	Content []byte
}

var (
	slugPattern     = regexp.MustCompile(`^[a-z0-9](?:[a-z0-9-]*[a-z0-9])?$`)
	semverPattern   = regexp.MustCompile(`^\d+\.\d+\.\d+(?:[-+][0-9A-Za-z.-]+)?$`)
	excludeDirs     = map[string]bool{".git": true, ".idea": true, ".vscode": true, "node_modules": true, "__pycache__": true}
	excludeSuffixes = []string{".pyc", ".DS_Store", "Thumbs.db"}
)

func ParseFrontmatter(skillMDPath string) (map[string]interface{}, error) {
	data, err := os.ReadFile(skillMDPath)
	if err != nil {
		return nil, fmt.Errorf("SKILL.md not found: %s", skillMDPath)
	}
	text := string(data)
	if !strings.HasPrefix(text, "---") {
		return nil, fmt.Errorf("SKILL.md missing front matter (first line must be ---)")
	}
	lines := strings.Split(text, "\n")
	if len(lines) == 0 || strings.TrimSpace(lines[0]) != "---" {
		return nil, fmt.Errorf("SKILL.md front matter start marker incorrect")
	}
	endIdx := -1
	for i := 1; i < len(lines); i++ {
		if strings.TrimSpace(lines[i]) == "---" {
			endIdx = i
			break
		}
	}
	if endIdx < 0 {
		return nil, fmt.Errorf("SKILL.md front matter missing closing ---")
	}
	body := lines[1:endIdx]
	out := make(map[string]interface{})
	for _, raw := range body {
		line := strings.TrimRight(raw, "\r")
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		if !strings.Contains(trimmed, ":") {
			continue
		}
		parts := strings.SplitN(trimmed, ":", 2)
		key := strings.TrimSpace(parts[0])
		val := strings.TrimSpace(parts[1])
		if key == "" {
			continue
		}
		// Simple list: [a, b, c]
		if strings.HasPrefix(val, "[") && strings.HasSuffix(val, "]") {
			inner := strings.TrimSpace(val[1 : len(val)-1])
			if inner == "" {
				out[key] = []string{}
			} else {
				var items []string
				for _, s := range strings.Split(inner, ",") {
					items = append(items, stripYamlScalar(strings.TrimSpace(s)))
				}
				out[key] = items
			}
		} else {
			out[key] = stripYamlScalar(val)
		}
	}
	return out, nil
}

func stripYamlScalar(val string) string {
	if len(val) >= 2 && val[0] == val[len(val)-1] && (val[0] == '\'' || val[0] == '"') {
		return val[1 : len(val)-1]
	}
	return val
}

func ValidateMetadata(md map[string]interface{}) error {
	slug := strVal(md, "slug")
	version := strVal(md, "version")
	displayName := strVal(md, "displayName")
	if slug == "" {
		return fmt.Errorf("SKILL.md missing slug")
	}
	if !slugPattern.MatchString(slug) {
		return fmt.Errorf("slug invalid (must be kebab-case 3-128 chars): %s", slug)
	}
	if len(slug) < 2 || len(slug) > 128 {
		return fmt.Errorf("slug length must be 2-128: %s", slug)
	}
	if version == "" {
		return fmt.Errorf("SKILL.md missing version")
	}
	if !semverPattern.MatchString(version) {
		return fmt.Errorf("version not valid SemVer: %s", version)
	}
	if displayName == "" {
		return fmt.Errorf("SKILL.md missing displayName")
	}
	return nil
}

func CollectSkillFiles(skillDir string) ([]FileEntry, error) {
	absDir, err := filepath.Abs(skillDir)
	if err != nil {
		return nil, err
	}
	var entries []FileEntry
	hasSkillMD := false

	err = filepath.Walk(absDir, func(absPath string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(absDir, absPath)
		rel = filepath.ToSlash(rel)

		// Skip excluded dirs
		if info.IsDir() {
			if excludeDirs[info.Name()] {
				return filepath.SkipDir
			}
			return nil
		}
		// Skip excluded files
		for _, suf := range excludeSuffixes {
			if strings.HasSuffix(info.Name(), suf) {
				return nil
			}
		}
		// Skip symlinks
		if info.Mode()&os.ModeSymlink != 0 {
			if _, err := os.Stat(absPath); err != nil {
				fmt.Fprintf(os.Stderr, "warn: skipping broken symlink %s\n", rel)
			} else {
				fmt.Fprintf(os.Stderr, "warn: skipping symlink %s\n", rel)
			}
			return nil
		}

		data, err := os.ReadFile(absPath)
		if err != nil {
			return fmt.Errorf("failed to read file %s: %w", absPath, err)
		}
		entries = append(entries, FileEntry{RelPath: rel, Content: data})
		if strings.ToLower(rel) == "skill.md" {
			hasSkillMD = true
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	if !hasSkillMD {
		return nil, fmt.Errorf("SKILL.md not found (must be in root)")
	}
	if len(entries) == 0 {
		return nil, fmt.Errorf("skill directory is empty")
	}
	return entries, nil
}

func PackZip(skillDir, dest string) error {
	absDir, err := filepath.Abs(skillDir)
	if err != nil {
		return err
	}
	f, err := os.Create(dest)
	if err != nil {
		return err
	}
	defer f.Close()

	w := zip.NewWriter(f)
	defer w.Close()

	err = filepath.Walk(absDir, func(absPath string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			if excludeDirs[info.Name()] {
				return filepath.SkipDir
			}
			return nil
		}
		for _, suf := range excludeSuffixes {
			if strings.HasSuffix(info.Name(), suf) {
				return nil
			}
		}
		rel, _ := filepath.Rel(absDir, absPath)
		rel = filepath.ToSlash(rel)

		zi, err := zip.FileInfoHeader(info)
		if err != nil {
			return err
		}
		zi.Name = rel
		zi.Method = zip.Deflate
		zi.SetMode(0o644)

		writer, err := w.CreateHeader(zi)
		if err != nil {
			return err
		}
		data, err := os.ReadFile(absPath)
		if err != nil {
			return fmt.Errorf("failed to read file %s: %w", absPath, err)
		}
		_, err = writer.Write(data)
		return err
	})
	return err
}

func ResolveSkillInput(pathArg string) (skillDir string, cleanupDir string, err error) {
	abs, err := filepath.Abs(config.ExpandPath(pathArg))
	if err != nil {
		return "", "", err
	}
	info, err := os.Stat(abs)
	if err != nil {
		return "", "", fmt.Errorf("path does not exist: %s", abs)
	}
	if info.IsDir() {
		return abs, "", nil
	}
	if info.Mode().IsRegular() && strings.ToLower(filepath.Ext(abs)) == ".zip" {
		tmpRoot, err := os.MkdirTemp("", "skillhub-publish-")
		if err != nil {
			return "", "", err
		}
		if err := safeExtractZipInline(abs, tmpRoot); err != nil {
			os.RemoveAll(tmpRoot)
			return "", "", fmt.Errorf("zip extraction failed: %w", err)
		}
		// Smart unwrap: if only one visible dir, use that
		entries, _ := os.ReadDir(tmpRoot)
		var visible []os.DirEntry
		for _, e := range entries {
			if strings.HasPrefix(e.Name(), ".") && e.Name() != "." {
				continue
			}
			if e.Name() == "__MACOSX" {
				continue
			}
			visible = append(visible, e)
		}
		if len(visible) == 1 && visible[0].IsDir() {
			return filepath.Join(tmpRoot, visible[0].Name()), tmpRoot, nil
		}
		return tmpRoot, tmpRoot, nil
	}
	return "", "", fmt.Errorf("path must be a skill directory or .zip file: %s", abs)
}

func safeExtractZipInline(zipPath, targetDir string) error {
	r, err := zip.OpenReader(zipPath)
	if err != nil {
		return fmt.Errorf("not a valid zip archive: %w", err)
	}
	defer r.Close()
	var totalSize int64
	const maxSize = 50 * 1024 * 1024
	for _, f := range r.File {
		totalSize += int64(f.UncompressedSize64)
		if totalSize > maxSize {
			return fmt.Errorf("zip uncompressed size exceeds %dMB, possible zip bomb", maxSize/(1024*1024))
		}
		parts := strings.Split(f.Name, "/")
		for _, p := range parts {
			if p == ".." {
				return fmt.Errorf("unsafe zip path entry: %s", f.Name)
			}
		}
		if strings.HasPrefix(f.Name, "/") || strings.HasPrefix(f.Name, "\\") {
			return fmt.Errorf("zip contains illegal absolute path: %s", f.Name)
		}
		dest := filepath.Join(targetDir, filepath.FromSlash(f.Name))
		if f.FileInfo().IsDir() {
			os.MkdirAll(dest, 0755)
			continue
		}
		os.MkdirAll(filepath.Dir(dest), 0755)
		rc, err := f.Open()
		if err != nil {
			return err
		}
		out, err := os.Create(dest)
		if err != nil {
			rc.Close()
			return err
		}
		_, err = io.Copy(out, rc)
		rc.Close()
		out.Close()
		if err != nil {
			return err
		}
	}
	return nil
}

func strVal(m map[string]interface{}, key string) string {
	if v, ok := m[key].(string); ok {
		return strings.TrimSpace(v)
	}
	return ""
}

func PublishPayload(md map[string]interface{}, changelog string) map[string]interface{} {
	tags, _ := md["tags"].([]string)
	if tags == nil {
		if raw, ok := md["tags"].([]interface{}); ok {
			for _, t := range raw {
				if s, ok := t.(string); ok {
					tags = append(tags, s)
				}
			}
		}
	}
	if tags == nil {
		tags = []string{}
	}
	return map[string]interface{}{
		"slug":        strVal(md, "slug"),
		"version":     strVal(md, "version"),
		"displayName": strVal(md, "displayName"),
		"summary":     strVal(md, "summary"),
		"description": strVal(md, "description"),
		"tags":        tags,
		"license":     strVal(md, "license"),
		"homepage":    strVal(md, "homepage"),
		"changelog":   changelog,
	}
}
