package images

import (
	"bytes"
	"errors"
	"image"
	"image/color"
	"image/gif"
	"image/jpeg"
	"image/png"
	"testing"

	"golang.org/x/image/webp"
)

func photo(w, h int) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.Set(x, y, color.RGBA{uint8(x), uint8(y), uint8(x ^ y), 255})
		}
	}
	return img
}

func size(t *testing.T, webpData []byte) (int, int) {
	t.Helper()
	img, err := webp.Decode(bytes.NewReader(webpData))
	if err != nil {
		t.Fatalf("output is not valid webp: %v", err)
	}
	return img.Bounds().Dx(), img.Bounds().Dy()
}

func TestProcessResizesAndKeepsAspect(t *testing.T) {
	var jpg, pg, gf bytes.Buffer
	jpeg.Encode(&jpg, photo(2000, 1000), nil)
	png.Encode(&pg, photo(2000, 1000))
	gif.Encode(&gf, photo(300, 200), nil)

	for name, in := range map[string][]byte{"jpeg": jpg.Bytes(), "png": pg.Bytes()} {
		v, err := Process(in)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if w, h := size(t, v.Large); w != LargeEdge || h != LargeEdge/2 {
			t.Errorf("%s large = %dx%d", name, w, h)
		}
		if w, h := size(t, v.Thumb); w != ThumbEdge || h != ThumbEdge/2 {
			t.Errorf("%s thumb = %dx%d", name, w, h)
		}
	}

	// Small images are not scaled up; the webp output also decodes back in (re-upload).
	v, err := Process(gf.Bytes())
	if err != nil {
		t.Fatal(err)
	}
	if w, h := size(t, v.Large); w != 300 || h != 200 {
		t.Errorf("small large = %dx%d, want 300x200", w, h)
	}
	if _, err := Process(v.Large); err != nil {
		t.Errorf("webp upload: %v", err)
	}
}

func TestProcessRejects(t *testing.T) {
	if _, err := Process([]byte("not an image")); !errors.Is(err, ErrUnsupported) {
		t.Errorf("garbage: %v", err)
	}
	// A tiny PNG header claiming a huge canvas must be refused before decoding.
	var pg bytes.Buffer
	png.Encode(&pg, image.NewGray(image.Rect(0, 0, 6000, 6000)))
	if _, err := Process(pg.Bytes()); !errors.Is(err, ErrTooLarge) {
		t.Errorf("36MP: %v", err)
	}
}

func TestStore(t *testing.T) {
	s := Store{Dir: t.TempDir()}
	v := &Variants{Thumb: []byte("t"), Large: []byte("l")}
	if err := s.Save("abc", v); err != nil {
		t.Fatal(err)
	}
	f, err := s.Open("abc", Thumb)
	if err != nil {
		t.Fatal(err)
	}
	f.Close()
	if _, err := s.Open("abc", "../../etc/passwd"); err == nil {
		t.Error("invalid variant should not open")
	}
	s.Remove("abc")
	if _, err := s.Open("abc", Large); err == nil {
		t.Error("removed image still opens")
	}
	s.Remove("abc") // idempotent
}
