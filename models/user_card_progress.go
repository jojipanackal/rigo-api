package models

import (
	"time"

	"github.com/jojipanackal/rigo-api/db"
)

// UserCardProgress holds per-user FSRS state for a single card.
// This is the fix for the "shared FSRS state" bug — each (user, card) pair
// has its own independent review schedule.
type UserCardProgress struct {
	UserId         int64      `db:"user_id"          json:"-"`
	CardId         int64      `db:"card_id"          json:"card_id"`
	TimesReviewed  int        `db:"times_reviewed"   json:"times_reviewed"`
	TimesCorrect   int        `db:"times_correct"    json:"times_correct"`
	LastReviewed   *time.Time `db:"last_reviewed"    json:"last_reviewed"`
	NextReview     *time.Time `db:"next_review"      json:"next_review"`
	Stability      float64    `db:"stability"        json:"stability"`
	FSRSDifficulty float64    `db:"fsrs_difficulty"  json:"fsrs_difficulty"`
	Reps           int        `db:"reps"             json:"reps"`
	Lapses         int        `db:"lapses"           json:"lapses"`
	// State: 0=New, 1=Learning, 2=Review, 3=Relearning
	State int `db:"state" json:"state"`
}

type UserCardProgressModel struct{}

// GetOrCreate fetches a (user, card) progress row, or returns a fresh default if none exists.
func (m *UserCardProgressModel) GetOrCreate(userId, cardId int64) (UserCardProgress, error) {
	var ucp UserCardProgress
	err := db.Instance.Get(&ucp,
		`SELECT * FROM user_card_progress WHERE user_id = $1 AND card_id = $2`,
		userId, cardId)
	if err != nil {
		// Row doesn't exist yet — return zero-value defaults
		return UserCardProgress{
			UserId:         userId,
			CardId:         cardId,
			FSRSDifficulty: 0.3,
			State:          StateNew,
		}, nil
	}
	return ucp, nil
}

// RecordFSRSReview processes a card review using FSRS and writes the result
// to user_card_progress (not the cards table).
func (m *UserCardProgressModel) RecordFSRSReview(userId, cardId int64, rating int) error {
	ucp, err := m.GetOrCreate(userId, cardId)
	if err != nil {
		return err
	}

	now := time.Now()
	fsrsCard := FSRSCard{
		Stability:  ucp.Stability,
		Difficulty: ucp.FSRSDifficulty,
		Reps:       ucp.Reps,
		Lapses:     ucp.Lapses,
		State:      ucp.State,
	}
	if ucp.LastReviewed != nil {
		fsrsCard.LastReview = *ucp.LastReviewed
	}
	if fsrsCard.Difficulty == 0 {
		fsrsCard.Difficulty = 0.3
	}

	fsrs := &FSRS{}
	newState, nextReview := fsrs.Review(fsrsCard, rating, now)

	correctIncrement := 0
	if rating >= RatingGood {
		correctIncrement = 1
	}

	query := `
		INSERT INTO user_card_progress 
			(user_id, card_id, times_reviewed, times_correct, last_reviewed, next_review,
			 stability, fsrs_difficulty, reps, lapses, state)
		VALUES ($1, $2, 1, $3, $4, $5, $6, $7, $8, $9, $10)
		ON CONFLICT (user_id, card_id) DO UPDATE SET
			times_reviewed  = user_card_progress.times_reviewed + 1,
			times_correct   = user_card_progress.times_correct + $3,
			last_reviewed   = $4,
			next_review     = $5,
			stability       = $6,
			fsrs_difficulty = $7,
			reps            = $8,
			lapses          = $9,
			state           = $10`

	_, err = db.Instance.Exec(query,
		userId, cardId,
		correctIncrement,
		now,
		nextReview,
		newState.Stability,
		newState.Difficulty,
		newState.Reps,
		newState.Lapses,
		newState.State,
	)
	return err
}

// GetDueCardIDs returns card IDs in the deck that are due for review for this user.
// New cards (no UCP row) and cards where next_review <= now are included.
func (m *UserCardProgressModel) GetDueCardIDs(userId, deckId int64) ([]int64, error) {
	var ids []int64
	query := `
		SELECT c.id FROM cards c
		LEFT JOIN user_card_progress ucp ON ucp.card_id = c.id AND ucp.user_id = $1
		WHERE c.deck_id = $2
		AND (ucp.next_review IS NULL OR ucp.next_review <= NOW())
		ORDER BY
			CASE WHEN ucp.state IS NULL THEN 0 ELSE 1 END,  -- New cards first
			ucp.next_review NULLS FIRST`
	err := db.Instance.Select(&ids, query, userId, deckId)
	return ids, err
}

// GetProgressForDeck returns all UCP rows for a user's cards in a deck
func (m *UserCardProgressModel) GetProgressForDeck(userId, deckId int64) ([]UserCardProgress, error) {
	var rows []UserCardProgress
	query := `
		SELECT ucp.* FROM user_card_progress ucp
		JOIN cards c ON c.id = ucp.card_id
		WHERE ucp.user_id = $1 AND c.deck_id = $2`
	err := db.Instance.Select(&rows, query, userId, deckId)
	return rows, err
}
