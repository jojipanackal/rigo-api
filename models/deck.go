package models

import (
	"time"

	"github.com/jojipanackal/rugo/db"
	"github.com/lib/pq"
)

type Deck struct {
	Id             int64          `db:"id"          json:"id"`
	Name           string         `db:"name"        json:"name"`
	Description    string         `db:"description" json:"description"`
	AuthorId       int64          `db:"author_id"   json:"author_id"`
	CardsCount     int64          `db:"cards_count" json:"cards_count"`
	Rating         float64        `db:"rating"      json:"rating"`
	VisitCount     int64          `db:"visit_count" json:"visit_count"`
	DeckType       string         `db:"deck_type"   json:"deck_type"`
	Tags           pq.StringArray `db:"tags"        json:"tags"`
	HeaderImageURL string         `db:"header_image_url" json:"header_image_url"`
	IsPublic       bool           `db:"is_public"   json:"is_public"`
	HasActiveSession bool           `db:"has_active_session" json:"has_active_session"`
	CreatedAt      time.Time      `db:"created_at"  json:"created_at"`
	UpdatedAt      time.Time      `db:"updated_at"  json:"updated_at"`
}

type DeckModel struct{}

func (m *DeckModel) Create(name, description string, authorId int64, isPublic bool, deckType string, tags []string, headerImageURL string) (int64, error) {
	var lastInsertId int64

	query := `
        INSERT INTO decks (name, description, author_id, is_public, deck_type, tags, header_image_url)
        VALUES ($1, $2, $3, $4, $5, $6, $7)
        RETURNING id`

	if deckType == "" {
		deckType = "flashcards"
	}

	err := db.Instance.QueryRow(query, name, description, authorId, isPublic, deckType, pq.StringArray(tags), headerImageURL).Scan(&lastInsertId)
	if err != nil {
		return 0, err
	}

	return lastInsertId, nil
}

func (m *DeckModel) GetById(id int64, userId int64) (Deck, error) {
	var d Deck
	query := `SELECT id, name, COALESCE(description, '') as description, COALESCE(author_id, 0) as author_id, 
		cards_count, COALESCE(rating, 0) as rating, visit_count, deck_type, tags, 
		COALESCE(header_image_url, '') as header_image_url, is_public, 
		EXISTS (SELECT 1 FROM study_sessions WHERE user_id = $2 AND deck_id = $1 AND completed_at IS NULL) as has_active_session,
		created_at, updated_at 
		FROM decks WHERE id = $1`
	err := db.Instance.Get(&d, query, id, userId)
	return d, err
}

func (m *DeckModel) Update(id int64, name, description string, isPublic bool, deckType string, tags []string, headerImageURL string) error {
	if deckType == "" {
		deckType = "flashcards"
	}

	query := `UPDATE decks 
		SET name = $1, description = $2, is_public = $3, deck_type = $4, tags = $5, header_image_url = $6, updated_at = NOW() 
		WHERE id = $7`
	_, err := db.Instance.Exec(query, name, description, isPublic, deckType, pq.StringArray(tags), headerImageURL, id)
	return err
}

// UpdateHeaderImageURL sets or clears the stored header image URL for a deck.
func (m *DeckModel) UpdateHeaderImageURL(id int64, headerImageURL string) error {
	_, err := db.Instance.Exec(`UPDATE decks SET header_image_url = $1, updated_at = NOW() WHERE id = $2`, headerImageURL, id)
	return err
}

func (m *DeckModel) Delete(id int64) error {
	_, err := db.Instance.Exec("DELETE FROM decks WHERE id = $1", id)
	return err
}

// GetAll returns paginated list of decks, always showing public decks and optionally
// including the authenticated user's private decks.
func (m *DeckModel) GetAll(limit, offset int, userId int64) ([]Deck, error) {
	var decks []Deck
	query := `
		SELECT id, name, COALESCE(description, '') as description, COALESCE(author_id, 0) as author_id, 
		cards_count, COALESCE(rating, 0) as rating, visit_count, deck_type, tags, 
		COALESCE(header_image_url, '') as header_image_url, is_public, 
		EXISTS (SELECT 1 FROM study_sessions WHERE user_id = $3 AND deck_id = id AND completed_at IS NULL) as has_active_session,
		created_at, updated_at 
		FROM decks 
		WHERE is_public = true 
		OR (author_id = $3 AND $3 <> 0)
		ORDER BY created_at DESC 
		LIMIT $1 OFFSET $2`
	err := db.Instance.Select(&decks, query, limit, offset, userId)
	return decks, err
}

// GetByAuthor returns all decks by a specific user (all if owner, public otherwise)
func (m *DeckModel) GetByAuthor(authorId int64, viewerId int64, includePrivate bool) ([]Deck, error) {
	var decks []Deck
	var query string
	columns := `id, name, COALESCE(description, '') as description, COALESCE(author_id, 0) as author_id, 
		cards_count, COALESCE(rating, 0) as rating, visit_count, deck_type, tags, 
		COALESCE(header_image_url, '') as header_image_url, is_public, 
		EXISTS (SELECT 1 FROM study_sessions WHERE user_id = $2 AND deck_id = id AND completed_at IS NULL) as has_active_session,
		created_at, updated_at`
	if includePrivate {
		query = "SELECT " + columns + " FROM decks WHERE author_id = $1 ORDER BY created_at DESC"
	} else {
		query = "SELECT " + columns + " FROM decks WHERE author_id = $1 AND is_public = true ORDER BY created_at DESC"
	}
	err := db.Instance.Select(&decks, query, authorId, viewerId)
	return decks, err
}

// GetPopular returns decks ordered by visit count
func (m *DeckModel) GetPopular(limit, offset int, userId int64) ([]Deck, error) {
	var decks []Deck
	query := `
		SELECT id, name, COALESCE(description, '') as description, COALESCE(author_id, 0) as author_id, 
		cards_count, COALESCE(rating, 0) as rating, visit_count, deck_type, tags, 
		COALESCE(header_image_url, '') as header_image_url, is_public, 
		EXISTS (SELECT 1 FROM study_sessions WHERE user_id = $3 AND deck_id = id AND completed_at IS NULL) as has_active_session,
		created_at, updated_at 
		FROM decks 
		WHERE is_public = true 
		ORDER BY visit_count DESC, rating DESC 
		LIMIT $1 OFFSET $2`
	err := db.Instance.Select(&decks, query, limit, offset, userId)
	return decks, err
}

// GetPopularCount returns total count of public decks
func (m *DeckModel) GetPopularCount() int {
	var count int
	db.Instance.Get(&count, "SELECT COUNT(*) FROM decks WHERE is_public = true")
	return count
}

// Search finds decks matching the query
func (m *DeckModel) Search(query string, limit int, userId int64) ([]Deck, error) {
	var decks []Deck
	searchQuery := `
		SELECT id, name, COALESCE(description, '') as description, COALESCE(author_id, 0) as author_id, 
		cards_count, COALESCE(rating, 0) as rating, visit_count, deck_type, tags, 
		COALESCE(header_image_url, '') as header_image_url, is_public, 
		EXISTS (SELECT 1 FROM study_sessions WHERE user_id = $3 AND deck_id = id AND completed_at IS NULL) as has_active_session,
		created_at, updated_at 
		FROM decks 
		WHERE is_public = true 
		AND (name ILIKE $1 OR description ILIKE $1)
		ORDER BY rating DESC 
		LIMIT $2`
	err := db.Instance.Select(&decks, searchQuery, "%"+query+"%", limit, userId)
	return decks, err
}

// UpdateUserMastery recalculates mastery % from user_card_progress (not cards table).
// A card is "mastered" when its state = 2 (Review state in FSRS).
func (m *DeckModel) UpdateUserMastery(userId, deckId int64) error {
	query := `
		INSERT INTO user_deck_mastery (user_id, deck_id, mastery_percent, updated_at)
		VALUES ($1, $2, 
			COALESCE(
				(SELECT 
					CASE 
						WHEN COUNT(*) = 0 THEN 0
						ELSE (COUNT(CASE WHEN ucp.state = 2 THEN 1 END) * 100 / COUNT(*))
					END
				FROM cards c
				LEFT JOIN user_card_progress ucp ON ucp.card_id = c.id AND ucp.user_id = $1
				WHERE c.deck_id = $2),
				0
			),
			NOW()
		)
		ON CONFLICT (user_id, deck_id) 
		DO UPDATE SET 
			mastery_percent = COALESCE(
				(SELECT 
					CASE 
						WHEN COUNT(*) = 0 THEN 0
						ELSE (COUNT(CASE WHEN ucp.state = 2 THEN 1 END) * 100 / COUNT(*))
					END
				FROM cards c
				LEFT JOIN user_card_progress ucp ON ucp.card_id = c.id AND ucp.user_id = $1
				WHERE c.deck_id = $2),
				0
			),
			updated_at = NOW()`
	_, err := db.Instance.Exec(query, userId, deckId)
	return err
}

// GetUserMastery returns the mastery percentage for a specific user+deck
func (m *DeckModel) GetUserMastery(userId, deckId int64) int64 {
	var mastery int64
	err := db.Instance.Get(&mastery,
		`SELECT mastery_percent FROM user_deck_mastery WHERE user_id = $1 AND deck_id = $2`,
		userId, deckId)
	if err != nil {
		return 0
	}
	return mastery
}

// IncrementVisitCount increases the visit count for a deck
func (m *DeckModel) IncrementVisitCount(deckId int64) error {
	_, err := db.Instance.Exec(`UPDATE decks SET visit_count = visit_count + 1 WHERE id = $1`, deckId)
	return err
}

// GetDecksBeingStudied returns decks the user has active bookmarks on with session history
func (m *DeckModel) GetDecksBeingStudied(userId int64) ([]Deck, error) {
	var decks []Deck
	query := `
		SELECT DISTINCT d.id, d.name, COALESCE(d.description, '') as description, COALESCE(d.author_id, 0) as author_id, 
		d.cards_count, COALESCE(d.rating, 0) as rating, d.visit_count, d.deck_type, d.tags, 
		COALESCE(d.header_image_url, '') as header_image_url, d.is_public, 
		EXISTS (SELECT 1 FROM study_sessions WHERE user_id = $1 AND deck_id = d.id AND completed_at IS NULL) as has_active_session,
		d.created_at, d.updated_at FROM decks d
		JOIN study_sessions s ON s.deck_id = d.id
		WHERE s.user_id = $1 
		AND (d.is_public = true OR d.author_id = $1)
		ORDER BY (SELECT MAX(started_at) FROM study_sessions WHERE deck_id = d.id AND user_id = $1) DESC`
	err := db.Instance.Select(&decks, query, userId)
	return decks, err
}
func (m *DeckModel) GetBookmarkedDecks(userId int64) ([]Deck, error) {
	var decks []Deck
	query := `
		SELECT d.id, d.name, COALESCE(d.description, '') as description, COALESCE(d.author_id, 0) as author_id, 
		d.cards_count, COALESCE(d.rating, 0) as rating, d.visit_count, d.deck_type, d.tags, 
		COALESCE(d.header_image_url, '') as header_image_url, d.is_public, 
		EXISTS (SELECT 1 FROM study_sessions WHERE user_id = $1 AND deck_id = d.id AND completed_at IS NULL) as has_active_session,
		d.created_at, d.updated_at 
		FROM decks d
		JOIN bookmarks b ON b.deck_id = d.id
		WHERE b.user_id = $1
		ORDER BY b.created_at DESC`
	err := db.Instance.Select(&decks, query, userId)
	return decks, err
}
