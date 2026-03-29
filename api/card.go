package api

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/jojipanackal/rugo/middlewares"
	"github.com/jojipanackal/rugo/models"
)

type CardHandler struct {
	CardModel *models.CardModel
	DeckModel *models.DeckModel
}

// GET /api/decks/{id}/cards
func (h *CardHandler) List(w http.ResponseWriter, r *http.Request) {
	deckId, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		WriteError(w, http.StatusBadRequest, "invalid deck id")
		return
	}

	cards, err := h.CardModel.GetByDeckId(deckId)
	if err != nil {
		WriteError(w, http.StatusInternalServerError, "could not fetch cards")
		return
	}
	WriteJSON(w, http.StatusOK, map[string]any{"cards": cards})
}

// POST /api/decks/{id}/cards
func (h *CardHandler) Create(w http.ResponseWriter, r *http.Request) {
	deckId, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		WriteError(w, http.StatusBadRequest, "invalid deck id")
		return
	}

	deck, err := h.DeckModel.GetById(deckId)
	if err != nil {
		WriteError(w, http.StatusNotFound, "deck not found")
		return
	}

	userId := middlewares.GetUserID(r.Context())
	if userId != deck.AuthorId {
		WriteError(w, http.StatusForbidden, "not the deck owner")
		return
	}

	var body struct {
		Front string `json:"front"`
		Back  string `json:"back"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		WriteError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if body.Front == "" || body.Back == "" {
		WriteError(w, http.StatusBadRequest, "front and back are required")
		return
	}

	cardId, err := h.CardModel.Create(deckId, body.Front, body.Back)
	if err != nil {
		WriteError(w, http.StatusInternalServerError, "could not create card")
		return
	}
	card, _ := h.CardModel.GetById(cardId)
	WriteJSON(w, http.StatusCreated, card)
}

// PUT /api/cards/{id}
func (h *CardHandler) Update(w http.ResponseWriter, r *http.Request) {
	cardId, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		WriteError(w, http.StatusBadRequest, "invalid card id")
		return
	}

	card, err := h.CardModel.GetById(cardId)
	if err != nil {
		WriteError(w, http.StatusNotFound, "card not found")
		return
	}

	deck, err := h.DeckModel.GetById(card.DeckId)
	if err != nil {
		WriteError(w, http.StatusInternalServerError, "could not verify ownership")
		return
	}

	userId := middlewares.GetUserID(r.Context())
	if userId != deck.AuthorId {
		WriteError(w, http.StatusForbidden, "not the deck owner")
		return
	}

	var body struct {
		Front string `json:"front"`
		Back  string `json:"back"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		WriteError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if body.Front == "" {
		body.Front = card.Front
	}
	if body.Back == "" {
		body.Back = card.Back
	}

	if err := h.CardModel.Update(cardId, body.Front, body.Back); err != nil {
		WriteError(w, http.StatusInternalServerError, "could not update card")
		return
	}

	updated, _ := h.CardModel.GetById(cardId)
	WriteJSON(w, http.StatusOK, updated)
}

// DELETE /api/cards/{id}
func (h *CardHandler) Delete(w http.ResponseWriter, r *http.Request) {
	cardId, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		WriteError(w, http.StatusBadRequest, "invalid card id")
		return
	}

	card, err := h.CardModel.GetById(cardId)
	if err != nil {
		WriteError(w, http.StatusNotFound, "card not found")
		return
	}

	deck, err := h.DeckModel.GetById(card.DeckId)
	if err != nil {
		WriteError(w, http.StatusInternalServerError, "could not verify ownership")
		return
	}

	userId := middlewares.GetUserID(r.Context())
	if userId != deck.AuthorId {
		WriteError(w, http.StatusForbidden, "not the deck owner")
		return
	}

	if err := h.CardModel.Delete(cardId); err != nil {
		WriteError(w, http.StatusInternalServerError, "could not delete card")
		return
	}
	WriteJSON(w, http.StatusOK, map[string]string{"message": "card deleted"})
}

// POST /api/decks/{id}/cards/import  — CSV: one card per line, "front,back"
func (h *CardHandler) ImportCSV(w http.ResponseWriter, r *http.Request) {
	deckId, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		WriteError(w, http.StatusBadRequest, "invalid deck id")
		return
	}

	deck, err := h.DeckModel.GetById(deckId)
	if err != nil {
		WriteError(w, http.StatusNotFound, "deck not found")
		return
	}

	userId := middlewares.GetUserID(r.Context())
	if userId != deck.AuthorId {
		WriteError(w, http.StatusForbidden, "not the deck owner")
		return
	}

	var body struct {
		Cards []struct {
			Front string `json:"front"`
			Back  string `json:"back"`
		} `json:"cards"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		WriteError(w, http.StatusBadRequest, "expected {cards: [{front, back}]}")
		return
	}

	created := 0
	for _, c := range body.Cards {
		if c.Front == "" || c.Back == "" {
			continue
		}
		if _, err := h.CardModel.Create(deckId, c.Front, c.Back); err == nil {
			created++
		}
	}
	WriteJSON(w, http.StatusCreated, map[string]int{"created": created})
}
