package api

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/jojipanackal/rigo-api/middlewares"
	"github.com/jojipanackal/rigo-api/models"
)

type DeckHandler struct {
	DeckModel     *models.DeckModel
	AuthModel     *models.AuthModel
	UserModel     *models.UserModel
	RatingModel   *models.RatingModel
	BookmarkModel *models.BookmarkModel
	SessionModel  *models.SessionModel
	DocumentModel *models.DocumentModel
}

// GET /api/decks?page=1&limit=20
func (h *DeckHandler) List(w http.ResponseWriter, r *http.Request) {
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 100 {
		limit = 20
	}
	offset := (page - 1) * limit

	userId := GetCurrentUserID(r, h.AuthModel)
	decks, err := h.DeckModel.GetAll(limit, offset, userId)
	if err != nil {
		WriteError(w, http.StatusInternalServerError, "could not fetch decks")
		return
	}
	WriteJSON(w, http.StatusOK, map[string]any{"decks": decks, "page": page, "limit": limit})
}

// GET /api/decks/popular?page=1&limit=20
func (h *DeckHandler) Popular(w http.ResponseWriter, r *http.Request) {
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 100 {
		limit = 20
	}
	offset := (page - 1) * limit

	userId := GetCurrentUserID(r, h.AuthModel)
	decks, err := h.DeckModel.GetPopular(limit, offset, userId)
	if err != nil {
		WriteError(w, http.StatusInternalServerError, "could not fetch decks")
		return
	}
	total := h.DeckModel.GetPopularCount()
	WriteJSON(w, http.StatusOK, map[string]any{
		"decks": decks, "page": page, "limit": limit, "total": total,
	})
}

// GET /api/decks/search?q=golang&limit=20
func (h *DeckHandler) Search(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query().Get("q")
	if q == "" {
		WriteError(w, http.StatusBadRequest, "q is required")
		return
	}
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	if limit < 1 || limit > 100 {
		limit = 20
	}
	userId := GetCurrentUserID(r, h.AuthModel)
	decks, err := h.DeckModel.Search(q, limit, userId)
	if err != nil {
		WriteError(w, http.StatusInternalServerError, "search failed")
		return
	}
	WriteJSON(w, http.StatusOK, map[string]any{"decks": decks, "query": q})
}

// GET /api/decks/{id}
func (h *DeckHandler) Get(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		WriteError(w, http.StatusBadRequest, "invalid deck id")
		return
	}

	userId := GetCurrentUserID(r, h.AuthModel)
	deck, err := h.DeckModel.GetById(id, userId)
	if err != nil {
		WriteError(w, http.StatusNotFound, "deck not found")
		return
	}
	// Increment visit count for non-owner public decks
	isOwner := userId > 0 && userId == deck.AuthorId
	if !isOwner && deck.IsPublic {
		h.DeckModel.IncrementVisitCount(id)
	}

	author, _ := h.UserModel.GetById(deck.AuthorId)

	resp := map[string]any{
		"deck":        deck,
		"author_name": author.Name,
		"is_owner":    isOwner,
	}

	if userId > 0 {
		resp["is_bookmarked"] = h.BookmarkModel.IsBookmarked(userId, id)
		resp["user_mastery"] = h.DeckModel.GetUserMastery(userId, id)
		if !isOwner {
			resp["can_rate"] = h.RatingModel.CanRate(userId, id)
			resp["user_rating"] = h.RatingModel.GetUserRating(userId, id)
		}
	}

	WriteJSON(w, http.StatusOK, resp)
}

// POST /api/decks
func (h *DeckHandler) Create(w http.ResponseWriter, r *http.Request) {
	userId := middlewares.GetUserID(r.Context())

	var body struct {
		Name           string   `json:"name"`
		Description    string   `json:"description"`
		IsPublic       bool     `json:"is_public"`
		DeckType       string   `json:"deck_type"`
		Tags           []string `json:"tags"`
		HeaderImageURL *string  `json:"header_image_url"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		WriteError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if body.Name == "" {
		WriteError(w, http.StatusBadRequest, "name is required")
		return
	}

	deckType := body.DeckType
	if deckType == "" {
		deckType = "flashcards"
	}
	tags := body.Tags
	if tags == nil {
		tags = []string{}
	}
	headerImageURL := ""
	if body.HeaderImageURL != nil {
		headerImageURL = *body.HeaderImageURL
	}

	deckId, err := h.DeckModel.Create(body.Name, body.Description, userId, body.IsPublic, deckType, tags, headerImageURL)
	if err != nil {
		WriteError(w, http.StatusInternalServerError, "could not create deck")
		return
	}

	deck, _ := h.DeckModel.GetById(deckId, userId)
	WriteJSON(w, http.StatusCreated, deck)
}

// PUT /api/decks/{id}
func (h *DeckHandler) Update(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		WriteError(w, http.StatusBadRequest, "invalid deck id")
		return
	}

	userId := middlewares.GetUserID(r.Context())
	deck, err := h.DeckModel.GetById(id, userId)
	if err != nil {
		WriteError(w, http.StatusNotFound, "deck not found")
		return
	}

	if userId != deck.AuthorId {
		WriteError(w, http.StatusForbidden, "not the deck owner")
		return
	}

	var body struct {
		Name           string   `json:"name"`
		Description    string   `json:"description"`
		IsPublic       bool     `json:"is_public"`
		DeckType       string   `json:"deck_type"`
		Tags           []string `json:"tags"`
		HeaderImageURL *string  `json:"header_image_url"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		WriteError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if body.Name == "" {
		body.Name = deck.Name
	}

	deckType := body.DeckType
	if deckType == "" {
		deckType = deck.DeckType
	}
	tags := body.Tags
	if tags == nil {
		// copy existing tags to avoid sharing underlying slice
		tags = append([]string{}, []string(deck.Tags)...)
	}
	headerImageURL := deck.HeaderImageURL
	if body.HeaderImageURL != nil {
		headerImageURL = *body.HeaderImageURL
	}

	if err := h.DeckModel.Update(id, body.Name, body.Description, body.IsPublic, deckType, tags, headerImageURL); err != nil {
		WriteError(w, http.StatusInternalServerError, "could not update deck")
		return
	}

	updated, _ := h.DeckModel.GetById(id, userId)
	WriteJSON(w, http.StatusOK, updated)
}

// DELETE /api/decks/{id}
func (h *DeckHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		WriteError(w, http.StatusBadRequest, "invalid deck id")
		return
	}
	userId := middlewares.GetUserID(r.Context())
	deck, err := h.DeckModel.GetById(id, userId)
	if err != nil {
		WriteError(w, http.StatusNotFound, "deck not found")
		return
	}

	if userId != deck.AuthorId {
		WriteError(w, http.StatusForbidden, "not the deck owner")
		return
	}

	if err := h.DeckModel.Delete(id); err != nil {
		WriteError(w, http.StatusInternalServerError, "could not delete deck")
		return
	}
	WriteJSON(w, http.StatusOK, map[string]string{"message": "deck deleted"})
}

// GET /api/decks/bookmarks
func (h *DeckHandler) ListBookmarks(w http.ResponseWriter, r *http.Request) {
	userId := middlewares.GetUserID(r.Context())
	decks, err := h.DeckModel.GetBookmarkedDecks(userId)
	if err != nil {
		WriteError(w, http.StatusInternalServerError, "could not fetch bookmarks")
		return
	}
	WriteJSON(w, http.StatusOK, map[string]any{"decks": decks})
}

// POST /api/decks/{id}/bookmark
func (h *DeckHandler) Bookmark(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		WriteError(w, http.StatusBadRequest, "invalid deck id")
		return
	}
	userId := middlewares.GetUserID(r.Context())
	bookmarked, err := h.BookmarkModel.Toggle(userId, id)
	if err != nil {
		WriteError(w, http.StatusInternalServerError, "could not toggle bookmark")
		return
	}
	WriteJSON(w, http.StatusOK, map[string]bool{"bookmarked": bookmarked})
}

// POST /api/decks/{id}/rate
func (h *DeckHandler) Rate(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		WriteError(w, http.StatusBadRequest, "invalid deck id")
		return
	}

	var body struct {
		Rating int `json:"rating"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Rating < 1 || body.Rating > 5 {
		WriteError(w, http.StatusBadRequest, "rating must be 1–5")
		return
	}

	userId := middlewares.GetUserID(r.Context())
	if !h.RatingModel.CanRate(userId, id) {
		WriteError(w, http.StatusForbidden, "you cannot rate this deck")
		return
	}

	if err := h.RatingModel.Rate(userId, id, body.Rating); err != nil {
		WriteError(w, http.StatusInternalServerError, "could not save rating")
		return
	}

	deck, _ := h.DeckModel.GetById(id, userId)
	WriteJSON(w, http.StatusOK, map[string]any{
		"rating":     body.Rating,
		"avg_rating": deck.Rating,
	})
}

// currentUserID attempts to extract the logged-in user ID without requiring middleware.
// Used on public endpoints that optionally enrich the response for logged-in users.
func GetCurrentUserID(r *http.Request, authModel *models.AuthModel) int64 {
	if uid := middlewares.GetUserID(r.Context()); uid != 0 {
		return uid
	}
	if authModel == nil {
		return 0
	}
	meta, err := authModel.ExtractTokenMetadata(r)
	if err != nil || meta == nil {
		return 0
	}
	uid, err := authModel.FetchAuth(meta)
	if err != nil {
		return 0
	}
	return uid
}
