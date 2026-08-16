package storage

import (
	"bytes"
	"image"
	"image/jpeg"
	"image/png"
	"testing"
)

func encodeJPEG(t *testing.T, w, h int) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, nil); err != nil {
		t.Fatalf("encode jpeg: %v", err)
	}
	return buf.Bytes()
}

func encodePNG(t *testing.T, w, h int) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatalf("encode png: %v", err)
	}
	return buf.Bytes()
}

func TestDetectContentType(t *testing.T) {
	jpg := encodeJPEG(t, 10, 10)
	pngBytes := encodePNG(t, 10, 10)
	webp := []byte("RIFF\x00\x00\x00\x00WEBPVP8 ") // minimal RIFF/WEBP sniff header

	tests := []struct {
		name    string
		data    []byte
		want    string
		wantErr bool
	}{
		{"jpeg", jpg, "image/jpeg", false},
		{"png", pngBytes, "image/png", false},
		{"webp", webp, "image/webp", false},
		{"plain text rejected", []byte("not an image, just text padding to be long enough"), "", true},
		{"gif rejected", []byte("GIF89a" + string(make([]byte, 20))), "", true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := DetectContentType(tc.data)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("DetectContentType() = %q, nil, want error", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("DetectContentType(): %v", err)
			}
			if got != tc.want {
				t.Errorf("DetectContentType() = %q, want %q", got, tc.want)
			}
		})
	}
}
