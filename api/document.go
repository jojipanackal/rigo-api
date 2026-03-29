package api

import (
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/jojipanackal/rugo/middlewares"
	"github.com/jojipanackal/rugo/models"
)

type DocumentHandler struct {
	DocumentModel *models.DocumentModel
	DeckModel     *models.DeckModel
}

// GET /api/documents/{docType}/{refId}  — serves file bytes
func (h *DocumentHandler) Serve(w http.ResponseWriter, r *http.Request) {
	docType := r.PathValue("docType")
	refId, err := strconv.ParseInt(r.PathValue("refId"), 10, 64)
	if err != nil {
		WriteError(w, http.StatusBadRequest, "invalid reference id")
		return
	}

	doc, err := h.DocumentModel.GetByRef(docType, refId)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	contentType := "application/octet-stream"
	switch strings.ToLower(doc.DocFormat) {
	case "png":
		contentType = "image/png"
	case "jpg", "jpeg":
		contentType = "image/jpeg"
	case "gif":
		contentType = "image/gif"
	case "webp":
		contentType = "image/webp"
	case "svg":
		contentType = "image/svg+xml"
	}

	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Content-Length", strconv.Itoa(doc.FileSize))
	w.Header().Set("Cache-Control", "public, max-age=86400")
	w.Write(doc.FileData)
}

// PUT /api/documents/{docType}/{refId}  — upload or replace (protected)
func (h *DocumentHandler) Upload(w http.ResponseWriter, r *http.Request) {
	docType := r.PathValue("docType")
	refId, err := strconv.ParseInt(r.PathValue("refId"), 10, 64)
	if err != nil {
		WriteError(w, http.StatusBadRequest, "invalid reference id")
		return
	}

	// Require auth
	if middlewares.GetUserID(r.Context()) == 0 {
		WriteError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 5<<20) // 5MB limit
	file, header, err := r.FormFile("file")
	if err != nil {
		WriteError(w, http.StatusBadRequest, "file field is required")
		return
	}
	defer file.Close()

	data, err := io.ReadAll(file)
	if err != nil {
		WriteError(w, http.StatusInternalServerError, "could not read file")
		return
	}

	ext := strings.TrimPrefix(filepath.Ext(header.Filename), ".")

	if err := h.DocumentModel.Upsert(docType, ext, refId, header.Filename, data); err != nil {
		WriteError(w, http.StatusInternalServerError, "could not save file")
		return
	}

	if docType == "deck_header" && h.DeckModel != nil {
		if err := h.DeckModel.UpdateHeaderImageURL(refId, deckHeaderURL(docType, refId)); err != nil {
			WriteError(w, http.StatusInternalServerError, "could not update deck header")
			return
		}
	}
	WriteJSON(w, http.StatusOK, map[string]string{
		"doc_type": docType,
		"filename": header.Filename,
	})
}

// DELETE /api/documents/{docType}/{refId}  — remove file (protected)
func (h *DocumentHandler) Delete(w http.ResponseWriter, r *http.Request) {
	docType := r.PathValue("docType")
	refId, err := strconv.ParseInt(r.PathValue("refId"), 10, 64)
	if err != nil {
		WriteError(w, http.StatusBadRequest, "invalid reference id")
		return
	}

	if err := h.DocumentModel.DeleteByRef(docType, refId); err != nil {
		WriteError(w, http.StatusInternalServerError, "could not delete document")
		return
	}
	if docType == "deck_header" && h.DeckModel != nil {
		h.DeckModel.UpdateHeaderImageURL(refId, "")
	}
	WriteJSON(w, http.StatusOK, map[string]string{"message": "document deleted"})
}

func deckHeaderURL(docType string, refId int64) string {
	return fmt.Sprintf("/api/documents/%s/%d", docType, refId)
}
