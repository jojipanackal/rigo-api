package models

import (
	"time"

	"github.com/jojipanackal/rigo-api/db"
)

type DeckRating struct {
	Id        int64     `db:"id"         json:"id"`
	UserId    int64     `db:"user_id"    json:"user_id"`
	DeckId    int64     `db:"deck_id"    json:"deck_id"`
	Rating    int       `db:"rating"     json:"rating"`
	CreatedAt time.Time `db:"created_at" json:"created_at"`
	UpdatedAt time.Time `db:"updated_at" json:"updated_at"`
}

type RatingModel struct{}

// Rate creates or updates a user's rating for a deck
func (m *RatingModel) Rate(userId, deckId int64, rating int) error {
	query := `
		INSERT INTO deck_ratings (user_id, deck_id, rating)
		VALUES ($1, $2, $3)
		ON CONFLICT (user_id, deck_id) 
		DO UPDATE SET rating = $3, updated_at = NOW()`
	_, err := db.Instance.Exec(query, userId, deckId, rating)
	if err != nil {
		return err
	}

	// Update deck's average rating
	return m.UpdateDeckRating(deckId)
}

// GetUserRating returns the user's rating for a deck, or 0 if not rated
func (m *RatingModel) GetUserRating(userId, deckId int64) int {
	var rating int
	err := db.Instance.Get(&rating,
		`SELECT rating FROM deck_ratings WHERE user_id = $1 AND deck_id = $2`,
		userId, deckId)
	if err != nil {
		return 0
	}
	return rating
}

// HasRated checks if a user has rated a specific deck
func (m *RatingModel) HasRated(userId, deckId int64) bool {
	var count int
	db.Instance.Get(&count,
		`SELECT COUNT(*) FROM deck_ratings WHERE user_id = $1 AND deck_id = $2`,
		userId, deckId)
	return count > 0
}

// CanRate checks if a user can rate a deck (must have completed it and not be the owner)
func (m *RatingModel) CanRate(userId, deckId int64) bool {
	// Check user is not the owner
	var authorId int64
	err := db.Instance.Get(&authorId, `SELECT author_id FROM decks WHERE id = $1`, deckId)
	if err != nil || authorId == userId {
		return false
	}

	// Check user has completed at least one study session for this deck
	var sessionCount int
	db.Instance.Get(&sessionCount,
		`SELECT COUNT(*) FROM study_sessions 
		 WHERE deck_id = $1 AND user_id = $2 AND completed_at IS NOT NULL`,
		deckId, userId)
	return sessionCount > 0
}

// UpdateDeckRating recalculates and updates the average rating for a deck
func (m *RatingModel) UpdateDeckRating(deckId int64) error {
	query := `
		UPDATE decks SET 
			rating = COALESCE(
				(SELECT AVG(rating)::DECIMAL(3,1) FROM deck_ratings WHERE deck_id = $1),
				0
			),
			updated_at = NOW()
		WHERE id = $1`
	_, err := db.Instance.Exec(query, deckId)
	return err
}

// GetDeckRatingCount returns the number of ratings for a deck
func (m *RatingModel) GetDeckRatingCount(deckId int64) int {
	var count int
	db.Instance.Get(&count, `SELECT COUNT(*) FROM deck_ratings WHERE deck_id = $1`, deckId)
	return count
}
