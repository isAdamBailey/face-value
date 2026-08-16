package storage

import (
	"fmt"
	"net/http"
	"strings"
)

// allowedContentTypes are the only image types accepted for upload.
var allowedContentTypes = map[string]bool{
	"image/jpeg": true,
	"image/png":  true,
	"image/webp": true,
}

// DetectContentType sniffs the actual content type of data — never the
// filename or a client-supplied MIME header, both of which are trivially
// spoofable — and returns it if it's an allowed image type.
func DetectContentType(data []byte) (string, error) {
	mime := http.DetectContentType(data)
	if idx := strings.IndexByte(mime, ';'); idx != -1 {
		mime = strings.TrimSpace(mime[:idx])
	}

	if !allowedContentTypes[mime] {
		return "", fmt.Errorf("unsupported content type %q", mime)
	}

	return mime, nil
}
