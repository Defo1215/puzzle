package database

import (
	"fmt"
	"log"
	"os"
	"puzzle/config"
	"time"

	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"gorm.io/gorm/schema"
)

// DB 数据库连接单例
var db *gorm.DB

func InitMySQL() {
	var err error

	// dsn := "root:fang1215@tcp(127.0.0.1:3306)/puzzle?charset=utf8mb4&parseTime=True&loc=Local"
	dsn := fmt.Sprintf("%s:%s@tcp(%s:%s)/%s?charset=utf8mb4&parseTime=True&loc=Local",
		config.Settings.Mysql.Username,
		config.Settings.Mysql.Password,
		config.Settings.Mysql.Host,
		config.Settings.Mysql.Port,
		config.Settings.Mysql.Database,
	)

	newLogger := logger.New(
		log.New(os.Stdout, "\r\n", log.LstdFlags), // io writer
		logger.Config{
			SlowThreshold: time.Second, // 慢 SQL 阈值
			LogLevel:      logger.Info, // Log level
			Colorful:      true,        // 启用彩色打印
		},
	)

	db, err = gorm.Open(mysql.Open(dsn), &gorm.Config{
		Logger: newLogger,
		NamingStrategy: schema.NamingStrategy{
			//TablePrefix:   "sw_", // 表名前缀，`User` 的表名应该是 `t_users`
			SingularTable: true, // 使用单数表名，启用该选项后，`User` 的表名应该是 `user`
		},
		// SkipDefaultTransaction: true, // 禁用默认事务
	})

	if err != nil {
		fmt.Printf("数据库链接错误: %v", err)
	}

	if db.Error != nil {
		fmt.Printf("数据库链接错误: %v", db.Error)
	}

	// 设置数据库连接池参数
	sqlDB, _ := db.DB()
	// 设置数据库连接池最大连接数
	sqlDB.SetMaxOpenConns(100)
	// 连接池最大允许的空闲连接数，如果没有sql任务需要执行的连接数大于20，超过的连接会被连接池关闭
	sqlDB.SetMaxIdleConns(20)
}

// GetMySQL 获取 gorm db，其他包调用此方法即可拿到 db
// 无需担心不同协程并发时使用这个 db 对象会公用一个连接，因为 db 在调用其方法时候会从数据库连接池获取新的连接
func GetMySQL() *gorm.DB {
	return db
}
