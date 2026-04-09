package main

import (
	"log"
	"net/http"
	"time"

	"github.com/jojipanackal/rigo-api/api"
	"github.com/jojipanackal/rigo-api/db"
	"github.com/jojipanackal/rigo-api/middlewares"
	"github.com/jojipanackal/rigo-api/models"
)

func main() {
	db.InitRedis()
	db.InitDB()

	// ── Models ────────────────────────────────────────────────────────────────
	authModel := &models.AuthModel{}
	userModel := &models.UserModel{}
	deckModel := &models.DeckModel{}
	cardModel := &models.CardModel{}
	documentModel := &models.DocumentModel{}
	ratingModel := &models.RatingModel{}
	bookmarkModel := &models.BookmarkModel{}
	subscriptionModel := &models.SubscriptionModel{}
	userCardProgressModel := &models.UserCardProgressModel{}
	userStatsModel := &models.UserStatsModel{}
	sessionModel := &models.SessionModel{UserStatsModel: userStatsModel}

	// ── Handlers ──────────────────────────────────────────────────────────────
	authHandler := &api.AuthHandler{
		AuthModel: authModel,
		UserModel: userModel,
	}

	deckHandler := &api.DeckHandler{
		DeckModel:     deckModel,
		AuthModel:     authModel,
		UserModel:     userModel,
		RatingModel:   ratingModel,
		BookmarkModel: bookmarkModel,
		SessionModel:  sessionModel,
		DocumentModel: documentModel,
	}

	cardHandler := &api.CardHandler{
		CardModel: cardModel,
		DeckModel: deckModel,
	}

	sessionHandler := &api.SessionHandler{
		SessionModel:          sessionModel,
		CardModel:             cardModel,
		DeckModel:             deckModel,
		UserCardProgressModel: userCardProgressModel,
		BookmarkModel:         bookmarkModel,
	}

	profileHandler := &api.ProfileHandler{
		UserModel:         userModel,
		DeckModel:         deckModel,
		AuthModel:         authModel,
		SubscriptionModel: subscriptionModel,
		BookmarkModel:     bookmarkModel,
		UserStatsModel:    userStatsModel,
	}

	documentHandler := &api.DocumentHandler{
		DocumentModel: documentModel,
		DeckModel:     deckModel,
	}

	// ── Router ────────────────────────────────────────────────────────────────
	mux := http.NewServeMux()

	// Helper: wraps a handler with the auth middleware
	protected := func(h http.HandlerFunc) http.Handler {
		return middlewares.AuthMiddleware(authModel)(h)
	}

	// Health Check
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"up","timestamp":"` + time.Now().Format(time.RFC3339) + `"}`))
	})

	// Auth
	mux.HandleFunc("POST /api/auth/login", authHandler.Login)
	mux.HandleFunc("POST /api/auth/signup", authHandler.Signup)
	mux.HandleFunc("POST /api/auth/refresh", authHandler.Refresh)
	mux.Handle("POST /api/auth/logout", protected(authHandler.Logout))
	mux.Handle("GET /api/auth/me", protected(authHandler.Me))

	// Decks — public
	mux.HandleFunc("GET /api/decks", deckHandler.List)
	mux.HandleFunc("GET /api/decks/popular", deckHandler.Popular)
	mux.HandleFunc("GET /api/decks/search", deckHandler.Search)
	mux.HandleFunc("GET /api/decks/{id}", deckHandler.Get)
	mux.HandleFunc("GET /api/decks/{id}/cards", cardHandler.List)

	// Decks — protected
	mux.Handle("POST /api/decks", protected(deckHandler.Create))
	mux.Handle("PUT /api/decks/{id}", protected(deckHandler.Update))
	mux.Handle("DELETE /api/decks/{id}", protected(deckHandler.Delete))
	mux.Handle("POST /api/decks/{id}/bookmark", protected(deckHandler.Bookmark))
	mux.Handle("GET /api/decks/bookmarks", protected(deckHandler.ListBookmarks))
	mux.Handle("POST /api/decks/{id}/rate", protected(deckHandler.Rate))

	// Cards — protected
	mux.Handle("POST /api/decks/{id}/cards", protected(cardHandler.Create))
	mux.Handle("POST /api/decks/{id}/cards/import", protected(cardHandler.ImportCSV))
	mux.Handle("PUT /api/cards/{id}", protected(cardHandler.Update))
	mux.Handle("DELETE /api/cards/{id}", protected(cardHandler.Delete))

	// Study sessions — protected
	mux.Handle("POST /api/decks/{id}/study", protected(sessionHandler.Start))
	mux.Handle("DELETE /api/decks/{id}/study", protected(sessionHandler.Stop))
	mux.Handle("POST /api/sessions/{id}/answer", protected(sessionHandler.Answer))

	// Users / Profiles
	mux.Handle("GET /api/users/me", protected(profileHandler.GetMe))
	mux.Handle("PUT /api/users/me", protected(profileHandler.UpdateMe))
	mux.HandleFunc("GET /api/users/{id}", profileHandler.GetUser)
	mux.Handle("POST /api/users/{id}/subscribe", protected(profileHandler.Subscribe))

	// Documents (file serving is public; upload/delete are protected)
	mux.HandleFunc("GET /api/documents/{docType}/{refId}", documentHandler.Serve)
	mux.Handle("PUT /api/documents/{docType}/{refId}", protected(documentHandler.Upload))
	mux.Handle("DELETE /api/documents/{docType}/{refId}", protected(documentHandler.Delete))

	log.Println("🚀 Rigo API running at :8080")
	log.Fatal(http.ListenAndServe(":8080", middlewares.CORSMiddleware(loggingMiddleware(mux))))
}

func loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, r)
		log.Printf("%s %s %v", r.Method, r.URL.Path, time.Since(start))
	})
}
