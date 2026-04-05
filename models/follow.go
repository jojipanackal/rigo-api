package models

import (
	"time"

	"github.com/jojipanackal/rigo-api/db"
)

type Subscription struct {
	SubscriberId int64     `db:"subscriber_id" json:"subscriber_id"`
	CreatorId    int64     `db:"creator_id"    json:"creator_id"`
	CreatedAt    time.Time `db:"created_at"    json:"created_at"`
}

// Toggle subscribes or unsubscribes; returns true if now subscribed.
func (m *SubscriptionModel) Toggle(subscriberId, creatorId int64) (bool, error) {
	if m.IsSubscribed(subscriberId, creatorId) {
		return false, m.Unsubscribe(subscriberId, creatorId)
	}
	return true, m.Subscribe(subscriberId, creatorId)
}

type SubscriptionModel struct{}

// Subscribe creates a subscription relationship
func (m *SubscriptionModel) Subscribe(subscriberId, creatorId int64) error {
	tx, err := db.Instance.Beginx()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// Insert subscription record
	_, err = tx.Exec(`
		INSERT INTO subscriptions (subscriber_id, creator_id) 
		VALUES ($1, $2) 
		ON CONFLICT DO NOTHING`,
		subscriberId, creatorId)
	if err != nil {
		return err
	}

	// Update creator's subscribers_count
	_, err = tx.Exec(`
		UPDATE users SET subscribers_count = subscribers_count + 1 
		WHERE id = $1`, creatorId)
	if err != nil {
		return err
	}

	return tx.Commit()
}

// Unsubscribe removes a subscription relationship
func (m *SubscriptionModel) Unsubscribe(subscriberId, creatorId int64) error {
	tx, err := db.Instance.Beginx()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// Delete subscription record
	result, err := tx.Exec(`
		DELETE FROM subscriptions 
		WHERE subscriber_id = $1 AND creator_id = $2`,
		subscriberId, creatorId)
	if err != nil {
		return err
	}

	rows, _ := result.RowsAffected()
	if rows == 0 {
		return nil // Was not subscribed
	}

	// Update creator's subscribers_count
	_, err = tx.Exec(`
		UPDATE users SET subscribers_count = subscribers_count - 1 
		WHERE id = $1 AND subscribers_count > 0`, creatorId)
	if err != nil {
		return err
	}

	return tx.Commit()
}

// IsSubscribed checks if a user is subscribed to another user
func (m *SubscriptionModel) IsSubscribed(subscriberId, creatorId int64) bool {
	var count int
	db.Instance.Get(&count, `
		SELECT COUNT(*) FROM subscriptions 
		WHERE subscriber_id = $1 AND creator_id = $2`,
		subscriberId, creatorId)
	return count > 0
}

// GetSubscribers returns users who are subscribed to the given creator
func (m *SubscriptionModel) GetSubscribers(creatorId int64) ([]User, error) {
	var users []User
	query := `
		SELECT u.* FROM users u
		JOIN subscriptions s ON s.subscriber_id = u.id
		WHERE s.creator_id = $1
		ORDER BY s.created_at DESC`
	err := db.Instance.Select(&users, query, creatorId)
	return users, err
}
