package web

import (
	"archive/zip"
	"bytes"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"
)

func TestHandler(t *testing.T) {
	h := handler(fstest.MapFS{
		"index.html":   {Data: []byte("<html>app</html>")},
		"assets/a.js":  {Data: []byte("js")},
		"favicon.ico":  {Data: []byte("ico")},
		"assets/b.css": {Data: []byte("css")},
	})
	cases := []struct {
		path  string
		code  int
		body  string
		cache string
	}{
		{"/", 200, "<html>app</html>", ""},
		{"/invite/abc", 200, "<html>app</html>", "no-cache"},
		{"/assets/a.js", 200, "js", "public, max-age=31536000, immutable"},
		{"/favicon.ico", 200, "ico", ""},
		{"/assets/missing.js", 404, "", ""},
	}
	for _, c := range cases {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, c.path, nil))
		if rec.Code != c.code {
			t.Errorf("%s: code %d, want %d", c.path, rec.Code, c.code)
		}
		if c.body != "" && rec.Body.String() != c.body {
			t.Errorf("%s: body %q, want %q", c.path, rec.Body.String(), c.body)
		}
		if got := rec.Header().Get("Cache-Control"); got != c.cache {
			t.Errorf("%s: cache %q, want %q", c.path, got, c.cache)
		}
	}
}

func TestHandlerWithoutBuild(t *testing.T) {
	rec := httptest.NewRecorder()
	handler(fstest.MapFS{}).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	if rec.Code != 404 {
		t.Errorf("code %d, want 404", rec.Code)
	}
}

func TestSkillHandler(t *testing.T) {
	rec := httptest.NewRecorder()
	SkillHandler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/skill", nil))
	if rec.Code != 200 {
		t.Fatalf("code %d, want 200", rec.Code)
	}
	if got := rec.Header().Get("Content-Type"); got != "application/zip" {
		t.Errorf("content type %q", got)
	}
	if got := rec.Header().Get("Content-Disposition"); !strings.Contains(got, "skill.zip") {
		t.Errorf("content disposition %q", got)
	}
	zr, err := zip.NewReader(bytes.NewReader(rec.Body.Bytes()), int64(rec.Body.Len()))
	if err != nil {
		t.Fatal(err)
	}
	var found bool
	for _, f := range zr.File {
		if f.Name == "impressipes/SKILL.md" {
			found = true
		}
	}
	if !found {
		t.Errorf("impressipes/SKILL.md missing from %d files", len(zr.File))
	}
}
