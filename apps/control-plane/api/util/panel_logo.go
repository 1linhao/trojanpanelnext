package util

import (
	"bytes"
	"errors"
	"image"
	_ "image/jpeg"
	"image/png"
	"io"
	"os"
	"path/filepath"

	"golang.org/x/image/draw"
)

const (
	PanelLogoSize         = 256
	PanelLogoMaxBytes     = 3 * 1024 * 1024
	panelLogoMaxDimension = 4096
)

var (
	ErrPanelLogoFormat = errors.New("invalid panel logo image")
	ErrPanelLogoSize   = errors.New("panel logo exceeds upload limit")
)

// OptimizePanelLogoFile upgrades existing/default logos once. Already
// normalized PNGs are left untouched on subsequent starts.
func OptimizePanelLogoFile(filename string) error {
	file, err := os.Open(filename)
	if err != nil {
		return err
	}
	defer file.Close()
	config, format, err := image.DecodeConfig(file)
	if err != nil {
		return ErrPanelLogoFormat
	}
	if format == "png" && config.Width == PanelLogoSize && config.Height == PanelLogoSize {
		return nil
	}
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		return err
	}
	return SavePanelLogo(file, filename)
}

// SavePanelLogo writes a fixed-size, losslessly compressed PNG. Decode and
// encode before replacing the old file so failed uploads leave it intact.
func SavePanelLogo(reader io.Reader, destination string) error {
	data, err := io.ReadAll(io.LimitReader(reader, PanelLogoMaxBytes+1))
	if err != nil {
		return err
	}
	if len(data) > PanelLogoMaxBytes {
		return ErrPanelLogoSize
	}
	config, format, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil || (format != "png" && format != "jpeg") || config.Width <= 0 || config.Height <= 0 || config.Width > panelLogoMaxDimension || config.Height > panelLogoMaxDimension {
		return ErrPanelLogoFormat
	}
	source, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return ErrPanelLogoFormat
	}
	longest := config.Width
	if config.Height > longest {
		longest = config.Height
	}
	width, height := PanelLogoSize*config.Width/longest, PanelLogoSize*config.Height/longest
	if width < 1 {
		width = 1
	}
	if height < 1 {
		height = 1
	}
	left, top := (PanelLogoSize-width)/2, (PanelLogoSize-height)/2
	output := image.NewNRGBA(image.Rect(0, 0, PanelLogoSize, PanelLogoSize))
	draw.CatmullRom.Scale(output, image.Rect(left, top, left+width, top+height), source, source.Bounds(), draw.Src, nil)

	file, err := os.CreateTemp(filepath.Dir(destination), ".panel-logo-*.png")
	if err != nil {
		return err
	}
	defer os.Remove(file.Name())
	defer file.Close()
	encoder := png.Encoder{CompressionLevel: png.BestCompression}
	if err := encoder.Encode(file, output); err != nil {
		return err
	}
	if err := file.Chmod(0644); err != nil {
		return err
	}
	if err := file.Close(); err != nil {
		return err
	}
	return os.Rename(file.Name(), destination)
}
