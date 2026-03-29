package models

import (
	"time"

	"github.com/jojipanackal/rugo/db"
)

type Card struct {
	Id        int64     `db:"id"         json:"id"`
	DeckId    int64     `db:"deck_id"    json:"deck_id"`
	Front     string    `db:"front"      json:"front"`
	Back      string    `db:"back"       json:"back"`
	CreatedAt time.Time `db:"created_at" json:"created_at"`
	UpdatedAt time.Time `db:"updated_at" json:"updated_at"`
}

type CardModel struct{}

func (m *CardModel) Create(deckId int64, front, back string) (int64, error) {
	var lastInsertId int64

	query := `
		INSERT INTO cards (deck_id, front, back)
		VALUES ($1, $2, $3)
		RETURNING id`

	err := db.Instance.QueryRow(query, deckId, front, back).Scan(&lastInsertId)
	if err != nil {
		return 0, err
	}

	// Update deck card count
	_, err = db.Instance.Exec(`UPDATE decks SET cards_count = cards_count + 1 WHERE id = $1`, deckId)
	if err != nil {
		return lastInsertId, err
	}

	return lastInsertId, nil
}

func (m *CardModel) GetById(id int64) (Card, error) {
	var c Card
	err := db.Instance.Get(&c, "SELECT * FROM cards WHERE id = $1", id)
	return c, err
}

func (m *CardModel) GetByDeckId(deckId int64) ([]Card, error) {
	var cards []Card
	err := db.Instance.Select(&cards, "SELECT * FROM cards WHERE deck_id = $1 ORDER BY created_at", deckId)
	return cards, err
}

func (m *CardModel) Update(id int64, front, back string) error {
	query := `UPDATE cards SET front = $1, back = $2, updated_at = NOW() WHERE id = $3`
	_, err := db.Instance.Exec(query, front, back, id)
	return err
}

func (m *CardModel) Delete(id int64) error {
	// Get deck_id first to update count
	var deckId int64
	err := db.Instance.Get(&deckId, "SELECT deck_id FROM cards WHERE id = $1", id)
	if err != nil {
		return err
	}

	_, err = db.Instance.Exec("DELETE FROM cards WHERE id = $1", id)
	if err != nil {
		return err
	}

	// Update deck card count
	_, err = db.Instance.Exec(`UPDATE decks SET cards_count = cards_count - 1 WHERE id = $1`, deckId)
	return err
}

// GetCardIDsForDeck returns all card IDs in a deck, ordered for consistent study ordering.
func (m *CardModel) GetCardIDsForDeck(deckId int64) ([]int64, error) {
	var ids []int64
	err := db.Instance.Select(&ids, "SELECT id FROM cards WHERE deck_id = $1 ORDER BY id", deckId)
	return ids, err
}
