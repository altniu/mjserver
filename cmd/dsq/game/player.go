package game

import (
    "github.com/lonng/nano/session"
    "github.com/lonng/nanoserver/cmd/dsq/db"
    "github.com/lonng/nanoserver/cmd/dsq/protocol"
    "github.com/lonng/nanoserver/pkg/async"
    log "github.com/sirupsen/logrus"
)

//Player
type Player struct {
    uid         int64            // 用户ID
    head        string           // 头像地址
    name        string           // 玩家名字
    ip          string           // ip地址
    sex         int              // 性别
    coin        int64            // 金币数量
    camp        int              // 当前玩家在桌上的阵营 1 2
    session     *session.Session // 玩家session
    desk        *Desk            // 当前桌
    logger      *log.Entry       // 日志
    chOperation chan *protocol.OpChoosed
}

//设置玩家所在的桌子
func (p *Player) setDesk(d *Desk) {
    if d == nil {
        p.logger.Error("桌号为空")
        return
    }
    p.desk = d
    p.logger = log.WithFields(log.Fields{fieldDesk: p.desk.roomNo, fieldPlayer: p.uid})
}

func (p *Player) setIp(ip string) {
    p.ip = ip
}

func (p *Player) bindSession(s *session.Session) {
    p.session = s
    p.session.Set(fieldPlayer, p) //"player"字段
}

func (p *Player) removeSession() {
    p.session.Remove(fieldPlayer)
    p.session = nil
}

func (p *Player) Uid() int64 {
    return p.uid
}

func (p *Player) reset() {
    close(p.chOperation)
    p.chOperation = make(chan *protocol.OpChoosed, 1)
}

func newPlayer(s *session.Session, uid int64, name, head string, sex int) *Player {
    p := &Player{
        uid:         uid,
        name:        name,
        head:        head,
        sex:         sex,
        camp:        -1,
        logger:      log.WithField(fieldPlayer, uid),
        chOperation: make(chan *protocol.OpChoosed, 1),
        coin:        1000,
    }
    p.bindSession(s)
    p.syncCoinFromDB()

    return p
}

// 异步从数据库同步玩家数据
func (p *Player) syncCoinFromDB() {
    async.Run(func() {
        /*
            u, err := db.QueryUser(p.uid)
            if err != nil {
                p.logger.Errorf("玩家同步金币错误, Error=%v", err)
                return
            }

            p.coin = u.Coin
            if s := p.session; s != nil {
                s.Push("onCoinChange", &protocol.CoinChangeInformation{p.coin})
            }
        */
        p.session.Push("onCoinChange", &protocol.CoinChangeInformation{1000})
    })
}

// 异步扣除玩家金币
func (p *Player) loseCoin(count int64) {
    async.Run(func() {
        u, err := db.QueryUser(p.uid)
        if err != nil {
            p.logger.Errorf("扣除金币，查询玩家错误, Error=%v", err)
            return
        }

        // 即使数据库不成功，玩家金币数量依然扣除
        p.coin -= count
        u.Coin = p.coin
        if err := db.UpdateUser(u); err != nil {
            p.logger.Errorf("扣除金币，更新金币数量错误, Error=%v", err)
            return
        }

        if u.Coin != p.coin {
            p.logger.Errorf("玩家扣除金币，同步到数据库后，发现金币数量不一致，玩家数量=%d，数据库数量=%d", p.coin, u.Coin)
        }

        if s := p.session; s != nil {
            s.Push("onCoinChange", &protocol.CoinChangeInformation{p.coin})
        }
    })
}

// 断线重连后，同步牌桌数据
func (p *Player) syncDeskData() error {
    desk := p.desk
    data := &protocol.SyncDesk{
        Status:  desk.status(),
        Players: []protocol.DeskPlayerData{},
    }

    for _, player := range desk.players {
        uid := player.Uid()
        playerData := protocol.DeskPlayerData{
            Uid: uid,
        }
        data.Players = append(data.Players, playerData)
    }
    p.logger.Debugf("同步房间数据: %+v", data)
    return p.session.Push("onSyncDesk", data)
}
