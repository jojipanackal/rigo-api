package api

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/jojipanackal/rugo/middlewares"
	"github.com/jojipanackal/rugo/models"
)

type ProfileHandler struct {
	UserModel         *models.UserModel
	DeckModel         *models.DeckModel
	AuthModel         *models.AuthModel
	SubscriptionModel *models.SubscriptionModel
	BookmarkModel     *models.BookmarkModel
	UserStatsModel    *models.UserStatsModel
}

// GET /api/users/{id}  — public profile
func (h *ProfileHandler) GetUser(w http.ResponseWriter, r *http.Request) {
	userId, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		WriteError(w, http.StatusBadRequest, "invalid user id")
		return
	}

	user, err := h.UserModel.GetById(userId)
	if err != nil {
		WriteError(w, http.StatusNotFound, "user not found")
		return
	}

	currentUser := GetCurrentUserID(r, h.AuthModel)
	decks, _ := h.DeckModel.GetByAuthor(userId, currentUser, false) // public only

	stats, _ := h.UserStatsModel.Get(userId)

	resp := map[string]any{
		"user":  user,
		"decks": decks,
		"stats": stats,
	}

	// If the requester is logged in, include subscription status
	currentUser := middlewares.GetUserID(r.Context())
	if currentUser > 0 && currentUser != userId {
		resp["is_subscribed"] = h.SubscriptionModel.IsSubscribed(currentUser, userId)
	}

	WriteJSON(w, http.StatusOK, resp)
}

// GET /api/users/me  — own profile + private data (protected)
func (h *ProfileHandler) GetMe(w http.ResponseWriter, r *http.Request) {
	userId := middlewares.GetUserID(r.Context())

	user, err := h.UserModel.GetById(userId)
	if err != nil {
		WriteError(w, http.StatusNotFound, "user not found")
		return
	}

	stats, _ := h.UserStatsModel.Get(userId)
	myDecks, _ := h.DeckModel.GetByAuthor(userId, userId, true) // include private; viewer is same as author
	bookmarks, _ := h.BookmarkModel.GetBookmarkedDecks(userId)
	inProgress, _ := h.BookmarkModel.GetInProgressDecks(userId)

	WriteJSON(w, http.StatusOK, map[string]any{
		"user":        user,
		"stats":       stats,
		"decks":       myDecks,
		"bookmarks":   bookmarks,
		"in_progress": inProgress,
	})
}

// PUT /api/users/me  — update own profile (protected)
func (h *ProfileHandler) UpdateMe(w http.ResponseWriter, r *http.Request) {
	userId := middlewares.GetUserID(r.Context())

	var body struct {
		Name string `json:"name"`
		Bio  string `json:"bio"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		WriteError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	// Fall back to existing values if fields are empty
	user, err := h.UserModel.GetById(userId)
	if err != nil {
		WriteError(w, http.StatusNotFound, "user not found")
		return
	}
	if body.Name == "" {
		body.Name = user.Name
	}

	if err := h.UserModel.UpdateProfile(userId, body.Name, body.Bio); err != nil {
		WriteError(w, http.StatusInternalServerError, "could not update profile")
		return
	}

	updated, _ := h.UserModel.GetById(userId)
	WriteJSON(w, http.StatusOK, updated)
}

// POST /api/users/{id}/subscribe  — toggle follow (protected)
func (h *ProfileHandler) Subscribe(w http.ResponseWriter, r *http.Request) {
	creatorId, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		WriteError(w, http.StatusBadRequest, "invalid user id")
		return
	}

	subscriberId := middlewares.GetUserID(r.Context())
	if subscriberId == creatorId {
		WriteError(w, http.StatusBadRequest, "cannot subscribe to yourself")
		return
	}

	subscribed, err := h.SubscriptionModel.Toggle(subscriberId, creatorId)
	if err != nil {
		WriteError(w, http.StatusInternalServerError, "could not toggle subscription")
		return
	}
	WriteJSON(w, http.StatusOK, map[string]bool{"subscribed": subscribed})
}
