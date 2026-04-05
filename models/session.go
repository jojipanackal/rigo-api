package models

import (
	"time"

	"github.com/jojipanackal/rigo-api/db"
)

type StudySession struct {
	Id           int64      `db:"id"            json:"id"`
	DeckId       int64      `db:"deck_id"       json:"deck_id"`
	UserId       int64      `db:"user_id"       json:"-"`
	CardsStudied int        `db:"cards_studied" json:"cards_studied"`
	CorrectCount int        `db:"correct_count" json:"correct_count"`
	StartedAt    time.Time  `db:"started_at"    json:"started_at"`
	CompletedAt  *time.Time `db:"completed_at"  json:"completed_at"`
}

type SessionModel struct {
	UserStatsModel *UserStatsModel
}

// Create starts a new study session
func (m *SessionModel) Create(deckId, userId int64) (int64, error) {
	var id int64

	query := `
		INSERT INTO study_sessions (deck_id, user_id)
		VALUES ($1, $2)
		RETURNING id`

	err := db.Instance.QueryRow(query, deckId, userId).Scan(&id)
	if err != nil {
		return 0, err
	}

	return id, nil
}

// GetById retrieves a session by ID
func (m *SessionModel) GetById(id int64) (StudySession, error) {
	var s StudySession
	err := db.Instance.Get(&s, "SELECT * FROM study_sessions WHERE id = $1", id)
	return s, err
}

// RecordAnswer updates session stats and logs the individual card answer to session_cards.
func (m *SessionModel) RecordAnswer(sessionId, cardId int64, rating int, correct bool) error {
	correctIncrement := 0
	if correct {
		correctIncrement = 1
	}

	// Update session aggregate counts
	_, err := db.Instance.Exec(`
		UPDATE study_sessions 
		SET cards_studied = cards_studied + 1,
		    correct_count = correct_count + $1
		WHERE id = $2`, correctIncrement, sessionId)
	if err != nil {
		return err
	}

	// Insert individual card response for per-session replay / leaderboards
	_, err = db.Instance.Exec(`
		INSERT INTO session_cards (session_id, card_id, rating)
		VALUES ($1, $2, $3)`, sessionId, cardId, rating)
	return err
}

// Complete marks a session as finished and updates user_stats.
func (m *SessionModel) Complete(sessionId int64) error {
	_, err := db.Instance.Exec(
		`UPDATE study_sessions SET completed_at = NOW() WHERE id = $1`, sessionId)
	if err != nil {
		return err
	}

	// Fetch session stats to pass to user_stats update
	session, err := m.GetById(sessionId)
	if err != nil {
		return err
	}

	// Update cached user stats (streak, XP, totals)
	if m.UserStatsModel != nil {
		return m.UserStatsModel.UpdateAfterSession(
			session.UserId, session.CardsStudied, session.CorrectCount)
	}
	return nil
}

// GetUserSessions retrieves recent sessions for a user
func (m *SessionModel) GetUserSessions(userId int64, limit int) ([]StudySession, error) {
	var sessions []StudySession
	query := `SELECT * FROM study_sessions WHERE user_id = $1 ORDER BY started_at DESC LIMIT $2`
	err := db.Instance.Select(&sessions, query, userId, limit)
	return sessions, err
}

// GetActiveSession finds an incomplete session for a user+deck (to resume)
func (m *SessionModel) GetActiveSession(deckId, userId int64) (*StudySession, error) {
	var s StudySession
	query := `
		SELECT * FROM study_sessions 
		WHERE deck_id = $1 AND user_id = $2 AND completed_at IS NULL
		ORDER BY started_at DESC
		LIMIT 1`
	err := db.Instance.Get(&s, query, deckId, userId)
	if err != nil {
		return nil, err
	}
	return &s, nil
}

// GetLastCompletedSession finds the most recent completed session for a user+deck
func (m *SessionModel) GetLastCompletedSession(deckId, userId int64) (*StudySession, error) {
	var s StudySession
	query := `
		SELECT * FROM study_sessions 
		WHERE deck_id = $1 AND user_id = $2 AND completed_at IS NOT NULL
		ORDER BY completed_at DESC
		LIMIT 1`
	err := db.Instance.Get(&s, query, deckId, userId)
	if err != nil {
		return nil, err
	}
	return &s, nil
}
// DeleteSessions removes all study sessions for a specific user and deck.
func (m *SessionModel) DeleteSessions(userId, deckId int64) error {
	_, err := db.Instance.Exec(
		`DELETE FROM study_sessions WHERE user_id = $1 AND deck_id = $2`,
		userId, deckId)
	return err
}
