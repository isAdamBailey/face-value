package storage

import (
	"bytes"
	"fmt"
	"image"
	"image/jpeg"
	_ "image/png" // register PNG decoding
	"math"

	"golang.org/x/image/draw"
	_ "golang.org/x/image/webp" // register WEBP decoding (decode-only; we always encode JPEG)
)

// maxEdge is the longest allowed dimension of a preprocessed image, in
// pixels. This is an appraisal tool, not a photo archive — we downscale
// before ever storing or sending an image to the vision model.
const maxEdge = 1024

// jpegQuality is the encode quality for preprocessed images.
const jpegQuality = 85

// Preprocess decodes an image, downscales it so its longest edge is at most
// maxEdge pixels (never upscaling), and re-encodes it as JPEG. Decoding to
// image.Image and re-encoding from scratch drops all source metadata,
// including EXIF GPS tags.
//
// Tradeoff: the standard decoders used here don't read the EXIF
// orientation tag, so it's discarded along with everything else rather
// than applied first — a photo whose orientation depends on that tag may
// come out visually rotated. Correcting that would need a dedicated EXIF
// parser, which is out of scope here.
func Preprocess(data []byte) (processed []byte, width, height int, err error) {
	src, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, 0, 0, fmt.Errorf("decode image: %w", err)
	}

	bounds := src.Bounds()
	srcW, srcH := bounds.Dx(), bounds.Dy()

	dstW, dstH := scaledDimensions(srcW, srcH)

	out := image.Image(src)
	if dstW != srcW || dstH != srcH {
		dst := image.NewRGBA(image.Rect(0, 0, dstW, dstH))
		draw.CatmullRom.Scale(dst, dst.Bounds(), src, bounds, draw.Over, nil)
		out = dst
	}

	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, out, &jpeg.Options{Quality: jpegQuality}); err != nil {
		return nil, 0, 0, fmt.Errorf("encode jpeg: %w", err)
	}

	return buf.Bytes(), dstW, dstH, nil
}

// scaledDimensions returns the target width/height so the longest edge is
// at most maxEdge, preserving aspect ratio. Images already within bounds
// are returned unchanged — never upscaled.
func scaledDimensions(w, h int) (int, int) {
	if w <= maxEdge && h <= maxEdge {
		return w, h
	}
	if w >= h {
		scaled := int(math.Round(float64(h) * float64(maxEdge) / float64(w)))
		return maxEdge, max(scaled, 1)
	}
	scaled := int(math.Round(float64(w) * float64(maxEdge) / float64(h)))
	return max(scaled, 1), maxEdge
}
