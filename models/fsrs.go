package models

import (
	"math"
	"time"
)

// FSRS Rating constants
const (
	RatingAgain = 1 // Forgot completely
	RatingHard  = 2 // Recalled with difficulty
	RatingGood  = 3 // Recalled correctly
	RatingEasy  = 4 // Very easy recall
)

// Card State constants
const (
	StateNew        = 0
	StateLearning   = 1
	StateReview     = 2
	StateRelearning = 3
)

// FSRS default parameters (pre-optimized from research)
// These are based on FSRS-4.5 defaults
var FSRSParams = struct {
	W                [19]float64 // Weight parameters
	RequestRetention float64     // Target retention rate
	MaximumInterval  float64     // Maximum days between reviews
	DecayFactor      float64     // Memory decay factor
}{
	W: [19]float64{
		0.4072, 1.1829, 3.1262, 15.4722, // Initial stability for Again/Hard/Good/Easy
		7.2102, 0.5316, 1.0651, 0.0234, // Difficulty parameters
		1.616, 0.1544, 1.0061, // Stability after recall
		1.9395, 0.1079, 0.3389, // Stability after forget
		0.2189, 0.0, 2.327, // Hard/Easy penalty/bonus
		0.32, 0.0, // Short-term stability
	},
	RequestRetention: 0.9,   // 90% target retention
	MaximumInterval:  365.0, // Max 1 year between reviews
	DecayFactor:      -0.5,  // Power law decay
}

// FSRS represents the Free Spaced Repetition Scheduler
type FSRS struct{}

// FSRSCard holds the FSRS-specific state for a card
type FSRSCard struct {
	Stability  float64 // Memory stability in days
	Difficulty float64 // Card difficulty (0-1)
	Reps       int     // Number of successful reviews
	Lapses     int     // Number of times forgotten
	State      int     // Current state
	LastReview time.Time
}

// NewFSRSCard creates a new FSRS card with initial values
func NewFSRSCard() FSRSCard {
	return FSRSCard{
		Stability:  0,
		Difficulty: 0.3, // Initial difficulty
		Reps:       0,
		Lapses:     0,
		State:      StateNew,
	}
}

// Review processes a review and returns updated card state and next review date
func (f *FSRS) Review(card FSRSCard, rating int, now time.Time) (FSRSCard, time.Time) {
	if card.State == StateNew {
		// First review - initialize stability based on rating
		card = f.initializeCard(card, rating)
	} else {
		// Update existing card
		card = f.updateCard(card, rating, now)
	}

	// Calculate next review interval
	interval := f.nextInterval(card.Stability)
	nextReview := now.Add(time.Duration(interval*24) * time.Hour)

	return card, nextReview
}

// initializeCard sets initial values for a new card
func (f *FSRS) initializeCard(card FSRSCard, rating int) FSRSCard {
	// Initial stability based on first rating
	switch rating {
	case RatingAgain:
		card.Stability = FSRSParams.W[0]
		card.Lapses++
		card.State = StateRelearning
	case RatingHard:
		card.Stability = FSRSParams.W[1]
		card.Reps++
		card.State = StateLearning
	case RatingGood:
		card.Stability = FSRSParams.W[2]
		card.Reps++
		card.State = StateReview
	case RatingEasy:
		card.Stability = FSRSParams.W[3]
		card.Reps++
		card.State = StateReview
	}

	// Initial difficulty based on rating
	card.Difficulty = f.initDifficulty(rating)

	return card
}

// initDifficulty calculates initial difficulty from first rating
func (f *FSRS) initDifficulty(rating int) float64 {
	// D0 = w4 - exp(w5 * (rating - 1)) + 1
	d := FSRSParams.W[4] - math.Exp(FSRSParams.W[5]*float64(rating-1)) + 1
	return clamp(d, 0.01, 1.0)
}

// updateCard processes a review for an existing card
func (f *FSRS) updateCard(card FSRSCard, rating int, now time.Time) FSRSCard {
	// Calculate elapsed time since last review
	elapsedDays := now.Sub(card.LastReview).Hours() / 24
	if elapsedDays < 0 {
		elapsedDays = 0
	}

	// Calculate retrievability (probability of recall)
	retrievability := f.retrievability(elapsedDays, card.Stability)

	// Update difficulty
	card.Difficulty = f.nextDifficulty(card.Difficulty, rating)

	if rating == RatingAgain {
		// Forgot - calculate post-lapse stability
		card.Stability = f.stabilityAfterForget(card.Difficulty, card.Stability, retrievability)
		card.Lapses++
		card.State = StateRelearning
	} else {
		// Recalled - calculate new stability
		card.Stability = f.stabilityAfterRecall(card.Difficulty, card.Stability, retrievability, rating)
		card.Reps++
		card.State = StateReview
	}

	return card
}

// retrievability calculates probability of recall
func (f *FSRS) retrievability(elapsedDays, stability float64) float64 {
	if stability <= 0 {
		return 0
	}
	// R = (1 + FACTOR * t/S)^DECAY
	factor := 19.0 / 81.0 // Derived from 90% retention target
	return math.Pow(1+factor*elapsedDays/stability, FSRSParams.DecayFactor)
}

// nextDifficulty calculates updated difficulty after a review
func (f *FSRS) nextDifficulty(d float64, rating int) float64 {
	// D' = w7 * D0 + (1 - w7) * (D - w6 * (rating - 3))
	d0 := f.initDifficulty(rating)
	newD := FSRSParams.W[7]*d0 + (1-FSRSParams.W[7])*(d-FSRSParams.W[6]*float64(rating-3))
	return clamp(newD, 0.01, 1.0)
}

// stabilityAfterRecall calculates new stability when card was recalled
func (f *FSRS) stabilityAfterRecall(d, s, r float64, rating int) float64 {
	// S' = S * (e^w8 * (11-D) * S^-w9 * (e^(w10*(1-R)) - 1) * hardPenalty * easyBonus + 1)
	hardPenalty := 1.0
	easyBonus := 1.0

	if rating == RatingHard {
		hardPenalty = FSRSParams.W[15]
	}
	if rating == RatingEasy {
		easyBonus = FSRSParams.W[16]
	}

	factor := math.Exp(FSRSParams.W[8]) *
		(11 - d) *
		math.Pow(s, -FSRSParams.W[9]) *
		(math.Exp(FSRSParams.W[10]*(1-r)) - 1) *
		hardPenalty *
		easyBonus

	newS := s * (factor + 1)
	return math.Min(newS, FSRSParams.MaximumInterval)
}

// stabilityAfterForget calculates new stability when card was forgotten
func (f *FSRS) stabilityAfterForget(d, s, r float64) float64 {
	// S' = w11 * D^-w12 * ((S+1)^w13 - 1) * e^(w14*(1-R))
	newS := FSRSParams.W[11] *
		math.Pow(d, -FSRSParams.W[12]) *
		(math.Pow(s+1, FSRSParams.W[13]) - 1) *
		math.Exp(FSRSParams.W[14]*(1-r))

	return math.Max(newS, 0.1) // Minimum stability of 0.1 days
}

// nextInterval calculates the interval in days for target retention
func (f *FSRS) nextInterval(stability float64) float64 {
	// I = S * (R^(1/DECAY) - 1) / FACTOR
	factor := 19.0 / 81.0
	interval := stability * (math.Pow(FSRSParams.RequestRetention, 1/FSRSParams.DecayFactor) - 1) / factor

	// Clamp to reasonable bounds
	interval = math.Max(interval, 1)
	interval = math.Min(interval, FSRSParams.MaximumInterval)

	return math.Round(interval)
}

// GetRetrievability returns current recall probability for a card
func (f *FSRS) GetRetrievability(card FSRSCard, now time.Time) float64 {
	if card.State == StateNew {
		return 0
	}
	elapsedDays := now.Sub(card.LastReview).Hours() / 24
	return f.retrievability(elapsedDays, card.Stability)
}

// Helper function to clamp a value between min and max
func clamp(value, min, max float64) float64 {
	if value < min {
		return min
	}
	if value > max {
		return max
	}
	return value
}
