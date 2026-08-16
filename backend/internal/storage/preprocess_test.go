package storage

import (
	"bytes"
	"image/jpeg"
	"testing"
)

func TestPreprocess_DownscalesLongestEdge(t *testing.T) {
	src := encodeJPEG(t, 2000, 1000)

	processed, w, h, err := Preprocess(src)
	if err != nil {
		t.Fatalf("Preprocess: %v", err)
	}

	if w != maxEdge {
		t.Errorf("width = %d, want %d", w, maxEdge)
	}
	if h != maxEdge/2 {
		t.Errorf("height = %d, want %d (aspect ratio preserved)", h, maxEdge/2)
	}

	decoded, err := jpeg.Decode(bytes.NewReader(processed))
	if err != nil {
		t.Fatalf("decode processed output: %v", err)
	}
	bounds := decoded.Bounds()
	if bounds.Dx() != w || bounds.Dy() != h {
		t.Errorf("decoded dimensions = %dx%d, want %dx%d", bounds.Dx(), bounds.Dy(), w, h)
	}
}

func TestPreprocess_DoesNotUpscaleSmallImages(t *testing.T) {
	src := encodeJPEG(t, 500, 400)

	_, w, h, err := Preprocess(src)
	if err != nil {
		t.Fatalf("Preprocess: %v", err)
	}

	if w != 500 || h != 400 {
		t.Errorf("dimensions = %dx%d, want unchanged 500x400", w, h)
	}
}

func TestPreprocess_PortraitOrientation(t *testing.T) {
	src := encodeJPEG(t, 800, 2400)

	_, w, h, err := Preprocess(src)
	if err != nil {
		t.Fatalf("Preprocess: %v", err)
	}

	if h != maxEdge {
		t.Errorf("height = %d, want %d", h, maxEdge)
	}
	wantW := maxEdge / 3
	if w != wantW {
		t.Errorf("width = %d, want %d (aspect ratio preserved)", w, wantW)
	}
}

func TestPreprocess_AlwaysOutputsJPEG(t *testing.T) {
	src := encodePNG(t, 100, 100)

	processed, _, _, err := Preprocess(src)
	if err != nil {
		t.Fatalf("Preprocess: %v", err)
	}

	if _, err := jpeg.Decode(bytes.NewReader(processed)); err != nil {
		t.Errorf("output is not valid JPEG: %v", err)
	}
}

func TestPreprocess_RejectsUndecodableInput(t *testing.T) {
	_, _, _, err := Preprocess([]byte("not an image"))
	if err == nil {
		t.Fatal("Preprocess() error = nil, want error for undecodable input")
	}
}

func TestScaledDimensions(t *testing.T) {
	tests := []struct {
		w, h  int
		wantW int
		wantH int
	}{
		{2000, 1000, 1024, 512},
		{1000, 2000, 512, 1024},
		{500, 400, 500, 400},
		{1024, 1024, 1024, 1024},
		{1025, 1025, 1024, 1024},
	}

	for _, tc := range tests {
		gotW, gotH := scaledDimensions(tc.w, tc.h)
		if gotW != tc.wantW || gotH != tc.wantH {
			t.Errorf("scaledDimensions(%d, %d) = (%d, %d), want (%d, %d)", tc.w, tc.h, gotW, gotH, tc.wantW, tc.wantH)
		}
	}
}
