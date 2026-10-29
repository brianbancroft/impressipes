package server

import (
	"bytes"
	"encoding/json"
	"image"
	"image/jpeg"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func (e *dataEnv) upload(recipeID string, file []byte, want int, out any) {
	e.t.Helper()
	var body bytes.Buffer
	mw := multipart.NewWriter(&body)
	fw, _ := mw.CreateFormFile("file", "photo.jpg")
	fw.Write(file)
	mw.Close()
	req := httptest.NewRequest("POST", "/recipes/"+recipeID+"/images", &body)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	req.AddCookie(e.cookie)
	rec := httptest.NewRecorder()
	e.h.ServeHTTP(rec, req)
	if rec.Code != want {
		e.t.Fatalf("upload: got %d, want %d: %s", rec.Code, want, rec.Body)
	}
	if out != nil {
		if err := json.Unmarshal(rec.Body.Bytes(), out); err != nil {
			e.t.Fatal(err)
		}
	}
}

func TestRecipeImages(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("IMAGE_DIR", dir)
	e := newDataEnv(t)

	var r obj
	e.call("POST", "/recipes", `{"title":"Soup","ingredients":[{"item":"Leek"}],"steps":[{"text":"Boil."}]}`, 201, &r)
	rid := r["id"].(string)
	var jpg bytes.Buffer
	jpeg.Encode(&jpg, image.NewRGBA(image.Rect(0, 0, 800, 600)), nil)

	var img obj
	e.upload(rid, jpg.Bytes(), 201, &img)
	thumb, large := img["thumbUrl"].(string), img["largeUrl"].(string)
	if thumb != "/images/"+img["id"].(string)+"/thumb" {
		t.Fatalf("image ref: %v", img)
	}

	// The recipe lists its photos, and its summary carries the first as a cover.
	e.call("GET", "/recipes/"+rid, "", 200, &r)
	if imgs := r["images"].([]any); len(imgs) != 1 || r["coverUrl"] != thumb {
		t.Fatalf("recipe images: %v cover=%v", r["images"], r["coverUrl"])
	}
	var list []obj
	e.call("GET", "/recipes", "", 200, &list)
	if list[0]["coverUrl"] != thumb {
		t.Fatalf("summary cover: %v", list[0])
	}

	// Served as WebP, signed-in only, cacheable, and revalidated by ETag.
	rec := do(e.h, "GET", thumb, "", e.cookie)
	if rec.Code != 200 || rec.Header().Get("Content-Type") != "image/webp" ||
		rec.Header().Get("Cache-Control") != "private, max-age=31536000, immutable" ||
		!bytes.HasPrefix(rec.Body.Bytes(), []byte("RIFF")) {
		t.Fatalf("thumb: %d %v", rec.Code, rec.Header())
	}
	if rec := do(e.h, "GET", large, ""); rec.Code != http.StatusUnauthorized {
		t.Fatalf("no session: got %d, want 401", rec.Code)
	}
	req := httptest.NewRequest("GET", thumb, nil)
	req.AddCookie(e.cookie)
	req.Header.Set("If-None-Match", rec.Header().Get("ETag"))
	cached := httptest.NewRecorder()
	e.h.ServeHTTP(cached, req)
	if cached.Code != http.StatusNotModified {
		t.Fatalf("etag: got %d, want 304", cached.Code)
	}
	if rec := do(e.h, "GET", "/images/"+img["id"].(string)+"/huge", "", e.cookie); rec.Code != 404 {
		t.Fatalf("unknown variant: %d", rec.Code)
	}

	// Bad uploads.
	e.upload(rid, []byte("not an image"), http.StatusUnsupportedMediaType, nil)
	e.upload(rid, append([]byte{0xff, 0xd8, 0xff}, make([]byte, maxUploadBytes+1)...), http.StatusRequestEntityTooLarge, nil)
	e.upload("00000000-0000-4000-8000-000000000000", jpg.Bytes(), 404, nil)
	for i := 1; i < maxImagesPerItem; i++ {
		e.upload(rid, jpg.Bytes(), 201, nil)
	}
	e.upload(rid, jpg.Bytes(), http.StatusConflict, nil)

	// A deleted recipe's photos stop being served; restoring it brings them back.
	e.call("DELETE", "/recipes/"+rid, "", 204, nil)
	if rec := do(e.h, "GET", thumb, "", e.cookie); rec.Code != 404 {
		t.Fatalf("image of deleted recipe: %d", rec.Code)
	}
	var del []obj
	e.call("GET", "/activity?action=recipe_deleted", "", 200, &del)
	e.call("POST", "/activity/"+itoa(del[0]["id"])+"/undo", "{}", 200, nil)
	if rec := do(e.h, "GET", thumb, "", e.cookie); rec.Code != 200 {
		t.Fatalf("image after restore: %d", rec.Code)
	}

	// Deleting removes the row and the files.
	e.call("DELETE", "/recipes/"+rid+"/images/"+img["id"].(string), "", 204, nil)
	e.call("DELETE", "/recipes/"+rid+"/images/"+img["id"].(string), "", 404, nil)
	if _, err := os.Stat(filepath.Join(dir, img["id"].(string))); !os.IsNotExist(err) {
		t.Fatalf("files not removed: %v", err)
	}
	if rec := do(e.h, "GET", thumb, "", e.cookie); rec.Code != 404 {
		t.Fatalf("deleted image: %d", rec.Code)
	}
}
