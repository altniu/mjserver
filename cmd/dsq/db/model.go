package db

import (
	"time"

	"github.com/lonng/nanoserver/cmd/dsq/db/model"

	_ "github.com/go-sql-driver/mysql"
	"github.com/go-xorm/xorm"
	log "github.com/sirupsen/logrus"
)

//xorm golang orm库支持sql和orm事务

const asyncTaskBacklog = 128

var (
	database *xorm.Engine
	logger   *log.Entry
	chWrite  chan interface{} // async write channel
	chUpdate chan interface{} // async update channel
)

type options struct {
	showSQL      bool
	maxOpenConns int
	maxIdleConns int
}

// ModelOption specifies an option for dialing a xordefaultModel.
type ModelOption func(*options)

// MaxIdleConns specifies the max idle connect numbers.
func MaxIdleConns(i int) ModelOption {
	return func(opts *options) {
		opts.maxIdleConns = i
	}
}

// MaxOpenConns specifies the max open connect numbers.
func MaxOpenConns(i int) ModelOption {
	return func(opts *options) {
		opts.maxOpenConns = i
	}
}

// ShowSQL specifies the buffer size.
func ShowSQL(show bool) ModelOption {
	return func(opts *options) {
		opts.showSQL = show
	}
}

func envInit() {
	// async task
	go func() {
		for {
			select {
			case t, ok := <-chWrite:
				if !ok {
					return
				}

				//insert
				if _, err := database.Insert(t); err != nil {
					logger.Error(err)
				}

			case t, ok := <-chUpdate:
				if !ok {
					return
				}

				//update
				if _, err := database.Update(t); err != nil {
					logger.Error(err)
				}
			}
		}
	}()

	// 定时ping数据库, 保持连接池连接
	go func() {
		//返回一个新的 Ticker，该 Ticker 包含一个通道字段，并会每隔时间段 d 就向该通道发送当时的时间
		ticker := time.NewTicker(time.Minute * 5)
		for {
			select {
			case <-ticker.C:
				database.Ping() //ping数据库
			}
		}
	}()
}

// brief: New create the database's connection
// param: dsn 用户名:密码@(数据库地址:3306)/数据库实例名称?charset=utf8
// param: ...ModelOption 不确定数量参数 slice  ...打散传入
func MustStartup(dsn string, opts ...ModelOption) func() {
	logger = log.WithField("component", "model")

	//声明一个options
	settings := &options{
		maxIdleConns: defaultMaxConns,
		maxOpenConns: defaultMaxConns,
		showSQL:      true,
	}

	// options handle opt为函数func(*options) 使用opt设置settings
	for _, opt := range opts {
		opt(settings)
	}

	logger.Infof("DSN=%s ShowSQL=%t MaxIdleConn=%v MaxOpenConn=%v", dsn, settings.showSQL, settings.maxIdleConns, settings.maxOpenConns)

	// create database instance
	if db, err := xorm.NewEngine("mysql", dsn); err != nil {
		panic(err)
	} else {
		database = db
	}

	// 设置日志相关
	database.SetLogger(&Logger{Entry: logger.WithField("orm", "xorm")})

	// options
	database.SetMaxIdleConns(settings.maxIdleConns) //设置连接池空闲连接数
	database.SetMaxOpenConns(settings.maxOpenConns) //设置最大连接数
	database.ShowSQL(settings.showSQL)              //print sql语句

	//insert update chan
	chWrite = make(chan interface{}, asyncTaskBacklog) //创建一个空接口类型的通道, 可以存放任意格式 缓冲区大小128
	chUpdate = make(chan interface{}, asyncTaskBacklog)

	//自动同步表结构
	syncSchema()

	//异步任务处理数据库的insert和update 和连接测试
	envInit()

	//闭包写法
	closer := func() {
		close(chWrite)
		close(chUpdate)
		database.Close()
		logger.Info("stopped")
	}

	return closer
}

func syncSchema() {
	/* Sync2
	 * 自动检测和创建表，这个检测是根据表的名字
	 * 自动检测和新增表中的字段，这个检测是根据字段名，同时对表中多余的字段给出警告信息
	 * 自动检测，创建和删除索引和唯一索引，
	 * 自动转换varchar字段类型到text字段类型
	 * 自动警告字段的默认值，是否为空信息在模型和数据库之间不匹配的情况
	 */
	database.StoreEngine("InnoDB").Sync2(
		new(model.Agent),
		new(model.CardConsume),
		new(model.Desk),
		new(model.History),
		new(model.Login),
		new(model.Online),
		new(model.Order),
		new(model.Recharge),
		new(model.Register),
		new(model.ThirdAccount),
		new(model.Trade),
		new(model.User),
		new(model.Uuid),
		new(model.Club),
		new(model.UserClub),
	)
}
