package server

import (
	"context"
	"errors"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"strings"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/brianbancroft/impressipies/apps/api/internal/api"
	"github.com/brianbancroft/impressipies/apps/api/internal/db/dbgen"
	"github.com/brianbancroft/impressipies/apps/api/internal/images"
	"github.com/brianbancroft/impressipies/apps/api/internal/uuidx"
)

const (
	maxUploadBytes   = 10 << 20
	maxImagesPerItem = 12
)

// imageDir is where image variants live: IMAGE_DIR, which in production is a volume.
func imageDir() string {
	if d := os.Getenv("IMAGE_DIR"); d != "" {
		return d
	}
	return "data/images"
}

func imageRef(id pgtype.UUID) api.Image {
	s := uuidx.String(id)
	return api.Image{Id: s, ThumbUrl: "/images/" + s + "/thumb", LargeUrl: "/images/" + s + "/large"}
}

func (s *server) UploadRecipeImage(ctx context.Context, req api.UploadRecipeImageRequestObject) (api.UploadRecipeImageResponseObject, error) {
	if _, err := s.member(ctx); err != nil {
		return nil, err
	}
	recipeID, err := idOf(req.RecipeId, "recipe")
	if err != nil {
		return nil, err
	}
	if _, err := s.q.GetRecipe(ctx, recipeID); err != nil {
		return nil, orNotFound(err, "recipe")
	}
	if req.Body == nil {
		return nil, fail(http.StatusBadRequest, "expected a multipart upload")
	}
	data, err := readFilePart(req.Body)
	if err != nil {
		return nil, err
	}
	v, err := images.Process(data)
	switch {
	case errors.Is(err, images.ErrTooLarge):
		return nil, fail(http.StatusRequestEntityTooLarge, "image has too many pixels (max 25 megapixels)")
	case errors.Is(err, images.ErrUnsupported):
		return nil, fail(http.StatusUnsupportedMediaType, err.Error())
	case err != nil:
		return nil, err
	}

	id := uuidx.New()
	err = s.withTx(ctx, func(q *dbgen.Queries) error {
		// The recipe row serialises concurrent uploads, so the cap cannot be raced past.
		if _, err := q.LockRecipe(ctx, recipeID); err != nil {
			return orNotFound(err, "recipe")
		}
		n, err := q.CountRecipeImages(ctx, recipeID)
		if err != nil {
			return err
		}
		if n >= maxImagesPerItem {
			return fail(http.StatusConflict, "a recipe holds at most 12 photos")
		}
		if _, err := q.InsertRecipeImage(ctx, dbgen.InsertRecipeImageParams{ID: id, RecipeID: recipeID}); err != nil {
			return err
		}
		return s.images.Save(uuidx.String(id), v)
	})
	if err != nil {
		s.images.Remove(uuidx.String(id))
		return nil, err
	}
	return api.UploadRecipeImage201JSONResponse(imageRef(id)), nil
}

// readFilePart returns the "file" part of an upload, refusing anything over the size cap.
func readFilePart(mr *multipart.Reader) ([]byte, error) {
	for {
		part, err := mr.NextPart()
		if err == io.EOF {
			return nil, fail(http.StatusBadRequest, "missing file part")
		}
		if err != nil {
			return nil, fail(http.StatusBadRequest, "invalid multipart upload")
		}
		if part.FormName() != "file" {
			continue
		}
		data, err := io.ReadAll(io.LimitReader(part, maxUploadBytes+1))
		if err != nil {
			return nil, fail(http.StatusBadRequest, "upload interrupted")
		}
		if len(data) > maxUploadBytes {
			return nil, fail(http.StatusRequestEntityTooLarge, "image is larger than 10 MB")
		}
		return data, nil
	}
}

func (s *server) DeleteRecipeImage(ctx context.Context, req api.DeleteRecipeImageRequestObject) (api.DeleteRecipeImageResponseObject, error) {
	if _, err := s.member(ctx); err != nil {
		return nil, err
	}
	recipeID, err := idOf(req.RecipeId, "recipe")
	if err != nil {
		return nil, err
	}
	id, err := idOf(req.ImageId, "image")
	if err != nil {
		return nil, err
	}
	n, err := s.q.DeleteRecipeImage(ctx, dbgen.DeleteRecipeImageParams{ID: id, RecipeID: recipeID})
	if err != nil {
		return nil, err
	}
	if n == 0 {
		return nil, notFound("image")
	}
	s.images.Remove(uuidx.String(id))
	return api.DeleteRecipeImage204Response{}, nil
}

// serveImage answers GET /images/{id}/{variant}. It is outside the OpenAPI document
// because it returns raw bytes. A signed-in member can fetch any image of a live recipe.
func (s *server) serveImage(w http.ResponseWriter, r *http.Request) {
	if _, f := s.current(r.Context()); f != nil {
		writeError(w, f.code, f.msg)
		return
	}
	if s.pool == nil {
		writeError(w, http.StatusServiceUnavailable, "database not configured")
		return
	}
	variant := r.PathValue("variant")
	id, ok := uuidx.Parse(r.PathValue("id"))
	if !ok || !images.Valid(variant) {
		writeError(w, http.StatusNotFound, "image not found")
		return
	}
	if _, err := s.q.GetLiveRecipeImage(r.Context(), id); err != nil {
		writeError(w, http.StatusNotFound, "image not found")
		return
	}
	f, err := s.images.Open(uuidx.String(id), variant)
	if err != nil {
		writeError(w, http.StatusNotFound, "image not found")
		return
	}
	defer f.Close()
	// An image id never changes content, so clients may keep it indefinitely. "private"
	// keeps shared caches from serving it to someone who is not signed in.
	etag := `"` + uuidx.String(id) + "-" + variant + `"`
	h := w.Header()
	h.Set("Content-Type", "image/webp")
	h.Set("Cache-Control", "private, max-age=31536000, immutable")
	h.Set("ETag", etag)
	h.Set("X-Content-Type-Options", "nosniff")
	if strings.Contains(r.Header.Get("If-None-Match"), etag) {
		w.WriteHeader(http.StatusNotModified)
		return
	}
	st, err := f.Stat()
	if err == nil {
		http.ServeContent(w, r, "", st.ModTime(), f)
		return
	}
	io.Copy(w, f)
}
