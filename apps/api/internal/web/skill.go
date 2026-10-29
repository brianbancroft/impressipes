package web

import (
	"archive/zip"
	"bytes"
	"embed"
	"io/fs"
	"net/http"
	"time"
)

// skillFS holds the Claude skill members download from the user menu.
//
//go:embed all:skill
var skillFS embed.FS

// SkillHandler serves skill.zip: the files under skill/ zipped with their folder
// (skill/impressipes/SKILL.md becomes impressipes/SKILL.md), which is the layout
// Claude expects when uploading a skill. It is public; the skill has no secrets.
func SkillHandler() http.Handler {
	sub, _ := fs.Sub(skillFS, "skill")
	return skillHandler(sub)
}

// SkillMarkdownHandler serves the skill's SKILL.md as plain text, so an assistant
// that can fetch a URL can read the skill without unzipping anything.
func SkillMarkdownHandler() http.Handler {
	data, err := fs.ReadFile(skillFS, "skill/impressipes/SKILL.md")
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err != nil {
			http.Error(w, "skill unavailable", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "text/markdown; charset=utf-8")
		w.Header().Set("Cache-Control", "no-cache")
		http.ServeContent(w, r, "SKILL.md", time.Time{}, bytes.NewReader(data))
	})
}

func skillHandler(fsys fs.FS) http.Handler {
	zipped, err := zipFS(fsys)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err != nil {
			http.Error(w, "skill unavailable", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/zip")
		w.Header().Set("Content-Disposition", `attachment; filename="skill.zip"`)
		w.Header().Set("Cache-Control", "no-cache")
		http.ServeContent(w, r, "skill.zip", time.Time{}, bytes.NewReader(zipped))
	})
}

func zipFS(fsys fs.FS) ([]byte, error) {
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	err := fs.WalkDir(fsys, ".", func(name string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || d.Name() == ".gitkeep" {
			return err
		}
		data, err := fs.ReadFile(fsys, name)
		if err != nil {
			return err
		}
		// A fixed timestamp keeps the archive identical between requests and builds.
		w, err := zw.CreateHeader(&zip.FileHeader{Name: name, Method: zip.Deflate})
		if err != nil {
			return err
		}
		_, err = w.Write(data)
		return err
	})
	if err != nil {
		return nil, err
	}
	if err := zw.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
