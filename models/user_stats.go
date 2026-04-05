package models

import (
	"time"

	"github.com/jojipanackal/rigo-api/db"
)

// UserStats holds cached aggregate stats for a user.
// Updated write-through after each session completes — avoids expensive
// aggregation queries on every profile/dashboard load.
type UserStats struct {
	UserId            int64      `db:"user_id"             json:"user_id"`
	TotalCardsStudied int        `db:"total_cards_studied" json:"total_cards_studied"`
	TotalCorrect      int        `db:"total_correct"       json:"total_correct"`
	TotalSessions     int        `db:"total_sessions"      json:"total_sessions"`
	CurrentStreak     int        `db:"current_streak"      json:"current_streak"`
	LongestStreak     int        `db:"longest_streak"      json:"longest_streak"`
	LastStudyDate     *time.Time `db:"last_study_date"     json:"last_study_date"`
	XP                int        `db:"xp"                  json:"xp"`
	UpdatedAt         time.Time  `db:"updated_at"          json:"updated_at"`
}

type UserStatsModel struct{}

// Get fetches stats for a user. Returns zero-value stats if none exist yet.
func (m *UserStatsModel) Get(userId int64) (UserStats, error) {
	var s UserStats
	err := db.Instance.Get(&s,
		`SELECT * FROM user_stats WHERE user_id = $1`, userId)
	if err != nil {
		return UserStats{UserId: userId}, nil
	}
	return s, nil
}

// UpdateAfterSession increments totals, recalculates streak, and awards XP.
// Called by SessionModel.Complete after a study session finishes.
// XP formula: 10 per session + 2 per correct card.
func (m *UserStatsModel) UpdateAfterSession(userId int64, cardsStudied, correct int) error {
	xpGained := 10 + (correct * 2)
	today := time.Now().UTC().Truncate(24 * time.Hour)

	// Upsert: if no row exists (new user), create it; otherwise update.
	query := `
		INSERT INTO user_stats 
			(user_id, total_cards_studied, total_correct, total_sessions,
			 current_streak, longest_streak, last_study_date, xp, updated_at)
		VALUES ($1, $2, $3, 1, 1, 1, $4, $5, NOW())
		ON CONFLICT (user_id) DO UPDATE SET
			total_cards_studied = user_stats.total_cards_studied + $2,
			total_correct       = user_stats.total_correct + $3,
			total_sessions      = user_stats.total_sessions + 1,
			xp                  = user_stats.xp + $5,
			-- Streak logic:
			-- If last_study_date was yesterday → increment streak
			-- If last_study_date was today → keep streak (multiple sessions same day)
			-- Otherwise → reset to 1
			current_streak = CASE
				WHEN user_stats.last_study_date = ($4::date - INTERVAL '1 day')::date THEN user_stats.current_streak + 1
				WHEN user_stats.last_study_date = $4::date THEN user_stats.current_streak
				ELSE 1
			END,
			longest_streak = GREATEST(
				user_stats.longest_streak,
				CASE
					WHEN user_stats.last_study_date = ($4::date - INTERVAL '1 day')::date THEN user_stats.current_streak + 1
					WHEN user_stats.last_study_date = $4::date THEN user_stats.current_streak
					ELSE 1
				END
			),
			last_study_date = $4,
			updated_at = NOW()`

	_, err := db.Instance.Exec(query, userId, cardsStudied, correct, today, xpGained)
	return err
}

// GetLeaderboard returns top users ranked by XP
func (m *UserStatsModel) GetLeaderboard(limit int) ([]UserStats, error) {
	var stats []UserStats
	err := db.Instance.Select(&stats,
		`SELECT * FROM user_stats ORDER BY xp DESC LIMIT $1`, limit)
	return stats, err
}
