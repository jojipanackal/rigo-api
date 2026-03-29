package models

import (
	"time"

	"github.com/jojipanackal/rugo/db"
)

type Bookmark struct {
	UserId    int64     `db:"user_id"    json:"user_id"`
	DeckId    int64     `db:"deck_id"    json:"deck_id"`
	CreatedAt time.Time `db:"created_at" json:"created_at"`
}

// DeckWithMastery extends Deck with per-user mastery percentage
type DeckWithMastery struct {
	Deck
	MasteryPercent int64 `db:"mastery_percent" json:"mastery_percent"`
}

type BookmarkModel struct{}

// Toggle adds or removes a bookmark
func (m *BookmarkModel) Toggle(userId, deckId int64) (bool, error) {
	if m.IsBookmarked(userId, deckId) {
		// Remove bookmark
		_, err := db.Instance.Exec(
			`DELETE FROM bookmarks WHERE user_id = $1 AND deck_id = $2`,
			userId, deckId)
		return false, err
	}
	// Add bookmark
	_, err := db.Instance.Exec(
		`INSERT INTO bookmarks (user_id, deck_id) VALUES ($1, $2) ON CONFLICT DO NOTHING`,
		userId, deckId)
	return true, err
}

// Add creates a bookmark (for auto-bookmarking)
func (m *BookmarkModel) Add(userId, deckId int64) error {
	_, err := db.Instance.Exec(
		`INSERT INTO bookmarks (user_id, deck_id) VALUES ($1, $2) ON CONFLICT DO NOTHING`,
		userId, deckId)
	return err
}

// IsBookmarked checks if a user has bookmarked a deck
func (m *BookmarkModel) IsBookmarked(userId, deckId int64) bool {
	var count int
	db.Instance.Get(&count,
		`SELECT COUNT(*) FROM bookmarks WHERE user_id = $1 AND deck_id = $2`,
		userId, deckId)
	return count > 0
}

// GetBookmarkedDecks returns all decks a user has bookmarked with their mastery
func (m *BookmarkModel) GetBookmarkedDecks(userId int64) ([]DeckWithMastery, error) {
	var decks []DeckWithMastery
	query := `
		SELECT d.*, COALESCE(m.mastery_percent, 0) as mastery_percent FROM decks d
		JOIN bookmarks b ON b.deck_id = d.id
		LEFT JOIN user_deck_mastery m ON m.deck_id = d.id AND m.user_id = $1
		WHERE b.user_id = $1
		ORDER BY b.created_at DESC`
	err := db.Instance.Select(&decks, query, userId)
	return decks, err
}

// GetInProgressDecks returns bookmarked decks with sessions and mastery < 100
func (m *BookmarkModel) GetInProgressDecks(userId int64) ([]DeckWithMastery, error) {
	var decks []DeckWithMastery
	query := `
		SELECT d.*, COALESCE(m.mastery_percent, 0) as mastery_percent FROM decks d
		JOIN bookmarks b ON b.deck_id = d.id
		LEFT JOIN user_deck_mastery m ON m.deck_id = d.id AND m.user_id = $1
		WHERE b.user_id = $1 
		AND COALESCE(m.mastery_percent, 0) < 100
		AND EXISTS (SELECT 1 FROM study_sessions s WHERE s.deck_id = d.id AND s.user_id = $1)
		ORDER BY (SELECT MAX(started_at) FROM study_sessions WHERE deck_id = d.id AND user_id = $1) DESC`
	err := db.Instance.Select(&decks, query, userId)
	return decks, err
}

// GetNotStartedDecks returns bookmarked decks with no study sessions
func (m *BookmarkModel) GetNotStartedDecks(userId int64) ([]DeckWithMastery, error) {
	var decks []DeckWithMastery
	query := `
		SELECT d.*, 0 as mastery_percent FROM decks d
		JOIN bookmarks b ON b.deck_id = d.id
		WHERE b.user_id = $1 
		AND NOT EXISTS (SELECT 1 FROM study_sessions s WHERE s.deck_id = d.id AND s.user_id = $1)
		ORDER BY b.created_at DESC`
	err := db.Instance.Select(&decks, query, userId)
	return decks, err
}

// GetMasteredDecks returns bookmarked decks with per-user mastery = 100
func (m *BookmarkModel) GetMasteredDecks(userId int64) ([]DeckWithMastery, error) {
	var decks []DeckWithMastery
	query := `
		SELECT d.*, m.mastery_percent FROM decks d
		JOIN bookmarks b ON b.deck_id = d.id
		JOIN user_deck_mastery m ON m.deck_id = d.id AND m.user_id = $1
		WHERE b.user_id = $1 AND m.mastery_percent = 100
		ORDER BY b.created_at DESC`
	err := db.Instance.Select(&decks, query, userId)
	return decks, err
}
