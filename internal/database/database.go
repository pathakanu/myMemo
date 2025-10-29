package database

import (
	"log"
	"strings"

	"github.com/pathakanu/myMemo/internal/model"
	"gorm.io/driver/postgres"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// New creates a GORM database connection.
// When databaseURL is provided PostgreSQL is used, otherwise SQLite is used.
func New(databaseURL string) (*gorm.DB, error) {
	var (
		db  *gorm.DB
		err error
	)

	gormConfig := &gorm.Config{
		Logger: logger.Default.LogMode(logger.Warn),
	}

	if databaseURL != "" {
		db, err = gorm.Open(postgres.Open(databaseURL), gormConfig)
	} else {
		db, err = gorm.Open(sqlite.Open("reminders.db"), gormConfig)
	}
	if err != nil {
		return nil, err
	}

	if err := db.AutoMigrate(&model.Reminder{}); err != nil {
		return nil, err
	}

	logBackend(db)
	return db, nil
}

func logBackend(db *gorm.DB) {
	dialector := db.Dialector.Name()
	switch strings.ToLower(dialector) {
	case "postgres":
		log.Printf("database: connected to PostgreSQL")
	case "sqlite":
		log.Printf("database: using SQLite reminders.db")
	default:
		log.Printf("database: connected via %s", dialector)
	}
}
