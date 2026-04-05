package api

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/jojipanackal/rigo-api/middlewares"
	"github.com/jojipanackal/rigo-api/models"
)

type SessionHandler struct {
	SessionModel          *models.SessionModel
	CardModel             *models.CardModel
	DeckModel             *models.DeckModel
	UserCardProgressModel *models.UserCardProgressModel
	BookmarkModel         *models.BookmarkModel
}

// POST /api/decks/{id}/study
// Starts a fresh study session. Returns the session_id and the full card list.
// The frontend holds the queue and presents cards; each call to /answer records the result.
func (h *SessionHandler) Start(w http.ResponseWriter, r *http.Request) {
	deckId, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		WriteError(w, http.StatusBadRequest, "invalid deck id")
		return
	}

	userId := middlewares.GetUserID(r.Context())
	deck, err := h.DeckModel.GetById(deckId, userId)
	if err != nil {
		WriteError(w, http.StatusNotFound, "deck not found")
		return
	}

	cards, err := h.CardModel.GetByDeckId(deckId)
	if err != nil {
		WriteError(w, http.StatusInternalServerError, "could not fetch cards")
		return
	}
	if len(cards) == 0 {
		WriteError(w, http.StatusUnprocessableEntity, "deck has no cards")
		return
	}

	// Count how many cards are actually due for this user
	dueIds, _ := h.UserCardProgressModel.GetDueCardIDs(userId, deckId)
	dueSet := map[int64]bool{}
	for _, id := range dueIds {
		dueSet[id] = true
	}

	sessionId, err := h.SessionModel.Create(deckId, userId)
	if err != nil {
		WriteError(w, http.StatusInternalServerError, "could not create session")
		return
	}

	// Auto-bookmark — user started studying this deck
	if h.BookmarkModel != nil {
		h.BookmarkModel.Add(userId, deckId)
	}

	WriteJSON(w, http.StatusCreated, map[string]any{
		"session_id": sessionId,
		"deck":       deck,
		"cards":      cards,          // full list; frontend manages queue order
		"due_count":  len(dueIds),    // cards actually due vs. all cards
		"total":      len(cards),
	})
}

// POST /api/sessions/{id}/answer
// Records the user's FSRS rating for one card.
// Body: {card_id, rating}  (rating: 1=Again, 2=Hard, 3=Good, 4=Easy)
// Returns: {completed, cards_studied, correct_count, mastery_percent} plus session_stats when done.
func (h *SessionHandler) Answer(w http.ResponseWriter, r *http.Request) {
	sessionId, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		WriteError(w, http.StatusBadRequest, "invalid session id")
		return
	}

	var body struct {
		CardId    int64 `json:"card_id"`
		Rating    int   `json:"rating"`
		Completed bool  `json:"completed"` // true on the last card
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		WriteError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if body.Rating < 1 || body.Rating > 4 {
		WriteError(w, http.StatusBadRequest, "rating must be 1–4")
		return
	}

	userId := middlewares.GetUserID(r.Context())

	// Record FSRS progress for this (user, card) pair
	if err := h.UserCardProgressModel.RecordFSRSReview(userId, body.CardId, body.Rating); err != nil {
		WriteError(w, http.StatusInternalServerError, "could not record review")
		return
	}

	correct := body.Rating >= 3
	if err := h.SessionModel.RecordAnswer(sessionId, body.CardId, body.Rating, correct); err != nil {
		WriteError(w, http.StatusInternalServerError, "could not record answer")
		return
	}

	session, _ := h.SessionModel.GetById(sessionId)

	// Update deck mastery for this user
	h.DeckModel.UpdateUserMastery(userId, session.DeckId)
	mastery := h.DeckModel.GetUserMastery(userId, session.DeckId)

	resp := map[string]any{
		"session_id":    sessionId,
		"cards_studied": session.CardsStudied,
		"correct_count": session.CorrectCount,
		"mastery":       mastery,
		"completed":     false,
	}

	// If the frontend signals this was the last card, complete the session
	if body.Completed {
		h.SessionModel.Complete(sessionId)
		resp["completed"] = true
		resp["session"] = session
	}

	WriteJSON(w, http.StatusOK, resp)
}
// DELETE /api/decks/{id}/study
func (h *SessionHandler) Stop(w http.ResponseWriter, r *http.Request) {
	deckId, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		WriteError(w, http.StatusBadRequest, "invalid deck id")
		return
	}

	userId := middlewares.GetUserID(r.Context())
	if err := h.SessionModel.DeleteSessions(userId, deckId); err != nil {
		WriteError(w, http.StatusInternalServerError, "could not stop studying")
		return
	}

	WriteJSON(w, http.StatusOK, map[string]string{"message": "deck removed from active study list"})
}
