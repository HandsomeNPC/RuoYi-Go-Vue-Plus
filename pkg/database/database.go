// Package database MySQL + GORM 初始化。
package database

import (
	"context"
	"fmt"
	"log"
	"sync"

	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/schema"

	"ruoyi-go-vue-plus/pkg/config"
	"ruoyi-go-vue-plus/pkg/repository"
)

// New 按配置建立数据库连接并完成连接池设置。
func New(cfg config.DatasourceConfig) (*gorm.DB, error) {
	db, err := gorm.Open(mysql.Open(cfg.DSN()), &gorm.Config{
		Logger: newLogger(cfg),
		NamingStrategy: schema.NamingStrategy{
			SingularTable: true,
		},
		DisableForeignKeyConstraintWhenMigrating: true,
		SkipDefaultTransaction:                   true,
	})
	if err != nil {
		return nil, fmt.Errorf("database: 连接 %s:%d/%s 失败: %w",
			cfg.Host, cfg.Port, cfg.DBName, err)
	}

	if err := setupPool(db, cfg); err != nil {
		return nil, err
	}
	if err := ping(db); err != nil {
		return nil, err
	}
	// 放 New 而非 Init：两个入口都覆盖，任何 main 都无法漏掉审计字段填充。
	if err := repository.RegisterAuditCallbacks(db); err != nil {
		return nil, fmt.Errorf("database: 注册审计字段回调失败: %w", err)
	}
	return db, nil
}

// setupPool 应用连接池参数。
func setupPool(db *gorm.DB, cfg config.DatasourceConfig) error {
	sqlDB, err := db.DB()
	if err != nil {
		return fmt.Errorf("database: 获取底层连接池失败: %w", err)
	}
	if cfg.MaxOpenConns > 0 {
		sqlDB.SetMaxOpenConns(cfg.MaxOpenConns)
	}
	if cfg.MaxIdleConns > 0 {
		sqlDB.SetMaxIdleConns(cfg.MaxIdleConns)
	}
	if cfg.ConnMaxLifetime > 0 {
		sqlDB.SetConnMaxLifetime(cfg.MaxLifetime())
	}
	if cfg.ConnMaxIdleTime > 0 {
		sqlDB.SetConnMaxIdleTime(cfg.MaxIdleTime())
	}
	return nil
}

// ping 启动探活，确认数据库真实可用。
func ping(db *gorm.DB) error {
	sqlDB, err := db.DB()
	if err != nil {
		return fmt.Errorf("database: 获取底层连接池失败: %w", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), pingTimeout)
	defer cancel()

	if err := sqlDB.PingContext(ctx); err != nil {
		return fmt.Errorf("database: 数据库探活失败: %w", err)
	}
	return nil
}

// Close 关闭连接池，供进程退出时调用。
func Close(db *gorm.DB) error {
	if db == nil {
		return nil
	}
	sqlDB, err := db.DB()
	if err != nil {
		return fmt.Errorf("database: 获取底层连接池失败: %w", err)
	}
	return sqlDB.Close()
}

// 包级默认实例。
var (
	mu        sync.RWMutex
	defaultDB *gorm.DB
)

// Init 建立连接并设置为包级默认实例。
func Init() {
	c := config.Get()
	cfg := c.Datasource
	db, err := New(cfg)
	if err != nil {
		panic(fmt.Errorf("database: 初始化失败: %w", err))
	}
	mu.Lock()
	defaultDB = db
	mu.Unlock()
	log.Printf("[%s] 数据库已连接 %s:%d/%s",
		c.Server.Name, cfg.Host, cfg.Port, cfg.DBName)
}

// DB 返回包级默认实例，未调用 Init 会 panic。
func DB() *gorm.DB {
	mu.RLock()
	db := defaultDB
	mu.RUnlock()
	if db == nil {
		panic("database: 尚未初始化，请先调用 database.Init")
	}
	return db
}

// CloseDefault 关闭并清空包级默认实例，供进程退出时 defer 调用。
func CloseDefault() {
	mu.Lock()
	db := defaultDB
	defaultDB = nil
	mu.Unlock()
	if err := Close(db); err != nil {
		log.Printf("关闭数据库连接失败: %v", err)
	}
}
