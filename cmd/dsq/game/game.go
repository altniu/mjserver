package game

import (
    "fmt"
    "math/rand"
    "net/http"
    "strconv"
    "strings"
    "time"

    "github.com/lonng/nano"
    "github.com/lonng/nano/component"
    "github.com/lonng/nano/serialize/json"
    log "github.com/sirupsen/logrus"
    "github.com/spf13/viper"
)

var (
    version     = ""            // 游戏版本
    consume     = map[int]int{} // 房卡消耗配置
    forceUpdate = false
    logger      = log.WithField("component", "game")
)

// SetCardConsume 设置房卡消耗数量
func SetCardConsume(cfg string) {
    for _, c := range strings.Split(cfg, ",") {
        parts := strings.Split(c, "/")
        if len(parts) < 2 {
            logger.Warnf("无效的房卡配置: %s", c)
            continue
        }
        round, card := strings.TrimSpace(parts[0]), strings.TrimSpace(parts[1])
        rd, err := strconv.Atoi(round)
        if err != nil {
            continue
        }
        cd, err := strconv.Atoi(card)
        if err != nil {
            continue
        }
        consume[rd] = cd
    }

    logger.Infof("当前游戏体力消耗配置: %+v", consume)
}

// Startup 初始化游戏服务器
func Startup() {
    // set nano logger
    nano.SetLogger(log.WithField("component", "nano"))

    rand.Seed(time.Now().Unix())
    version = viper.GetString("update.version")

    heartbeat := viper.GetInt("core.heartbeat")
    if heartbeat < 5 {
        heartbeat = 5
    }
    nano.SetHeartbeatInterval(time.Duration(heartbeat) * time.Second)

    // 房卡消耗配置
    csm := viper.GetString("core.consume")
    SetCardConsume(csm)
    forceUpdate = viper.GetBool("update.force")

    logger.Infof("当前游戏服务器版本: %s, 是否强制更新: %t, 当前心跳时间间隔: %d秒", version, forceUpdate, heartbeat)
    logger.Info("game service starup")

    // register game handler 注册组件
    nano.Register(defaultPlayerManager, component.WithName("gate"))
    nano.Register(defaultDeskManager, component.WithName("game")) //component.WithNameFunc(strings.ToLower)

    // 加密管道
    c := newCrypto()
    pipeline := nano.NewPipeline()
    pipeline.Inbound().PushBack(c.inbound)
    pipeline.Outbound().PushBack(c.outbound)

    //json作为传输协议

    nano.SetWSPath("/nano")
    nano.SetSerializer(json.NewSerializer())
    addr := fmt.Sprintf(":%d", viper.GetInt("game-server.port"))
    nano.SetCheckOriginFunc(func(_ *http.Request) bool { return true })
    nano.ListenWS(addr) //nano.WithPipeline(pipeline)
}
