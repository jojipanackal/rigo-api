package models

import (
	"time"

	"github.com/jojipanackal/rugo/db"
	"golang.org/x/crypto/bcrypt"
)

type User struct {
	Id               int64     `db:"id"                json:"id"`
	Name             string    `db:"name"              json:"name"`
	Email            string    `db:"email"             json:"email"`
	PasswordHash     string    `db:"password_hash"     json:"-"`
	Bio              *string   `db:"bio"               json:"bio"`
	SubscribersCount int       `db:"subscribers_count" json:"subscribers_count"`
	CreatedAt        time.Time `db:"created_at"        json:"created_at"`
	UpdatedAt        time.Time `db:"updated_at"        json:"updated_at"`
}

type UserModel struct{}

// Create registers a new user with hashed password and seeds user_stats
func (m *UserModel) Create(name, email, password string) (int64, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return 0, err
	}

	var id int64
	query := `
		INSERT INTO users (name, email, password_hash)
		VALUES ($1, $2, $3)
		RETURNING id`

	err = db.Instance.QueryRow(query, name, email, string(hash)).Scan(&id)
	if err != nil {
		return 0, err
	}

	// Seed user_stats row so stats queries never fail
	db.Instance.Exec(`INSERT INTO user_stats (user_id) VALUES ($1) ON CONFLICT DO NOTHING`, id)

	return id, nil
}

// GetByEmail retrieves a user by email address
func (m *UserModel) GetByEmail(email string) (User, error) {
	var u User
	err := db.Instance.Get(&u, "SELECT * FROM users WHERE email = $1", email)
	return u, err
}

// GetById retrieves a user by ID
func (m *UserModel) GetById(id int64) (User, error) {
	var u User
	err := db.Instance.Get(&u, "SELECT * FROM users WHERE id = $1", id)
	return u, err
}

// VerifyPassword checks if the provided password matches the user's hash
func (m *UserModel) VerifyPassword(user User, password string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password))
	return err == nil
}

// EmailExists checks if an email is already registered
func (m *UserModel) EmailExists(email string) bool {
	var count int
	db.Instance.Get(&count, "SELECT COUNT(*) FROM users WHERE email = $1", email)
	return count > 0
}

// UpdateProfile updates the user's name and bio
func (m *UserModel) UpdateProfile(id int64, name, bio string) error {
	_, err := db.Instance.Exec(
		"UPDATE users SET name = $1, bio = $2, updated_at = NOW() WHERE id = $3",
		name, bio, id)
	return err
}

// GetFollowing returns all users that userId is subscribed to
func (m *UserModel) GetFollowing(userId int64) ([]User, error) {
	var users []User
	query := `
		SELECT u.* FROM users u
		JOIN subscriptions s ON s.creator_id = u.id
		WHERE s.subscriber_id = $1`
	err := db.Instance.Select(&users, query, userId)
	return users, err
}
