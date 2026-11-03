// Package database opens the blog's SQLite database, migrates its
// schema with GORM's AutoMigrate and seeds its members.
package database

import (
	"log"
	"os"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"gorm.io/gorm/logger"

	"github.com/Piechutowski/volt/compare/gingorm/models"
)

// Open opens the SQLite file at path with foreign keys on, times in UTC
// and driver errors translated to GORM's.
func Open(path string) (*gorm.DB, error) {
	return gorm.Open(sqlite.Open(path+"?_foreign_keys=on&_busy_timeout=5000"), &gorm.Config{
		NowFunc:        func() time.Time { return time.Now().UTC() },
		TranslateError: true,
		Logger: logger.New(log.New(os.Stderr, "", log.LstdFlags), logger.Config{
			SlowThreshold:             200 * time.Millisecond,
			LogLevel:                  logger.Warn,
			IgnoreRecordNotFoundError: true,
		}),
	})
}

// Migrate creates or updates the tables of every model.
func Migrate(db *gorm.DB) error {
	return db.AutoMigrate(&models.Member{}, &models.Post{}, &models.Tag{}, &models.Comment{}, &models.Audit{})
}

// Seed inserts the three members the blog starts with, once.
func Seed(db *gorm.DB) error {
	members := []models.Member{
		{ID: 1, Name: "Ada", Email: "ada@blog.test"},
		{ID: 2, Name: "Grace", Email: "grace@blog.test"},
		{ID: 3, Name: "Ken", Email: "ken@blog.test"},
	}
	return db.Clauses(clause.OnConflict{DoNothing: true}).Create(&members).Error
}
