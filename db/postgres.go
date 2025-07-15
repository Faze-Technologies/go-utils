package db

import (
	"context"
	"fmt"
	"github.com/Faze-Technologies/go-utils/config"
	"net/url"

	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"
)

func InitPostgresDB(logger *zap.Logger) *pgxpool.Pool {
	dbURL := fmt.Sprintf("postgres://%s:%s@%s:%d/%s?sslmode=%s",
		config.GetString("postgres.user"),
		url.PathEscape(config.GetString("postgres.password")),
		config.GetString("postgres.host"),
		config.GetInt("postgres.port"),
		config.GetString("postgres.dbname"),
		config.GetString("postgres.sslmode"))

	logger.Info("Connecting to database", zap.String("db_url", dbURL))

	conn, err := pgxpool.New(context.Background(), dbURL)
	if err != nil {
		logger.Fatal("Error connecting to database", zap.Error(err))
	}

	logger.Info("Connected to database!!")
	return conn
}

func ClosePostgresDB(db *pgxpool.Pool) {
	db.Close()
}
