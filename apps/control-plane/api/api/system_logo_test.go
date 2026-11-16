package api

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"hash/crc32"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"math/rand"
	"mime/multipart"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/gin-gonic/gin"
	"trojan-panel/model/constant"
)

func logoUpload(t *testing.T, filename string, data []byte) (int, string) {
	t.Helper()
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, err := writer.CreateFormFile("file", filename)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = part.Write(data); err != nil {
		t.Fatal(err)
	}
	if err = writer.Close(); err != nil {
		t.Fatal(err)
	}
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest("POST", "/api/system/uploadLogo", &body)
	ctx.Request.Header.Set("Content-Type", writer.FormDataContentType())
	UploadLogo(ctx)
	var response struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	return response.Code, response.Message
}

func logoDestination(t *testing.T) string {
	t.Helper()
	previous, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	directory := t.TempDir()
	if err := os.MkdirAll(filepath.Join(directory, filepath.Dir(constant.LogoImagePath)), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(directory); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.Chdir(previous); err != nil {
			t.Fatal(err)
		}
	})
	return filepath.Join(directory, constant.LogoImagePath)
}

func TestUploadLogoNormalizesSizeAndPreservesTransparency(t *testing.T) {
	for _, bounds := range []image.Rectangle{image.Rect(0, 0, 1024, 512), image.Rect(0, 0, 512, 1024), image.Rect(0, 0, 32, 32)} {
		t.Run(bounds.String(), func(t *testing.T) {
			destination := logoDestination(t)
			source := image.NewNRGBA(bounds)
			for y := 0; y < bounds.Dy(); y++ {
				for x := 0; x < bounds.Dx(); x++ {
					source.SetNRGBA(x, y, color.NRGBA{R: 220, A: 128})
				}
			}
			var upload bytes.Buffer
			if err := png.Encode(&upload, source); err != nil {
				t.Fatal(err)
			}
			if code, message := logoUpload(t, "brand.PNG", upload.Bytes()); code != constant.CodeSuccess {
				t.Fatalf("upload: %d %s", code, message)
			}
			data, err := os.ReadFile(destination)
			if err != nil {
				t.Fatal(err)
			}
			result, err := png.Decode(bytes.NewReader(data))
			if err != nil {
				t.Fatal(err)
			}
			if result.Bounds() != image.Rect(0, 0, 256, 256) {
				t.Fatalf("unexpected output size: %v", result.Bounds())
			}
			center := color.NRGBAModel.Convert(result.At(128, 128)).(color.NRGBA)
			if center.R < 215 || center.A < 126 || center.A > 130 {
				t.Fatalf("color/alpha changed: %+v", center)
			}
			if bounds.Dx() != bounds.Dy() {
				_, _, _, alpha := result.At(0, 0).RGBA()
				if alpha != 0 {
					t.Fatal("rectangular logo stretched or lost transparent padding")
				}
			}
			entries, err := os.ReadDir(filepath.Dir(destination))
			if err != nil || len(entries) != 1 {
				t.Fatalf("temporary files remain: %v %v", entries, err)
			}
		})
	}
}

func TestUploadLogoReducesLargeImageAndConvertsLegacyJPEG(t *testing.T) {
	destination := logoDestination(t)
	source := image.NewNRGBA(image.Rect(0, 0, 1024, 512))
	random := rand.New(rand.NewSource(1))
	for i := 0; i < len(source.Pix); i += 4 {
		source.Pix[i] = uint8(random.Intn(256))
		source.Pix[i+1] = uint8(random.Intn(256))
		source.Pix[i+2] = uint8(random.Intn(256))
		source.Pix[i+3] = 255
	}
	for _, format := range []string{"png", "jpeg"} {
		var upload bytes.Buffer
		var err error
		if format == "png" {
			err = png.Encode(&upload, source)
		} else {
			err = jpeg.Encode(&upload, source, &jpeg.Options{Quality: 90})
		}
		if err != nil {
			t.Fatal(err)
		}
		// Existing installations may contain JPEG bytes in a file named logo.png.
		if code, message := logoUpload(t, "logo.png", upload.Bytes()); code != constant.CodeSuccess {
			t.Fatalf("%s: %d %s", format, code, message)
		}
		data, err := os.ReadFile(destination)
		if err != nil {
			t.Fatal(err)
		}
		config, err := png.DecodeConfig(bytes.NewReader(data))
		if err != nil || config.Width != 256 || config.Height != 256 {
			t.Fatalf("invalid normalized PNG: %v %v", config, err)
		}
		if len(data) >= upload.Len()/2 {
			t.Fatalf("large %s not reduced: %d -> %d", format, upload.Len(), len(data))
		}
	}
}

func TestUploadLogoRejectsInvalidImagesWithoutReplacingPreviousLogo(t *testing.T) {
	destination := logoDestination(t)
	previous := []byte("previous-logo")
	if err := os.WriteFile(destination, previous, 0644); err != nil {
		t.Fatal(err)
	}
	var valid bytes.Buffer
	if err := png.Encode(&valid, image.NewNRGBA(image.Rect(0, 0, 1, 1))); err != nil {
		t.Fatal(err)
	}
	huge := append([]byte(nil), valid.Bytes()...)
	binary.BigEndian.PutUint32(huge[16:20], 16384)
	binary.BigEndian.PutUint32(huge[29:33], crc32.ChecksumIEEE(huge[12:29]))
	for _, tc := range []struct {
		name    string
		data    []byte
		message string
	}{
		{"broken.png", []byte("not an image"), constant.FileFormatError},
		{"truncated.png", valid.Bytes()[:40], constant.FileFormatError},
		{"oversized.png", bytes.Repeat([]byte("x"), 3*1024*1024+1), constant.FileSizeTooBig},
		{"oversized-body.png", bytes.Repeat([]byte("x"), 3*1024*1024+128*1024), constant.FileSizeTooBig},
		{"huge.png", huge, constant.FileFormatError},
		{"document.txt", valid.Bytes(), constant.FileFormatError},
	} {
		if code, message := logoUpload(t, tc.name, tc.data); code == constant.CodeSuccess || message != tc.message {
			t.Errorf("%s accepted/wrong failure: %d %s", tc.name, code, message)
		}
		current, err := os.ReadFile(destination)
		if err != nil || !bytes.Equal(current, previous) {
			t.Fatalf("%s replaced previous logo", tc.name)
		}
	}
}

func TestGetLogoServesInlinePNG(t *testing.T) {
	destination := logoDestination(t)
	var data bytes.Buffer
	if err := png.Encode(&data, image.NewNRGBA(image.Rect(0, 0, 256, 256))); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(destination, data.Bytes(), 0644); err != nil {
		t.Fatal(err)
	}
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest("GET", "/api/image/logo", nil)
	GetLogo(ctx)
	if recorder.Code != 200 || recorder.Header().Get("Content-Type") != "image/png" || recorder.Header().Get("Content-Disposition") != "" {
		t.Fatalf("incorrect image response: %d %v", recorder.Code, recorder.Header())
	}
	if !bytes.Equal(recorder.Body.Bytes(), data.Bytes()) {
		t.Fatal("served bytes changed")
	}
}
