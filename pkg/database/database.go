// Package database MySQL + GORM 初始化。各进程共用同一数据库。
//
// 两种用法，按需选择：
//
//	db, err := database.New(cfg.Datasource)   // 返回实例，自行注入 repository
//	database.Init()                           // 同时设置为包级默认，database.DB() 取用
//	                                          // 数据源配置取自 config.Get()，失败 panic
//
// 表名策略为 **单数**（SingularTable），与原项目表结构对齐：
// 实体 SysUser → 表 sys_user，而非 GORM 默认的 sys_users。
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
//
// 数据源配置取自 config.Get()（Load 已写入包级实例），调用方无需传入 ——
// 与本仓库「配置一律走 config.Get()」的约定一致。故必须在 config.Load 之后调用。
// 连接成功后打印一行连接信息，把 main 里那行 log 收进来。
//
// 连不上库直接 panic，不回传 error —— 与 config.Load / config.Get 同源：
// 数据库初始化是启动期编排问题，进程本就无法工作，把 error 逐层往上传只是把
// 「立刻崩」延后成「崩在别处」。失败时不改动包级实例。
//
// 注意：资源回收不在本函数内 defer。defer CloseDefault() 会在 Init 返回时
// 立即触发、当场把连接关掉，server 随后就是空跑。关闭必须留在进程入口的
// run() 里 defer，跟着 r.Run 的退出一起收尾。
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
//
// 供进程退出时直接 defer 调用：`defer database.CloseDefault()`。
// 错误只打日志不外抛 —— 此时进程已在收尾，调用方也没法再做什么，
// 把 error 返出去只会让 main 多写一层 if 兜底。失败时仍清空包级实例。
//
// 注意：Close（按实例关闭）仍返回 error，那是可复用的通用工具，调用方
// 可能要在非进程退出场景下处理失败；只有 CloseDefault 是启动/退出的编排钩子，
// 才把日志收进来。
func CloseDefault() {
	mu.Lock()
	db := defaultDB
	defaultDB = nil
	mu.Unlock()
	if err := Close(db); err != nil {
		log.Printf("关闭数据库连接失败: %v", err)
	}
}
