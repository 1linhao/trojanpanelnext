package util

import (
	"bytes"
	"image"
	"image/jpeg"
	"image/png"
	"os"
	"path/filepath"
	"testing"
)

func TestOptimizePanelLogoFileMigratesLegacyImageOnce(t *testing.T) {
	filename := filepath.Join(t.TempDir(), "logo.png")
	var original bytes.Buffer
	if err := jpeg.Encode(&original, image.NewRGBA(image.Rect(0, 0, 1153, 1080)), nil); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filename, original.Bytes(), 0644); err != nil {
		t.Fatal(err)
	}
	if err := OptimizePanelLogoFile(filename); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filename)
	if err != nil {
		t.Fatal(err)
	}
	config, err := png.DecodeConfig(bytes.NewReader(data))
	if err != nil || config.Width != PanelLogoSize || config.Height != PanelLogoSize {
		t.Fatalf("migration failed: %+v %v", config, err)
	}
	before, err := os.Stat(filename)
	if err != nil {
		t.Fatal(err)
	}
	if err := OptimizePanelLogoFile(filename); err != nil {
		t.Fatal(err)
	}
	after, err := os.Stat(filename)
	if err != nil {
		t.Fatal(err)
	}
	if !os.SameFile(before, after) {
		t.Fatal("already normalized logo replaced at startup")
	}
}

func TestOptimizePanelLogoFilePreservesInvalidLegacyImage(t *testing.T) {
	filename := filepath.Join(t.TempDir(), "logo.png")
	original := []byte("invalid image")
	if err := os.WriteFile(filename, original, 0644); err != nil {
		t.Fatal(err)
	}
	if err := OptimizePanelLogoFile(filename); err != ErrPanelLogoFormat {
		t.Fatalf("wrong error: %v", err)
	}
	data, err := os.ReadFile(filename)
	if err != nil || !bytes.Equal(data, original) {
		t.Fatal("invalid logo was destroyed")
	}
}
