package main

import (
	"fmt"
	"os"
	"runtime/pprof"
	"sync"
	"time"

	_ "github.com/go-sql-driver/mysql"
	"github.com/lonng/nano"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/viper"
	"github.com/urfave/cli"

	"github.com/lonng/nanoserver/cmd/dsq/game"
	"github.com/lonng/nanoserver/cmd/dsq/web"
)

func main() {
	//命令行应用
	app := cli.NewApp()

	app.Name = "dsq server"
	app.Author = "dsq"
	app.Version = "0.0.1"
	app.Copyright = "dsq team reserved"
	app.Usage = "dsq server"

	// flags
	app.Flags = []cli.Flag{
		cli.StringFlag{
			Name:  "config, c",
			Value: "./configs/dsq/config.toml",
			Usage: "load configuration from `FILE`",
		},
	}

	app.Action = serve
	err := app.Run(os.Args)
	if err != nil {
		log.Fatal(err)
	}
}

func serve(c *cli.Context) error {
	nano.EnableDebug()

	//c.Args().Get(0) 可以获得运行参数
	//viper从命令行标志 环境变量 本地配置文件 远程配置系统etcd等读取配置信息
	viper.SetConfigType("toml")
	viper.SetConfigFile(c.String("config"))
	viper.ReadInConfig()

	//设置日志的输出格式 logrus.JSONFormatter{}和logrus.TextFormatter{}
	log.SetFormatter(&log.TextFormatter{DisableColors: true})
	if viper.GetBool("core.debug") {
		log.SetLevel(log.DebugLevel) //设置最低的日志级别
	}

	//是否启动性能监控
	if viper.GetBool("core.cpuprofile") {
		filename := fmt.Sprintf("cpuprofile-%d.pprof", time.Now().Unix())
		f, err := os.OpenFile(filename, os.O_WRONLY|os.O_CREATE, os.ModePerm)
		if err != nil {
			panic(err)
		}
		pprof.StartCPUProfile(f)
		defer pprof.StopCPUProfile()
	}

	//等待一组操作完成
	wg := sync.WaitGroup{}
	wg.Add(2)

	//启动web和game
	go func() { defer wg.Done(); game.Startup() }() // 开启游戏服
	go func() { defer wg.Done(); web.Startup() }()  // 开启web服务器

	wg.Wait()
	return nil
}
