package models

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/golang-jwt/jwt"
	"github.com/google/uuid"
	"github.com/jojipanackal/rigo-api/db"
)

var ctx = context.Background()

type Token struct {
	AccessToken  string
	RefreshToken string
	AccessUUID   string
	RefreshUUID  string
	AtExpires    int64
	RtExpires    int64
}

type AccessDetails struct {
	AccessUUID string
	UserID     int64
}

type RefreshDetails struct {
	RefreshUUID string
	UserID      int64
}

type AuthModel struct{}

func (m *AuthModel) CreateToken(userId int64) (*Token, error) {

	t := &Token{}
	t.AtExpires = time.Now().Add(time.Minute * 15).Unix()
	t.AccessUUID = uuid.New().String()
	t.RtExpires = time.Now().Add(time.Hour * 24 * 7).Unix()
	t.RefreshUUID = uuid.New().String()

	var err error
	atClaims := jwt.MapClaims{}
	atClaims["authorized"] = true
	atClaims["access_uuid"] = t.AccessUUID
	atClaims["user_id"] = userId
	atClaims["exp"] = t.AtExpires

	at := jwt.NewWithClaims(jwt.SigningMethodHS256, atClaims)
	t.AccessToken, err = at.SignedString([]byte(os.Getenv("ACCESS_SECRET")))
	if err != nil {
		return nil, err
	}

	rtClaims := jwt.MapClaims{}
	rtClaims["refresh_uuid"] = t.RefreshUUID
	rtClaims["user_id"] = userId
	rtClaims["exp"] = t.RtExpires

	rt := jwt.NewWithClaims(jwt.SigningMethodHS256, rtClaims)
	t.RefreshToken, err = rt.SignedString([]byte(os.Getenv("REFRESH_SECRET")))
	if err != nil {
		return nil, err
	}
	return t, nil
}

func (m *AuthModel) CreateAuth(userId int64, td *Token) error {
	at := time.Unix(td.AtExpires, 0)
	rt := time.Unix(td.RtExpires, 0)
	now := time.Now()

	errAccess := db.Redis.Set(ctx, td.AccessUUID, userId, at.Sub(now)).Err()
	if errAccess != nil {
		return errAccess
	}

	errRefresh := db.Redis.Set(ctx, td.RefreshUUID, userId, rt.Sub(now)).Err()
	if errRefresh != nil {
		return errRefresh
	}

	return nil
}

func (m *AuthModel) ExtractToken(r *http.Request) string {
	// First check Authorization header
	bearToken := r.Header.Get("Authorization")
	if strings.HasPrefix(strings.ToLower(bearToken), "bearer ") {
		return strings.TrimSpace(bearToken[7:])
	}

	// Fallback to cookie
	cookie, err := r.Cookie("access_token")
	if err == nil && cookie.Value != "" {
		return cookie.Value
	}

	return ""
}

func (m *AuthModel) ExtractRefreshToken(r *http.Request) string {
	if token := r.Header.Get("X-Refresh-Token"); token != "" {
		return token
	}
	cookie, err := r.Cookie("refresh_token")
	if err == nil && cookie.Value != "" {
		return cookie.Value
	}
	return ""
}

func (m *AuthModel) VerifyToken(r *http.Request) (*jwt.Token, error) {
	tokenString := m.ExtractToken(r)
	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return []byte(os.Getenv("ACCESS_SECRET")), nil
	})
	if err != nil {
		return nil, err
	}
	return token, nil
}

func (m *AuthModel) TokenValid(r *http.Request) error {
	token, err := m.VerifyToken(r)
	if err != nil {
		return err
	}
	if _, ok := token.Claims.(jwt.MapClaims); !ok && !token.Valid {
		return err
	}
	return nil
}

func (m *AuthModel) ExtractTokenMetadata(r *http.Request) (*AccessDetails, error) {
	token, err := m.VerifyToken(r)
	if err != nil {
		return nil, err
	}
	claims, ok := token.Claims.(jwt.MapClaims)
	if ok && token.Valid {
		accessUUID, ok := claims["access_uuid"].(string)
		if !ok {
			return nil, err
		}
		userID, err := strconv.ParseInt(fmt.Sprintf("%.f", claims["user_id"]), 10, 64)
		if err != nil {
			return nil, err
		}
		return &AccessDetails{
			AccessUUID: accessUUID,
			UserID:     userID,
		}, nil
	}
	return nil, err
}

func (m *AuthModel) FetchAuth(authD *AccessDetails) (int64, error) {

	userid, err := db.Redis.Get(ctx, authD.AccessUUID).Result()
	if err != nil {
		return 0, err
	}
	userID, _ := strconv.ParseInt(userid, 10, 64)
	return userID, nil
}

func (m *AuthModel) DeleteAuth(givenUUID string) (int64, error) {

	deleted, err := db.Redis.Del(ctx, givenUUID).Result()
	if err != nil {
		return 0, err
	}
	return deleted, nil
}

// ValidateRefreshToken parses a refresh JWT and returns its metadata.
func (m *AuthModel) ValidateRefreshToken(tokenString string) (*RefreshDetails, error) {
	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return []byte(os.Getenv("REFRESH_SECRET")), nil
	})
	if err != nil || !token.Valid {
		return nil, fmt.Errorf("invalid refresh token")
	}
	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return nil, fmt.Errorf("invalid claims")
	}
	refreshUUID, ok := claims["refresh_uuid"].(string)
	if !ok || refreshUUID == "" {
		return nil, fmt.Errorf("missing refresh uuid")
	}
	userID, err := strconv.ParseInt(fmt.Sprintf("%.f", claims["user_id"]), 10, 64)
	if err != nil {
		return nil, err
	}
	return &RefreshDetails{
		RefreshUUID: refreshUUID,
		UserID:      userID,
	}, nil
}
