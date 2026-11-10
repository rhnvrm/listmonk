// Build the admin frontend from static/admin with the pinned esgun tool, then
// prepare the compressed assets expected by the static file server.
package main

import (
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"flag"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"
)

func main() {
	watch := flag.Bool("watch", false, "rebuild when assets or esgun.config.json change")
	flag.Parse()
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if err := run(ctx, *watch); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(ctx context.Context, watch bool) error {
	previous, err := snapshot()
	if err != nil {
		return err
	}
	if err := build(ctx); err != nil {
		return err
	}
	if !watch {
		return nil
	}
	fmt.Println("watching assets/ and esgun.config.json for changes…")
	// Polling also detects edits on bind mounts that do not deliver file events.
	ticker := time.NewTicker(300 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			current, err := snapshot()
			if err != nil {
				fmt.Fprintln(os.Stderr, err)
				continue
			}
			if current == previous {
				continue
			}
			// Record before building so edits during a build trigger another run.
			previous = current
			if err := build(ctx); err != nil {
				fmt.Fprintln(os.Stderr, err)
			}
		}
	}
}

func snapshot() ([sha256.Size]byte, error) {
	h := sha256.New()
	for _, root := range []string{"assets", "esgun.config.json"} {
		if err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if entry.IsDir() {
				return nil
			}
			info, err := entry.Info()
			if err != nil {
				return err
			}
			fmt.Fprintf(h, "%s\x00%d\x00%d\n", path, info.Size(), info.ModTime().UnixNano())
			return nil
		}); err != nil {
			return [sha256.Size]byte{}, err
		}
	}
	return [sha256.Size]byte(h.Sum(nil)), nil
}

func build(ctx context.Context) error {
	cmd := exec.CommandContext(ctx, "go", "tool", "esgun", "build")
	cmd.Stdout, cmd.Stderr = os.Stdout, os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("esgun build: %w", err)
	}
	return assemble(".build", "dist", "../public/static/altcha.umd.js")
}

func assemble(stage, dist, altcha string) error {
	// ALTCHA is served by public subscription forms, without admin compression.
	publicAsset := filepath.Join(stage, "altcha.umd.js")
	data, err := os.ReadFile(publicAsset)
	if err != nil {
		return err
	}
	if err := os.WriteFile(altcha, data, 0o644); err != nil {
		return err
	}
	if err := os.Remove(publicAsset); err != nil {
		return err
	}

	// Compress in staging so failed builds leave the current admin assets intact.
	if err := filepath.WalkDir(stage, func(path string, entry fs.DirEntry, err error) error {
		if err != nil || entry.IsDir() {
			return err
		}
		switch filepath.Ext(path) {
		case ".js", ".css", ".svg":
			data, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			var compressed bytes.Buffer
			writer, err := gzip.NewWriterLevel(&compressed, gzip.BestCompression)
			if err != nil {
				return err
			}
			if _, err := writer.Write(data); err != nil {
				return err
			}
			if err := writer.Close(); err != nil {
				return err
			}
			if err := os.WriteFile(path+".gz", compressed.Bytes(), 0o644); err != nil {
				return err
			}
			return os.Remove(path)
		}
		return nil
	}); err != nil {
		return err
	}
	if err := os.MkdirAll(dist, 0o755); err != nil {
		return err
	}
	entries, err := os.ReadDir(dist)
	if err != nil {
		return err
	}
	// Keep dist itself: it is a named volume in the Docker dev environment.
	for _, entry := range entries {
		if err := os.RemoveAll(filepath.Join(dist, entry.Name())); err != nil {
			return err
		}
	}
	if err := os.CopyFS(dist, os.DirFS(stage)); err != nil {
		return err
	}
	fmt.Println("compressed admin assets ->", dist)
	return nil
}
