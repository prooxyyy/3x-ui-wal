package database

import (
	"bytes"
	"io"
	"io/fs"
	"os"
	"path"

	"x-ui/config"
	"x-ui/database/model"
	"x-ui/xray"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

var db *gorm.DB

var initializers = []func() error{
	initUser,
	initInbound,
	initOutbound,
	initSetting,
	initInboundClientIps,
	initClientTraffic,
}

func initUser() error {
	err := db.AutoMigrate(&model.User{})
	if err != nil {
		return err
	}
	var count int64
	err = db.Model(&model.User{}).Count(&count).Error
	if err != nil {
		return err
	}
	if count == 0 {
		user := &model.User{
			Username:    "admin",
			Password:    "admin",
			LoginSecret: "",
		}
		return db.Create(user).Error
	}
	return nil
}

func initInbound() error {
	return db.AutoMigrate(&model.Inbound{})
}

func initOutbound() error {
	return db.AutoMigrate(&model.OutboundTraffics{})
}

func initSetting() error {
	return db.AutoMigrate(&model.Setting{})
}

func initInboundClientIps() error {
	return db.AutoMigrate(&model.InboundClientIps{})
}

func initClientTraffic() error {
	return db.AutoMigrate(&xray.ClientTraffic{})
}

func InitDB(dbPath string) error {
	dir := path.Dir(dbPath)
	if err := os.MkdirAll(dir, fs.ModePerm); err != nil {
		return err
	}

	var gormLogger logger.Interface
	if config.IsDebug() {
		gormLogger = logger.Default
	} else {
		gormLogger = logger.Discard
	}

	c := &gorm.Config{Logger: gormLogger}

	// Открываем БД
	var err error
	db, err = gorm.Open(sqlite.Open(dbPath), c)
	if err != nil {
		return err
	}

	// Применяем PRAGMA
	if err := applyPragmas(); err != nil {
		return err
	}

	// Миграции
	for _, initialize := range initializers {
		if err := initialize(); err != nil {
			return err
		}
	}

	return nil
}

func applyPragmas() error {
	// WAL режим журнала
	if err := db.Exec("PRAGMA journal_mode = WAL;").Error; err != nil {
		return err
	}
	// Таймаут на блокировки (мс)
	if err := db.Exec("PRAGMA busy_timeout = 5000;").Error; err != nil {
		return err
	}
	// Разрешить read-uncommitted (если тебе это нужно)
	if err := db.Exec("PRAGMA read_uncommitted = 1;").Error; err != nil {
		return err
	}
	// Немедленный чекпоинт с усечением WAL
	if err := db.Exec("PRAGMA wal_checkpoint(TRUNCATE);").Error; err != nil {
		return err
	}
	// Оптимизация
	if err := db.Exec("PRAGMA optimize;").Error; err != nil {
		return err
	}
	return nil
}

func GetDB() *gorm.DB {
	return db
}

func IsNotFound(err error) bool {
	return err == gorm.ErrRecordNotFound
}

func IsSQLiteDB(file io.ReaderAt) (bool, error) {
	signature := []byte("SQLite format 3\x00")
	buf := make([]byte, len(signature))
	_, err := file.ReadAt(buf, 0)
	if err != nil {
		return false, err
	}
	return bytes.Equal(buf, signature), nil
}

func Checkpoint() error {
	// Принудительно сбросить WAL и обрезать его
	if err := db.Exec("PRAGMA wal_checkpoint(TRUNCATE);").Error; err != nil {
		return err
	}
	// Дополнительно можно оптимизировать
	if err := db.Exec("PRAGMA optimize;").Error; err != nil {
		return err
	}
	return nil
}
