package main

import (
	"context"
	"database/sql"

	"github.com/DIMO-Network/meta-transaction-processor/internal/config"
	"github.com/pressly/goose/v3"
	"github.com/rs/zerolog"
)

func migrateDatabase(logger zerolog.Logger, settings *config.Settings, command string) {
	var db *sql.DB
	// setup database
	db, err := sql.Open("postgres", settings.DB.BuildConnectionString(true))
	defer func() {
		if err := db.Close(); err != nil {
			logger.Fatal().Msgf("goose: failed to close DB: %v\n", err)
		}
	}()
	if err != nil {
		logger.Fatal().Msgf("failed to open db connection: %v\n", err)
	}
	if err = db.Ping(); err != nil {
		logger.Fatal().Msgf("failed to ping db: %v\n", err)
	}
	// set default
	if command == "" {
		command = "up"
	}
	// todo manually run sql to create devices_api schema
	_, err = db.Exec("CREATE SCHEMA IF NOT EXISTS meta_transaction_processor;")
	if err != nil {
		logger.Fatal().Err(err).Msg("could not create schema")
	}
	goose.SetTableName("meta_transaction_processor.migrations")
	if err := goose.RunContext(context.TODO(), command, db, "migrations"); err != nil {
		logger.Fatal().Msgf("failed to apply go code migrations: %v\n", err)
	}
	// if we add any code migrations import _ "github.com/DIMO-Network/devices-api/migrations" // migrations won't work without this
}
