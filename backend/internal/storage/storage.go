// Package storage stores and retrieves appraisal images. It also handles
// upload validation and preprocessing (resize, EXIF strip) before an image
// ever reaches an ImageStore.
package storage

import (
	"context"
	"io"
)

// ImageStore stores and retrieves appraisal images. Callers persist only
// the key returned by Put — never a full URL — so the bucket or region can
// change without a data migration.
type ImageStore interface {
	// Put uploads r (expected to already be preprocessed: resized,
	// EXIF-stripped, JPEG) and returns its key.
	Put(ctx context.Context, r io.Reader, mime string) (key string, err error)
	// Get returns the object's bytes and content type.
	Get(ctx context.Context, key string) (io.ReadCloser, string, error)
	// URL returns a time-limited, presigned GET URL for key.
	URL(ctx context.Context, key string) (string, error)
	// Delete removes the object at key. A missing object is not an error —
	// deleting the containing row should not be blocked by an
	// already-missing image.
	Delete(ctx context.Context, key string) error
}
