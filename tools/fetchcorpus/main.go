// Command fetchcorpus downloads the official Ikemen GO release that the
// corpus tests read (Kung Fu Man and the data/ folder) into ikemen/release.
//
//	cd tools && go run ./fetchcorpus
//
// The archive is pinned by URL and SHA-256 to the engine version in
// ikemen/AGENTS.md. Nothing is downloaded when the release is already there.
// The Windows archive is used on every OS: the tests only read its data
// files, never run the engine.
package main

import (
	"archive/zip"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const (
	name    = "Ikemen_GO-v1.0.0-windows"
	url     = "https://github.com/ikemen-engine/Ikemen-GO/releases/download/v1.0.0/" + name + ".zip"
	wantSHA = "9338eaeb68599ceb13b0867819a091ea3f92eba58077e800b4540b7a1f9c3731"
)

func main() {
	root := flag.String("root", "", "repository root (default: the folder holding go.work, searched upwards)")
	flag.Parse()
	if err := run(*root); err != nil {
		fmt.Fprintln(os.Stderr, "fetchcorpus:", err)
		os.Exit(1)
	}
}

func run(root string) error {
	if root == "" {
		var err error
		if root, err = findRoot(); err != nil {
			return err
		}
	}
	dest := filepath.Join(root, "ikemen", "release", name)
	if _, err := os.Stat(filepath.Join(dest, "chars", "kfm", "kfm.def")); err == nil {
		fmt.Println("corpus already present:", dest)
		return nil
	}

	tmp, err := os.CreateTemp("", "ikemen-*.zip")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	defer tmp.Close()

	fmt.Println("downloading", url)
	client := &http.Client{Timeout: 10 * time.Minute}
	resp, err := client.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download: %s", resp.Status)
	}
	h := sha256.New()
	n, err := io.Copy(io.MultiWriter(tmp, h), resp.Body)
	if err != nil {
		return err
	}
	if got := hex.EncodeToString(h.Sum(nil)); got != wantSHA {
		return fmt.Errorf("checksum mismatch: got %s, want %s", got, wantSHA)
	}
	fmt.Printf("verified %d bytes\n", n)

	if err := extract(tmp.Name(), dest); err != nil {
		os.RemoveAll(dest)
		return err
	}
	fmt.Println("extracted to", dest)
	return nil
}

// findRoot walks up from the working directory to the folder with go.work.
func findRoot() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.work")); err == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", errors.New("go.work not found; run from inside the repository or pass -root")
		}
		dir = parent
	}
}

func extract(zipPath, dest string) error {
	z, err := zip.OpenReader(zipPath)
	if err != nil {
		return err
	}
	defer z.Close()
	for _, f := range z.File {
		p := filepath.Join(dest, filepath.FromSlash(f.Name))
		// Refuse entries that would land outside dest.
		if rel, err := filepath.Rel(dest, p); err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			return fmt.Errorf("unsafe path in archive: %s", f.Name)
		}
		if f.FileInfo().IsDir() {
			if err := os.MkdirAll(p, 0o755); err != nil {
				return err
			}
			continue
		}
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			return err
		}
		if err := writeFile(f, p); err != nil {
			return err
		}
	}
	return nil
}

func writeFile(f *zip.File, p string) error {
	r, err := f.Open()
	if err != nil {
		return err
	}
	defer r.Close()
	w, err := os.Create(p)
	if err != nil {
		return err
	}
	if _, err := io.Copy(w, r); err != nil {
		w.Close()
		return err
	}
	return w.Close()
}
