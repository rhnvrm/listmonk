package main

import (
	"compress/gzip"
	"io"
	"os"
	"path/filepath"
	"testing"
)

func TestAssemble(t *testing.T) {
	root := t.TempDir()
	stage, dist := filepath.Join(root, "stage"), filepath.Join(root, "dist")
	if err := os.MkdirAll(filepath.Join(stage, "js", "views"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(dist, 0o755); err != nil {
		t.Fatal(err)
	}
	before, err := os.Stat(dist)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dist, "stale.js.gz"), []byte("stale"), 0o644); err != nil {
		t.Fatal(err)
	}
	files := map[string]string{
		"js/views/campaigns.js": "export const campaign = 1;",
		"style.css":             "body { color: red; }",
		"icons.svg":             "<svg></svg>",
		"image.png":             "image data",
		"altcha.umd.js":         "public captcha",
	}
	for name, content := range files {
		if err := os.WriteFile(filepath.Join(stage, name), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	altcha := filepath.Join(root, "public-altcha.js")
	if err := assemble(stage, dist, altcha); err != nil {
		t.Fatal(err)
	}
	after, err := os.Stat(dist)
	if err != nil || !os.SameFile(before, after) {
		t.Fatalf("output directory was replaced: %v", err)
	}
	for name, expected := range files {
		path := filepath.Join(dist, name)
		if name == "altcha.umd.js" {
			path = altcha
		} else if filepath.Ext(name) != ".png" {
			if _, err := os.Stat(path); !os.IsNotExist(err) {
				t.Fatalf("uncompressed asset still exists: %s", path)
			}
			path += ".gz"
		}
		f, err := os.Open(path)
		if err != nil {
			t.Fatal(err)
		}
		var reader io.Reader = f
		if filepath.Ext(path) == ".gz" {
			gz, err := gzip.NewReader(f)
			if err != nil {
				t.Fatal(err)
			}
			defer gz.Close()
			reader = gz
		}
		actual, err := io.ReadAll(reader)
		f.Close()
		if err != nil || string(actual) != expected {
			t.Fatalf("%s: got %q, want %q (error: %v)", name, actual, expected, err)
		}
	}
	for _, name := range []string{"stale.js.gz", "altcha.umd.js", "altcha.umd.js.gz"} {
		if _, err := os.Stat(filepath.Join(dist, name)); !os.IsNotExist(err) {
			t.Fatalf("unexpected output: %s", name)
		}
	}
}

func TestAssembleMissingPublicAssetPreservesOutput(t *testing.T) {
	root := t.TempDir()
	stage, dist := filepath.Join(root, "stage"), filepath.Join(root, "dist")
	if err := os.Mkdir(dist, 0o755); err != nil {
		t.Fatal(err)
	}
	asset := filepath.Join(dist, "main.js.gz")
	if err := os.WriteFile(asset, []byte("previous build"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := assemble(stage, dist, filepath.Join(root, "altcha.js")); err == nil {
		t.Fatal("expected missing staged asset to fail")
	}
	if data, err := os.ReadFile(asset); err != nil || string(data) != "previous build" {
		t.Fatalf("failed build changed existing output: %q, %v", data, err)
	}
}
