package db

import (
	"context"
	"log"
	"os"

	"github.com/jmoiron/sqlx"
	"github.com/redis/go-redis/v9"

	_ "github.com/lib/pq"
)

var Instance *sqlx.DB
var Redis *redis.Client

func InitDB() {
	addr := os.Getenv("DB_ADDR")
	if addr == "" {
		addr = "postgres://rigo_admin:password@localhost:5432/rigo_db?sslmode=disable"
	}

	var err error
	Instance, err = sqlx.Open("postgres", addr)
	if err != nil {
		log.Fatalf("Invalid DB config: %v", err)
	}

	if err = Instance.Ping(); err != nil {
		log.Fatalf("Could not connect to Postgres: %v", err)
	}

	log.Println("Postgres connected successfully!")
}

func InitRedis() {
	addr := os.Getenv("REDIS_ADDR")
	if addr == "" {
		addr = "localhost:6379"
	}

	Redis = redis.NewClient(&redis.Options{
		Addr:     addr,
		Password: "",
		DB:       0,
	})

	ctx := context.Background()

	_, err := Redis.Ping(ctx).Result()
	if err != nil {
		log.Fatalf("Could not connect to Redis: %v", err)
	}

	log.Println("Redis connected successfully!")
}
