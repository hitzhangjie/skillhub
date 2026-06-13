package install

import (
	"archive/tar"
	"archive/zip"
	"crypto/sha256"
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

func SHA256File(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	buf := make([]byte, 1024*1024)
	for {
		n, err := f.Read(buf)
		if n > 0 {
			h.Write(buf[:n])
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			return "", err
		}
	}
	return fmt.Sprintf("%x", h.Sum(nil)), nil
}

func SafeExtractZip(zipPath, targetDir string) error {
	r, err := zip.OpenReader(zipPath)
	if err != nil {
		return fmt.Errorf("not a valid zip archive: %w", err)
	}
	defer r.Close()
	for _, f := range r.File {
		p := filepath.Clean(f.Name)
		if filepath.IsAbs(p) || strings.HasPrefix(p, "..") {
			return fmt.Errorf("unsafe zip path entry: %s", f.Name)
		}
		dest := filepath.Join(targetDir, filepath.FromSlash(p))
		if f.FileInfo().IsDir() {
			os.MkdirAll(dest, 0755)
			continue
		}
		os.MkdirAll(filepath.Dir(dest), 0755)
		rc, err := f.Open()
		if err != nil {
			return fmt.Errorf("failed to open zip entry %s: %w", f.Name, err)
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
			return fmt.Errorf("failed to extract %s: %w", f.Name, err)
		}
	}
	return nil
}

func SafeExtractTar(tarPath, targetDir string) error {
	f, err := os.Open(tarPath)
	if err != nil {
		return err
	}
	defer f.Close()
	tr := tar.NewReader(f)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
		p := filepath.Clean(hdr.Name)
		if filepath.IsAbs(p) || strings.HasPrefix(p, "..") {
			return fmt.Errorf("unsafe tar path entry: %s", hdr.Name)
		}
		dest := filepath.Join(targetDir, filepath.FromSlash(p))
		switch hdr.Typeflag {
		case tar.TypeDir:
			os.MkdirAll(dest, 0755)
		case tar.TypeReg:
			os.MkdirAll(filepath.Dir(dest), 0755)
			out, err := os.Create(dest)
			if err != nil {
				return err
			}
			_, err = io.Copy(out, tr)
			out.Close()
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func DownloadFile(urlStr, dest string) error {
	return download(urlStr, dest, 30)
}

func download(urlStr, dest string, timeoutSec int) error {
	parsed, err := url.Parse(urlStr)
	if err != nil {
		return fmt.Errorf("invalid URL: %s", urlStr)
	}
	if parsed.Scheme == "file" || parsed.Scheme == "" {
		src := urlStr
		if parsed.Scheme == "file" {
			src = parsed.Path
		}
		src = config.ExpandPath(src)
		if _, err := os.Stat(src); err != nil {
			return fmt.Errorf("local file not found: %s", src)
		}
		return copyFile(src, dest)
	}
	client := &http.Client{Timeout: time.Duration(timeoutSec) * time.Second}
	req, _ := http.NewRequest("GET", urlStr, nil)
	req.Header.Set("User-Agent", config.CLIUserAgent())
	req.Header.Set("Accept", "application/zip,application/octet-stream,*/*")
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("download failed for %s: %w", urlStr, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		detail := fmt.Sprintf("HTTP %d", resp.StatusCode)
		if resp.StatusCode == 429 {
			detail += " (rate limited)"
		}
		return fmt.Errorf("download failed: %s for %s", detail, urlStr)
	}
	out, err := os.Create(dest)
	if err != nil {
		return err
	}
	defer out.Close()
	_, err = io.Copy(out, resp.Body)
	return err
}

func copyFile(src, dst string) error {
	s, err := os.Open(src)
	if err != nil {
		return err
	}
	defer s.Close()
	d, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer d.Close()
	_, err = io.Copy(d, s)
	return err
}

func InstallZipToTarget(slug, zipURI, targetDir string, force bool, expectedSHA256 string) error {
	tmpDir, err := os.MkdirTemp("", "skillhub-install-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmpDir)

	zipPath := filepath.Join(tmpDir, slug+".zip")
	fmt.Fprintf(os.Stderr, "Downloading: %s\n", zipURI)
	if err := DownloadFile(zipURI, zipPath); err != nil {
		return err
	}

	if expectedSHA256 != "" {
		actual, err := SHA256File(zipPath)
		if err != nil {
			return err
		}
		if strings.ToLower(actual) != strings.ToLower(expectedSHA256) {
			return fmt.Errorf("SHA256 mismatch for %s: expected %s, got %s", slug, expectedSHA256, actual)
		}
	}

	stageDir := filepath.Join(tmpDir, "stage")
	if err := os.MkdirAll(stageDir, 0755); err != nil {
		return err
	}
	if err := SafeExtractZip(zipPath, stageDir); err != nil {
		return err
	}

	return moveToTarget(stageDir, targetDir, force)
}

func InstallZipToTargetWithFallback(slug string, zipURIs []string, targetDir string, force bool, expectedSHA256 string, quiet bool) error {
	if len(zipURIs) == 0 {
		return fmt.Errorf("no download URL candidates for %q", slug)
	}
	if !force {
		if _, err := os.Stat(targetDir); err == nil {
			return fmt.Errorf("target exists: %s (use --force to overwrite)", targetDir)
		}
	}

	tmpDir, err := os.MkdirTemp("", "skillhub-install-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmpDir)

	zipPath := filepath.Join(tmpDir, slug+".zip")
	stageDir := filepath.Join(tmpDir, "stage")
	os.MkdirAll(stageDir, 0755)

	var lastErr string
	usedURI := ""
	for i, uri := range zipURIs {
		uri = strings.TrimSpace(uri)
		if uri == "" {
			continue
		}
		if !quiet {
			fmt.Fprintf(os.Stderr, "Downloading: %s\n", uri)
		}
		if err := download(uri, zipPath, 30); err != nil {
			lastErr = err.Error()
			if i+1 < len(zipURIs) && !quiet {
				fmt.Fprintf(os.Stderr, "Download failed, fallback next source: %s\n", err)
			}
			continue
		}
		usedURI = uri
		lastErr = ""
		break
	}
	if lastErr != "" {
		return fmt.Errorf("%s", lastErr)
	}
	_ = usedURI

	if expectedSHA256 != "" {
		actual, err := SHA256File(zipPath)
		if err != nil {
			return err
		}
		if strings.ToLower(actual) != strings.ToLower(expectedSHA256) {
			return fmt.Errorf("SHA256 mismatch for %s: expected %s, got %s", slug, expectedSHA256, actual)
		}
	}
	if err := SafeExtractZip(zipPath, stageDir); err != nil {
		return err
	}
	return moveToTarget(stageDir, targetDir, force)
}

func moveToTarget(stageDir, targetDir string, force bool) error {
	if force {
		if _, err := os.Stat(targetDir); err == nil {
			if err := os.RemoveAll(targetDir); err != nil {
				return err
			}
		}
	}
	if err := os.MkdirAll(filepath.Dir(targetDir), 0755); err != nil {
		return err
	}
	return os.Rename(stageDir, targetDir)
}

func FindCLIScriptInExtracted(root string) string {
	for _, name := range []string{"skills_store_cli.py"} {
		p := filepath.Join(root, name)
		if _, err := os.Stat(p); err == nil {
			return p
		}
		p = filepath.Join(root, "cli", name)
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	matches, _ := filepath.Glob(filepath.Join(root, "**", "skills_store_cli.py"))
	if len(matches) > 0 {
		return matches[0]
	}
	return ""
}

func FindPeerFileInExtracted(root, filename string) string {
	for _, name := range []string{filename} {
		p := filepath.Join(root, name)
		if _, err := os.Stat(p); err == nil {
			return p
		}
		p = filepath.Join(root, "cli", name)
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	matches, _ := filepath.Glob(filepath.Join(root, "**", filename))
	if len(matches) > 0 {
		return matches[0]
	}
	return ""
}

func FindSkillFileInExtracted(root, filename string) string {
	for _, sub := range []string{"skill", filepath.Join("cli", "skill")} {
		p := filepath.Join(root, sub, filename)
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	matches, _ := filepath.Glob(filepath.Join(root, "**", filename))
	for _, m := range matches {
		if filepath.Base(filepath.Dir(m)) == "skill" {
			return m
		}
	}
	return ""
}
