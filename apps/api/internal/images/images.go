// Package images turns an uploaded photo into the two WebP variants the API serves
// (a card thumbnail and a large view) and keeps them on disk.
package images

import (
	"bytes"
	"errors"
	"fmt"
	"image"
	_ "image/gif" // register decoders for uploads
	_ "image/jpeg"
	_ "image/png"
	"io"
	"os"
	"path/filepath"

	"github.com/HugoSmits86/nativewebp"
	"golang.org/x/image/draw"
	_ "golang.org/x/image/webp"
)

// Longest edge of each variant, in pixels. Images are never scaled up.
const (
	ThumbEdge = 360
	LargeEdge = 1280

	// MaxPixels bounds the memory a decode can take (4 bytes per pixel).
	MaxPixels = 25_000_000
)

const (
	Thumb = "thumb"
	Large = "large"
)

var (
	ErrUnsupported = errors.New("unsupported image: upload a JPEG, PNG, GIF or WebP")
	ErrTooLarge    = errors.New("image has too many pixels")
)

// Variants is the processed upload.
type Variants struct{ Thumb, Large []byte }

// Process decodes an upload and renders both variants as lossless WebP.
func Process(data []byte) (*Variants, error) {
	cfg, _, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		return nil, ErrUnsupported
	}
	if cfg.Width <= 0 || cfg.Height <= 0 || cfg.Width*cfg.Height > MaxPixels {
		return nil, ErrTooLarge
	}
	src, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, ErrUnsupported
	}
	var v Variants
	if v.Large, err = render(src, LargeEdge); err != nil {
		return nil, err
	}
	if v.Thumb, err = render(src, ThumbEdge); err != nil {
		return nil, err
	}
	return &v, nil
}

func render(src image.Image, edge int) ([]byte, error) {
	b := src.Bounds()
	w, h := b.Dx(), b.Dy()
	if long := max(w, h); long > edge {
		w, h = max(1, w*edge/long), max(1, h*edge/long)
	}
	dst := image.NewNRGBA(image.Rect(0, 0, w, h))
	draw.CatmullRom.Scale(dst, dst.Bounds(), src, b, draw.Src, nil)
	var out bytes.Buffer
	if err := nativewebp.Encode(&out, dst, nil); err != nil {
		return nil, fmt.Errorf("encode webp: %w", err)
	}
	return out.Bytes(), nil
}

// Store keeps variants under dir/<id>/<variant>.webp.
type Store struct{ Dir string }

func (s Store) path(id, variant string) string { return filepath.Join(s.Dir, id, variant+".webp") }

// Valid reports whether variant names a stored variant.
func Valid(variant string) bool { return variant == Thumb || variant == Large }

// Save writes both variants. Each file lands atomically, so a reader never sees half an image.
func (s Store) Save(id string, v *Variants) error {
	if err := os.MkdirAll(filepath.Join(s.Dir, id), 0o755); err != nil {
		return err
	}
	for variant, data := range map[string][]byte{Thumb: v.Thumb, Large: v.Large} {
		if err := writeAtomic(s.path(id, variant), data); err != nil {
			s.Remove(id)
			return err
		}
	}
	return nil
}

func writeAtomic(path string, data []byte) error {
	tmp, err := os.CreateTemp(filepath.Dir(path), ".upload-*")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	if _, err := io.Copy(tmp, bytes.NewReader(data)); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), path)
}

// Open returns a stored variant, or os.ErrNotExist.
func (s Store) Open(id, variant string) (*os.File, error) {
	if !Valid(variant) {
		return nil, os.ErrNotExist
	}
	return os.Open(s.path(id, variant))
}

// Remove deletes an image's files. A missing image is not an error.
func (s Store) Remove(id string) { _ = os.RemoveAll(filepath.Join(s.Dir, id)) }
