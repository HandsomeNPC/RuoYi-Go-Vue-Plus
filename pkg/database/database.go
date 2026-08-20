// Package database MySQL + GORM 初始化。各进程共用同一数据库。
//
// 两种用法，按需选择：
//
//	db, err := database.New(cfg.Datasource)   // 返回实例，自行注入 repository
//	err := database.Init(cfg.Datasource)      // 同时设置为包级默认，database.DB() 取用
//
// 表名策略为 **单数**（SingularTable），与原项目表结构对齐：
// 实体 SysUser → 表 sys_user，而非 GORM 默认的 sys_users。
package database

import (
	"context"
	"fmt"
	"sync"

	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/schema"

	"ruoyi-go-vue-plus/pkg/config"
)

// New 按配置建立数据库连接并完成连接池设置。
//
// 返回前会 Ping 一次，连不上直接报错，避免进程带着坏连接启动。
func New(cfg config.Datasource) (*gorm.DB, error) {
	db, err := gorm.Open(mysql.Open(cfg.DSN()), &gorm.Config{
		Logger: newLogger(cfg),
		NamingStrategy: schema.NamingStrategy{
			SingularTable: true, // sys_user 而非 sys_users
		},
		// 原项目不使用物理外键，迁移时不生成约束。
		DisableForeignKeyConstraintWhenMigrating: true,
		// 关闭默认事务，写操作由 service 层显式控制。
		SkipDefaultTransaction: true,
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
	return db, nil
}

// setupPool 应用连接池参数。未配置(<=0)的项交给 database/sql 默认值。
func setupPool(db *gorm.DB, cfg config.Datasource) error {
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

// 包级默认实例。供 Init / DB / CloseDefault 使用，读写加锁以免竞态。
var (
	mu        sync.RWMutex
	defaultDB *gorm.DB
)

// Init 建立连接并设置为包级默认实例。
func Init(cfg config.Datasource) error {
	db, err := New(cfg)
	if err != nil {
		return err
	}
	mu.Lock()
	defaultDB = db
	mu.Unlock()
	return nil
}

// DB 返回包级默认实例。未调用 Init 会 panic——
// 这是启动期编排错误，不该留到运行时才发现。
func DB() *gorm.DB {
	mu.RLock()
	db := defaultDB
	mu.RUnlock()
	if db == nil {
		panic("database: 尚未初始化，请先调用 database.Init")
	}
	return db
}

// CloseDefault 关闭并清空包级默认实例。
func CloseDefault() error {
	mu.Lock()
	db := defaultDB
	defaultDB = nil
	mu.Unlock()
	return Close(db)
}
