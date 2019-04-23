package game

import (
	"strings"
	"time"

	"github.com/lonng/nanoserver/cmd/dsq/db"
	"github.com/lonng/nanoserver/cmd/dsq/protocol"
	"github.com/lonng/nanoserver/cmd/dsq/room"
	"github.com/lonng/nanoserver/pkg/async"
	"github.com/lonng/nanoserver/pkg/errutil"

	"github.com/lonng/nano"
	"github.com/lonng/nano/component"
	"github.com/lonng/nano/session"
)

var (
	deskPlayerNumEnough  = &protocol.JoinDeskResponse{Code: eDeskPlayerNumEnough, Error: deskPlayerNumEnoughMessage}
	joinVersionExpire    = &protocol.JoinDeskResponse{Code: eJoinVersionExpire, Error: versionExpireMessage}
	deskNotFoundResponse = &protocol.JoinDeskResponse{Code: eDeskNotFoundResponse, Error: deskNotFoundMessage}
	reentryDesk          = &protocol.CreateDeskResponse{Code: eReentryDesk, Error: inRoomPlayerNowMessage}
	createVersionExpire  = &protocol.CreateDeskResponse{Code: eCreateVersionExpire, Error: versionExpireMessage}
	deskCardNotEnough    = &protocol.CreateDeskResponse{Code: eDeskCardNotEnough, Error: deskCardNotEnoughMessage}
)

type (
	DeskManager struct {
		component.Base
		desks map[RoomNumber]*Desk
	}
)

var defaultDeskManager = NewDeskManager()

func NewDeskManager() *DeskManager {
	return &DeskManager{
		desks: map[RoomNumber]*Desk{},
	}
}

func (manager *DeskManager) AfterInit() {
	session.Lifetime.OnClosed(func(s *session.Session) {
		if s.UID() > 0 {
			if err := manager.onPlayerDisconnect(s); err != nil {
				logger.Errorf("玩家退出: UID=%d, Error=%s", s.UID, err.Error())
			}
		}
	})

	nano.NewTimer(300*time.Second, func() {
		destroyDesk := map[RoomNumber]*Desk{}
		deadline := time.Now().Add(30 * time.Minute).Unix()
		for no, d := range manager.desks {
			if d.status() == DeskStatusDestory || d.createdAt < deadline {
				destroyDesk[no] = d
			}
		}
		for _, d := range destroyDesk {
			d.destroy()
		}

		manager.dumpDeskInfo()

		//统计结果异步写入数据库
		sCount := defaultPlayerManager.sessionCount()
		dCount := len(manager.desks)
		async.Run(func() {
			db.InsertOnline(sCount, dCount)
		})
	})
}

func (manager *DeskManager) dumpDeskInfo() {
	c := len(manager.desks)
	if c < 1 {
		return
	}

	logger.Infof("剩余房间数量: %d 在线人数: %d  当前时间: %s", c, defaultPlayerManager.sessionCount(), time.Now().Format("2006-01-02 15:04:05"))
	for no, d := range manager.desks {
		logger.Debugf("房号: %s, 创建时间: %s, 创建玩家: %d, 状态: %d:", no, time.Unix(d.createdAt, 0).String(), d.creator, d.status())
	}
}

// 玩家网络断开
func (manager *DeskManager) onPlayerDisconnect(s *session.Session) error {
	uid := s.UID()
	p, err := playerWithSession(s)
	if err != nil {
		return err
	}
	p.logger.Debug("DeskManager.onPlayerDisconnect: 玩家网络断开")

	// 移除session
	p.removeSession()

	if p.desk == nil || p.desk.isDestroy() {
		defaultPlayerManager.offline(uid)
		return nil
	}

	d := p.desk
	d.onPlayerExit(s, true)
	return nil
}

// 根据桌号返回牌桌数据
func (manager *DeskManager) desk(number RoomNumber) (*Desk, bool) {
	d, ok := manager.desks[number]
	return d, ok
}

// 设置桌号对应的牌桌数据
func (manager *DeskManager) setDesk(number RoomNumber, desk *Desk) {
	if desk == nil {
		delete(manager.desks, number)
		logger.WithField(fieldDesk, number).Debugf("清除房间: 剩余: %d", len(manager.desks))
	} else {
		manager.desks[number] = desk
	}
}

// 检查登录玩家关闭应用之前是否正在游戏
func (manager *DeskManager) UnCompleteDesk(s *session.Session, _ []byte) error {
	resp := &protocol.UnCompleteDeskResponse{}

	p, err := playerWithSession(s)
	if err != nil {
		return nil
	}

	if p.desk == nil {
		p.logger.Debug("DeskManager.UnCompleteDesk: 玩家不在房间内")
		return s.Response(resp)
	}

	d := p.desk
	if d.isDestroy() {
		delete(manager.desks, d.roomNo)
		p.desk = nil
		p.logger.Debug("DeskManager.UnCompleteDesk: 房间已销毁")
		return s.Response(resp)
	}

	return s.Response(&protocol.UnCompleteDeskResponse{
		Exist: true,
		TableInfo: protocol.TableInfo{
			DeskNo:    string(d.roomNo),
			CreatedAt: d.createdAt,
			Creator:   d.creator,
			Title:     d.title(),
			Desc:      d.desc(true),
			Status:    d.status(),
			Mode:      d.opts.Mode,
		},
	})
}

// 网络断开后, 重新连接网络
func (manager *DeskManager) ReConnect(s *session.Session, req *protocol.ReConnect) error {
	uid := req.Uid

	// 绑定UID
	if err := s.Bind(uid); err != nil {
		return err
	}

	logger.Infof("玩家重新连接服务器: UID=%d", uid)

	// 设置用户
	p, ok := defaultPlayerManager.player(uid)
	if !ok {
		ip := ""
		if parts := strings.Split(s.RemoteAddr().String(), ":"); len(parts) > 0 {
			ip = parts[0]
		}
		logger.Infof("玩家之前用户信息已被清除，重新初始化用户信息: UID=%d ip=%s", uid, ip)

		p = newPlayer(s, uid, req.Name, req.HeadUrl, req.Sex)
		defaultPlayerManager.setPlayer(uid, p)
	} else {
		logger.Infof("玩家之前用户信息存在服务器上，替换session: UID=%d", uid)

		// 重置之前的session
		prevSession := p.session
		if prevSession != nil {
			prevSession.Clear()
			prevSession.Close()
		}

		// 绑定新session
		p.bindSession(s)

		// 移除广播频道
		if d := p.desk; d != nil && prevSession != nil {
			d.group.Leave(prevSession)
		}
	}

	return nil
}

// 网络断开后, 如果ReConnect后发现当前正在房间中, 则重新进入, 桌号是之前的桌号
func (manager *DeskManager) ReJoin(s *session.Session, data *protocol.ReJoinDeskRequest) error {
	d, ok := manager.desk(RoomNumber(data.DeskNo))
	if !ok || d.isDestroy() {
		return s.Response(&protocol.ReJoinDeskResponse{
			Code:  -1,
			Error: "房间已解散",
		})
	}
	d.logger.Debugf("玩家重新加入房间: UID=%d, Data=%+v", s.UID(), data)

	return d.onPlayerReJoin(s)
}

// 应用退出后重新进入房间
func (manager *DeskManager) ReEnter(s *session.Session, msg *protocol.ReEnterDeskRequest) error {
	p, err := playerWithSession(s)
	if err != nil {
		logger.Errorf("玩家重新进入房间: UID=%d", s.UID())
		return nil
	}

	if p.desk == nil {
		p.logger.Debugf("玩家没有未完成房间，但是发送了重进请求: 请求房号: %s", msg.DeskNo)
		return nil
	}

	d := p.desk

	if string(d.roomNo) != msg.DeskNo {
		p.logger.Debugf("玩家正在试图进入非上次未完成房间: 房号: %s", d.roomNo)
		return nil
	}

	return d.onPlayerReJoin(s)
}

// 玩家切换到后台
func (manager *DeskManager) Pause(s *session.Session, _ []byte) error {
	p, err := playerWithSession(s)
	if err != nil {
		return err
	}

	d := p.desk
	if d == nil {
		p.logger.Debug("玩家不在房间内")
		return nil
	}

	p.logger.Debug("玩家切换到后台")

	return nil
}

// 玩家切换到前台
func (manager *DeskManager) Resume(s *session.Session, _ []byte) error {
	p, err := playerWithSession(s)
	if err != nil {
		return err
	}

	d := p.desk
	if d == nil {
		p.logger.Debug("玩家不在房间内")
		return nil
	}

	p.logger.Debug("玩家切换到前台")

	// 人数不够, 未开局, 或没有人申请解散
	if len(d.players) < d.totalPlayerCount() {
		return nil
	}

	return nil
}

//---------------------------------------------------------------------------------------------------

//创建一张桌子
func (manager *DeskManager) CreateDesk(s *session.Session, data *protocol.CreateDeskRequest) error {
	p, err := playerWithSession(s)
	if err != nil {
		return err
	}

	//桌子存在玩家在游戏中
	if p.desk != nil {
		return s.Response(reentryDesk)
	}

	//强制更新
	if forceUpdate && data.Version != version {
		return s.Response(createVersionExpire)
	}

	logger.Infof("牌桌选项: %#v  %d", data.DeskOpts, data.DeskOpts.Mode)

	if !verifyOptions(data.DeskOpts) {
		return errutil.ErrIllegalParameter
	}

	//消耗金币
	count := requireCardCount(data.DeskOpts.Mode)
	logger.Infof("used coin %d, player has coin:%d", count, p.coin)
	if p.coin < int64(count) {
		return s.Response(deskCardNotEnough)
	}

	//创建房间
	no := room.Next()
	d := NewDesk(RoomNumber(no), data.DeskOpts)
	d.createdAt = time.Now().Unix()
	d.creator = s.UID()
	d.gameCtx.Init()

	//房间创建者自动join
	if err := d.playerJoin(s, false); err != nil {
		return nil
	}

	manager.desks[d.roomNo] = d // save desk information

	resp := &protocol.CreateDeskResponse{
		Code: SUC,
		TableInfo: protocol.TableInfo{
			DeskNo:    string(no),
			CreatedAt: d.createdAt,
			Creator:   s.UID(),
			Title:     d.title(),
			Desc:      d.desc(true),
			Status:    d.status(),
			Mode:      d.opts.Mode,
		},
	}
	d.logger.Infof("当前已有牌桌数: %d", len(manager.desks))
	return s.Response(resp)
}

//新join在session的context中尚未有desk的cache
func (manager *DeskManager) Join(s *session.Session, data *protocol.JoinDeskRequest) error {
	_, err := playerWithSession(s)
	if err != nil {
		return nil
	}

	if forceUpdate && data.Version != version {
		return s.Response(joinVersionExpire)
	}

	dn := RoomNumber(data.DeskNo)
	d, ok := manager.desk(dn)
	if !ok {
		return s.Response(deskNotFoundResponse)
	}

	if len(d.players) >= d.totalPlayerCount() {
		return s.Response(deskPlayerNumEnough)
	}

	if err := d.playerJoin(s, false); err != nil {
		d.logger.Errorf("玩家加入房间失败，UID=%d, Error=%s", s.UID(), err.Error())
	}

	return s.Response(&protocol.JoinDeskResponse{
		Code: SUC,
		TableInfo: protocol.TableInfo{
			DeskNo:    string(d.roomNo),
			CreatedAt: d.createdAt,
			Creator:   d.creator,
			Title:     d.title(),
			Desc:      d.desc(true),
			Status:    d.status(),
			Mode:      d.opts.Mode,
		},
	})
}

// Exit 处理玩家退出, 客户端会在房间人没有满的情况下可以退出解散房间，一旦加入房间就不能手动离开
func (manager *DeskManager) Exit(s *session.Session, msg *protocol.ExitRequest) error {
	p, err := playerWithSession(s)
	if err != nil {
		return nil
	}
	p.logger.Debugf("DeskManager.Exit: %+v", msg)
	d := p.desk
	if d == nil || d.isDestroy() {
		p.logger.Debug("玩家不在房间内")
		s.Response(&protocol.ExitResponse{Code: ePlayerNotInDesk})
		return nil
	}

	if d.status() != DeskStatusCreate {
		p.logger.Debug("房间已经开始，中途不能退出")
		s.Response(&protocol.ExitResponse{Code: FAIL})
		return nil
	}

	s.Response(&protocol.ExitResponse{Code: SUC})
	d.onPlayerExit(s, false)

	return nil
}

// 玩家准备
func (manager *DeskManager) Ready(s *session.Session, _ []byte) error {
	p, err := playerWithSession(s)
	if err != nil {
		return err
	}

	d := p.desk
	d.prepare.ready(s.UID())
	d.syncDeskStatus()

	// 必须在广播消息以后调用checkStart
	d.checkStart()
	return err
}

// 理牌结束
func (manager *DeskManager) QiPaiFinished(s *session.Session, msg []byte) error {
	p, err := playerWithSession(s)
	if err != nil {
		return err
	}

	d := p.desk
	if d == nil {
		p.logger.Debug("玩家不在房间内")
		return nil
	}

	return d.qiPaiFinished(s.UID())
}

// 投降
func (manager *DeskManager) GiveUp(s *session.Session, msg *protocol.GiveupRequest) error {
	p, err := playerWithSession(s)
	if err != nil {
		return err
	}

	d := p.desk
	if d == nil || d.isDestroy() {
		logger.Infof("玩家: %d申请投降，但是房间为空或者已解散", s.UID())
		return nil
	}

	d.onGiveUp(s)

	return nil
}

// 翻牌
func (manager *DeskManager) OpenPiece(s *session.Session, msg *protocol.PieceOpenRequest) error {
	p, err := playerWithSession(s)
	if err != nil {
		return err
	}

	d := p.desk
	if d == nil || d.isDestroy() {
		logger.Infof("牌桌已经解散")
		return nil
	}

	return d.onOpenPiece(s, msg.Index)
}

// 移动
func (manager *DeskManager) MovePiece(s *session.Session, msg *protocol.PieceMoveRequest) error {
	p, err := playerWithSession(s)
	if err != nil {
		return nil
	}

	d := p.desk
	if d == nil || d.isDestroy() {
		logger.Infof("牌桌已经解散")
		return nil
	}

	return d.onMovePiece(s, msg)
}

// 吃牌
func (manager *DeskManager) EatPiece(s *session.Session, msg *protocol.PieceEatRequest) error {
	p, err := playerWithSession(s)
	if err != nil {
		return err
	}

	d := p.desk
	if d == nil || d.isDestroy() {
		logger.Infof("牌桌已经解散")
		return nil
	}

	return d.onEatPiece(s, msg)
}

// 表情
func (manager *DeskManager) ShowEnjoy(s *session.Session, msg *protocol.PlayEjoyReq) error {
	p, err := playerWithSession(s)
	if err != nil {
		return nil
	}

	d := p.desk
	if d == nil || d.isDestroy() {
		logger.Infof("牌桌已经解散")
		return nil
	}

	return d.onShowEnjoy(s, msg)
}
