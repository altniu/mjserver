package game

import (
	"github.com/lonng/nanoserver/cmd/dsq/protocol"

	"time"

	"github.com/lonng/nano"
	"github.com/lonng/nano/component"
	"github.com/lonng/nano/session"
	log "github.com/sirupsen/logrus"
)

const kickResetBacklog = 8

//用户管理组件
var defaultPlayerManager = NewPlayerManager()

type (
	PlayerManager struct {
		component.Base
		group      *nano.Group       // 全局广播channel session管理
		players    map[int64]*Player // 玩家列表
		chKick     chan int64        // 退出队列
		chReset    chan int64        // 重置队列
		chRecharge chan RechargeInfo // 充值信息
	}

	RechargeInfo struct {
		Uid  int64 // 用户ID
		Coin int64 // 金币数量
	}
)

func NewPlayerManager() *PlayerManager {
	return &PlayerManager{
		group:      nano.NewGroup("_SYSTEM_MESSAGE_BROADCAST"),
		players:    map[int64]*Player{},
		chKick:     make(chan int64, kickResetBacklog),
		chReset:    make(chan int64, kickResetBacklog),
		chRecharge: make(chan RechargeInfo, 32),
	}
}

//Init，AfterInit， BeforeShutdown，Shutdown
func (m *PlayerManager) AfterInit() {
	//session 负责网络接口、uid、id和数据 session关闭时从group中移除
	session.Lifetime.OnClosed(func(s *session.Session) {
		m.group.Leave(s)
	})

	// 处理踢出玩家和重置玩家消息(来自http)
	nano.NewTimer(time.Second, func() {
	ctrl:
		for {
			select {
			case uid := <-m.chKick:
				p, ok := defaultPlayerManager.player(uid)
				if !ok || p.session == nil {
					logger.Errorf("玩家%d不在线", uid)
				}
				p.session.Close()
				logger.Infof("踢出玩家, UID=%d", uid)

			case uid := <-m.chReset:
				p, ok := defaultPlayerManager.player(uid)
				if !ok {
					return
				}
				if p.session != nil {
					logger.Errorf("玩家正在游戏中，不能重置: %d", uid)
					return
				}
				p.desk = nil
				logger.Infof("重置玩家, UID=%d", uid)

			case ri := <-m.chRecharge:
				player, ok := m.player(ri.Uid)
				// 如果玩家在线 通知玩家金币变化
				if s := player.session; ok && s != nil {
					s.Push("onCoinChange", &protocol.CoinChangeInformation{Coin: ri.Coin})
				}

			default:
				break ctrl // break 和 continue 配合for 可以做goto类似语法
			}
		}
	})
}

//处理登录
func (m *PlayerManager) Login(s *session.Session, req *protocol.LoginToGameServerRequest) error {
	uid := req.Uid
	s.Bind(uid) //session绑定uid

	log.Infof("玩家: %d登录: %+v", uid, req)
	if p, ok := m.player(uid); !ok {
		log.Infof("玩家: %d不在线，创建新的玩家", uid)
		p = newPlayer(s, uid, req.Name, req.HeadUrl, req.Sex)
		m.setPlayer(uid, p)
	} else {
		log.Infof("玩家: %d已经在线", uid)
		// 移除广播频道
		m.group.Leave(s)

		// 重置之前的session
		if prevSession := p.session; prevSession != nil && prevSession != s {
			// 如果之前房间存在，则退出来
			if p, err := playerWithSession(prevSession); err == nil && p != nil && p.desk != nil && p.desk.group != nil {
				p.desk.group.Leave(prevSession)
			}
			// 重置uid和data close net
			prevSession.Clear()
			prevSession.Close()
		}

		// 绑定新session
		p.bindSession(s)
	}

	// 添加到广播频道
	m.group.Add(s)

	res := &protocol.LoginToGameServerResponse{
		Uid:      s.UID(),
		Nickname: req.Name,
		Sex:      req.Sex,
		HeadUrl:  req.HeadUrl,
		Coin:     10000,
	}

	//返回消息给client
	return s.Response(res)
}

func (m *PlayerManager) player(uid int64) (*Player, bool) {
	p, ok := m.players[uid]

	return p, ok
}

func (m *PlayerManager) setPlayer(uid int64, p *Player) {
	if _, ok := m.players[uid]; ok {
		log.Warnf("玩家已经存在，正在覆盖玩家， UID=%d", uid)
	}
	m.players[uid] = p
}

func (m *PlayerManager) sessionCount() int {
	return len(m.players)
}

func (m *PlayerManager) offline(uid int64) {
	delete(m.players, uid)
	log.Infof("玩家: %d从在线列表中删除, 剩余：%d", uid, len(m.players))
}

func (m *PlayerManager) CheckOrder(s *session.Session, msg *protocol.CheckOrderReqeust) error {
	log.Infof("%+v", msg)

	return s.Response(&protocol.CheckOrderResponse{
		FangKa: 20,
	})
}
