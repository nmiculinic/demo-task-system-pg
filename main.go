package main

import (
	"context"
	"os"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"go.uber.org/zap"
)

var (
	readyTasks = make(chan uuid.UUID, 1000)
	db = *sqlx.D
)

func producer(ctx context.Context) {

}

func worker(ctx context.Context) {
	for task := range readyTasks {
		db.

	}
}

func run() error {
	logger, err := zap.NewDevelopment()
	if err != nil {
		return err
	}
	zap.ReplaceGlobals(logger)
	zap.RedirectStdLog(logger)

	connConfig, err := pgx.ParseConnectionString(os.Getenv("DATABASE_URL"))
	if err != nil {
		return err
	}
	connConfig.Logger = zapadapter.NewLogger(zap.L())
	db = sqlx.NewDb(stdlib.OpenDB(connConfig), "pgx")
	//db.SetMaxIdleConns(2)
	//db.SetMaxOpenConns()
	//db.SetConnMaxIdleTime(cfg.ConnMaxIdleTime)
	//db.SetConnMaxLifetime(cfg.ConnMaxLifetime)
	return nil
}

func main() {
	if err := run(); err != nil {
		panic(err)
	}
}
