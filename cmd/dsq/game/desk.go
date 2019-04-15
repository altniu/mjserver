package game

import (
	"fmt"
	"math/rand"
	"strings"
	"sync/atomic"
	"time"

	"github.com/lonng/nano"
	"github.com/lonng/nano/session"
	"github.com/lonng/nanoserver/cmd/dsq/db"
	"github.com/lonng/nanoserver/cmd/dsq/db/model"
	"github.com/lonng/nanoserver/cmd/dsq/game/dsq"
	"github.com/lonng/nanoserver/cmd/dsq/protocol"
	"github.com/lonng/nanoserver/pkg/async"
	"github.com/lonng/nanoserver/pkg/errutil"
	"github.com/pborman/uuid"
	log "github.com/sirupsen/logrus"
)

type Desk struct {
	group       *nano.Group // 组播通道
	roomNo      RoomNumber  // 房间号
	deskID      int64       // desk表的pk
	state       int32       // 状态
	creator     int64       // 创建玩家UID
	createdAt   int64       // 创建时间
	players     []*Player   // 所有玩家
	lastHintUid int64       // 最后一次提示的玩家
	die         chan struct{}
	curCamp     int                       //当前方位 1 2 红蓝
	gameCtx     *dsq.Dsq                  //游戏上下文
	prepare     *prepareContext           //准备相关状态
	opts        *protocol.DeskOptions     //房间选项
	latestEnter *protocol.PlayerEnterDesk //最新的进入状态
	logger      *log.Entry
}

func NewDesk(roomNo RoomNumber, opts *protocol.DeskOptions) *Desk {
	d := &Desk{
		group:   nano.NewGroup(uuid.New()),
		roomNo:  roomNo,
		state:   DeskStatusCreate,
		players: []*Player{},
		die:     make(chan struct{}),
		gameCtx: dsq.NewDsq(),
		curCamp: 0,
		prepare: newPrepareContext(),
		logger:  log.WithField(fieldDesk, roomNo),
		opts:    opts,
	}
	return d
}

func (d *Desk) totalPlayerCount() int {
	return playerMax
}

func (d *Desk) title() string {
	return strings.TrimSpace(fmt.Sprintf("房号: %s", d.roomNo))
}

func (d *Desk) playerWithId(uid int64) (*Player, error) {
	for _, p := range d.players {
		if p.Uid() == uid {
			return p, nil
		}
	}
	return nil, errutil.ErrPlayerNotFound
}

func (d *Desk) desc(detail bool) string {
	desc := []string{}
	name := "斗兽棋"
	desc = append(desc, name)
	if detail {
		opts := d.opts
		if opts.Mode == ModeRoom {
			desc = append(desc, "房间模式")
		} else {
			desc = append(desc, "比赛模式")
		}
	}
	return strings.Join(desc, " ")
}

func (d *Desk) nextTurn() {
	d.curCamp = d.curCamp%d.totalPlayerCount() + 1
}

func (d *Desk) currentPlayer() *Player {
	return d.players[d.curCamp-1]
}

//持续化Desk
func (d *Desk) save() error {
	// save to database
	desk := &model.Desk{
		Creator:     d.creator,
		CreatedAt:   time.Now().Unix(),
		DeskNo:      string(d.roomNo),
		Player0:     d.players[0].Uid(),
		Player1:     d.players[1].Uid(),
		PlayerName0: d.players[0].name,
		PlayerName1: d.players[1].name,
	}
	d.logger.Infof("保存房间数据, 创建时间: %d", desk.CreatedAt)

	var onDBResult = func(desk *model.Desk) {
		d.deskID = desk.Id
	}

	go func() {
		if err := db.UpdateDesk(desk); err == nil {
			nano.Invoke(func() { onDBResult(desk) })
		} else {
			log.Error(err)
		}
	}()
	return nil
}

func (d *Desk) syncDeskStatus() {
	d.latestEnter = &protocol.PlayerEnterDesk{Data: []protocol.EnterDeskInfo{}}
	for i, p := range d.players {
		uid := p.Uid()
		d.latestEnter.Data = append(d.latestEnter.Data, protocol.EnterDeskInfo{
			DeskPos:  i,
			Uid:      uid,
			Nickname: p.name,
			IsReady:  d.prepare.isReady(uid),
			Sex:      p.sex,
			IsExit:   false,
			HeadUrl:  p.head,
			IP:       p.ip,
		})
	}
	d.group.Broadcast("onPlayerEnter", d.latestEnter)
}

// 如果是重新进入 isReJoin: true
func (d *Desk) playerJoin(s *session.Session, isReJoin bool) error {
	uid := s.UID()
	var (
		p   *Player
		err error
	)

	if isReJoin {
		p, err = d.playerWithId(uid)
		if err != nil {
			d.logger.Errorf("玩家: %d重新加入房间, 但是没有找到玩家在房间中的数据", uid)
			return err
		}
		d.group.Add(s)
	} else {
		exists := false
		for _, p := range d.players {
			if p.Uid() == uid {
				exists = true
				p.logger.Warn("玩家已经在房间中")
				break
			}
		}
		if !exists {
			p = s.Value(fieldPlayer).(*Player)
			d.players = append(d.players, p)
			for _, p := range d.players {
				p.setDesk(d)
			}
		}
	}

	return nil
}

func (d *Desk) checkStart() {
	s := d.status()
	if (s != DeskStatusCreate) && (s != DeskStatusCleaned) {
		d.logger.Infof("当前房间状态不对，不能开始游戏，当前状态=%s", stringify[s])
		return
	}

	if count, num := len(d.players), d.totalPlayerCount(); count < num {
		d.logger.Infof("当前房间玩家数量不足，不能开始游戏，当前玩家=%d, 最低数量=%d", count, num)
		return
	}
	for _, p := range d.players { /**/
		if uid := p.Uid(); !d.prepare.isReady(uid) {
			p.logger.Info("玩家未准备")
			return
		}
	}

	d.start()
}

// 牌桌开始, 此方法只在开桌时执行
func (d *Desk) start() {
	d.setStatus(DeskStatusDuanPai)              //发牌
	var totalPlayerCount = d.totalPlayerCount() // 玩家数量

	//随机出0、1 0则第一个玩家是红方 1则第二个玩家是红方 红方先开始
	d.curCamp = rand.Intn(totalPlayerCount)

	if d.curCamp == 1 {
		d.players[0].camp = 1 //红方
		d.players[1].camp = 2
	} else {
		d.players[0].camp = 2
		d.players[1].camp = 1 //红方
	}

	d.gameCtx.Ready()
	d.logger.Debugf("玩家数量=%d, 所有麻将=%v", totalPlayerCount)
	d.logger.Debugf("游戏开局, 麻将数量=%d 所有麻将: %v", len(d.gameCtx.Origin), d.gameCtx.Origin)

	duan := &protocol.DuanPai{Pieces: d.gameCtx.Current, Camps: []protocol.CampInfo{}}
	for _, p := range d.players {
		duan.Camps = append(duan.Camps, protocol.CampInfo{Uid: p.Uid(), Camp: p.camp})
	}
	d.group.Broadcast("onDuanPai", duan)
}

// 理牌状态之后开始游戏
func (d *Desk) qiPaiFinished(uid int64) error {
	if d.status() > DeskStatusDuanPai {
		d.logger.Debugf("当前牌桌状态: %s", stringify[d.status()])
		return errutil.ErrIllegalDeskStatus
	}

	d.prepare.sorted(uid)

	//等待所有人齐牌
	for _, p := range d.players {
		if !d.prepare.isSorted(p.Uid()) {
			return nil
		}
	}

	d.setStatus(DeskStatusQiPai)

	go d.play()

	return nil
}

//表示本局结束
func (d *Desk) isRoundOver() bool {
	s := d.status()
	if s == DeskStatusInterruption || s == DeskStatusDestory {
		return true
	}
	return false
}

// 循环中的核心逻辑
func (d *Desk) play() {
	defer func() {
		if err := recover(); err != nil {
			d.logger.Errorf("Error=%v", err)
			println(stack())
		}
	}()

	d.setStatus(DeskStatusPlaying)
	d.logger.Debug("开始游戏")

	var (
		playerOp int = opNull
		campWin      = -1
		bOver        = false
		bTimeOut     = false
		bGiveup      = false
	)

	for !d.isRoundOver() {
		//提示当前玩家操作

		curPlayer := d.currentPlayer()
		d.hint(curPlayer)

		//等待当前玩家操作
		select {
		case op, ok := <-curPlayer.chOperation:
			if !ok {
				return
			}
			playerOp = op.OpType
			d.logger.Debug(stringOpType[op.OpType])
		case <-d.die:
			return
		case <-time.After(30 * time.Second):
			bTimeOut = true
		}

		//是否超时
		if bTimeOut {
			campWin = curPlayer.camp%d.totalPlayerCount() + 1
			d.setStatus(DeskStatusInterruption)
			break
		}

		//判断玩家的操作
		if playerOp == opGiveup {
			bGiveup = true
			campWin = curPlayer.camp%d.totalPlayerCount() + 1
			d.setStatus(DeskStatusInterruption)
			break
		} else if playerOp == opEat {
			bOver, campWin = d.gameCtx.CheckOver() //true 结束 0:平局 1:A 阵营 2:B阵营
			if bOver {
				d.setStatus(DeskStatusInterruption)
				break
			}
		}

		//没有结束则下一个回合
		d.nextTurn()
	}

	if d.status() != DeskStatusInterruption {
		d.logger.Info("没有完局")
		return
	}

	d.roundOver(campWin, bGiveup, bTimeOut)
}

func (d *Desk) setStatus(s int32) {
	atomic.StoreInt32((*int32)(&d.state), s)
}

func (d *Desk) status() int32 {
	return atomic.LoadInt32((*int32)(&d.state))
}

func (d *Desk) clean() {
	d.state = DeskStatusCleaned
	d.prepare.reset()
	d.gameCtx.Init()

	for _, p := range d.players {
		p.reset()
	}
}

//提示玩家操作
func (d *Desk) hint(p *Player) {
	msg := &protocol.RoundInfo{Uid: p.Uid(), Camp: p.camp, TimeStamp: time.Now().Unix()}
	d.lastHintUid = p.Uid()
	d.logger.Debugf("玩家最后提示: Hint=%+v", msg)
	d.group.Broadcast("onHintPlayer", msg)
}

//赢家 0:平局 1:A 阵营 2:B阵营 放弃 超时
func (d *Desk) roundOver(winCamp int, bGiveup bool, bTimeOut bool) {
	var uid int64 = 0
	if winCamp != 0 {
		uid = d.players[winCamp-1].Uid()
	}
	msg := &protocol.GameResult{Winner: uid, Coin: 10, Camp: winCamp, Giveup: bGiveup, TimeOut: bTimeOut}
	d.gameResult(msg)
}

// 结算
func (d *Desk) gameResult(msg *protocol.GameResult) {
	d.logger.Debugf("本场游戏结束结算数据: %#v", msg)

	//发送单场统计
	err := d.group.Broadcast("onGameEnd", msg)
	if err != nil {
		log.Error(err)
	}

	d.destroy()

	async.Run(func() {
		//if err = db.UpdateDesk(desk); err != nil {
		//log.Error(err)
		//}
	})
}

func (d *Desk) isDestroy() bool {
	return d.status() == DeskStatusDestory
}

// 摧毁桌子
func (d *Desk) destroy() {
	if d.status() == DeskStatusDestory {
		d.logger.Info("桌子已经解散")
		return
	}

	close(d.die)

	// 标记为销毁
	d.setStatus(DeskStatusDestory)

	d.logger.Info("销毁房间")
	for i := range d.players {
		p := d.players[i]
		d.logger.Debugf("销毁房间，清除玩家%d数据", p.Uid())
		p.reset()
		p.desk = nil
		p.camp = 0
		p.logger = log.WithField(fieldPlayer, p.uid)
		d.players[i] = nil
	}

	// 释放desk资源
	d.group.Close()
	d.prepare.reset()

	//删除桌子
	nano.Invoke(func() {
		defaultDeskManager.setDesk(d.roomNo, nil)
	})
}

//玩家离开房在游戏开始之前
func (d *Desk) onPlayerExit(s *session.Session, isDisconnect bool) {
	uid := s.UID()
	d.group.Leave(s)

	restPlayers := []*Player{}
	for _, p := range d.players {
		if p.Uid() != uid {
			restPlayers = append(restPlayers, p)
		} else {
			p.reset()
			p.desk = nil
			p.camp = 0
		}
	}
	d.players = restPlayers

	if len(d.players) == 0 && !isDisconnect {
		d.logger.Info("所有玩家下线")
		d.destroy()

		//数据库异步更新
		async.Run(func() {
			desk := &model.Desk{
				Id: d.deskID,
			}
			if err := db.UpdateDesk(desk); err != nil {
				log.Error(err)
			}
		})
	}
}

// 网络断开后, 如果ReConnect后发现当前正在房间中, 则或者应用退出后重新进入, 桌号是之前的桌号
func (d *Desk) onPlayerReJoin(s *session.Session) error {
	// 同步房间基本信息
	basic := &protocol.DeskBasicInfo{
		DeskID: string(d.roomNo),
		Title:  d.title(),
		Desc:   d.desc(true),
	}
	if err := s.Push("onDeskBasicInfo", basic); err != nil {
		log.Error(err.Error())
		return err
	}

	// 同步所有玩家数据
	enter := &protocol.PlayerEnterDesk{Data: []protocol.EnterDeskInfo{}}
	for i, p := range d.players {
		uid := p.Uid()
		enter.Data = append(enter.Data, protocol.EnterDeskInfo{
			DeskPos:  i,
			Uid:      uid,
			Nickname: p.name,
			IsReady:  d.prepare.isReady(uid),
			Sex:      p.sex,
			IsExit:   false,
			HeadUrl:  p.head,
			IP:       p.ip,
		})
	}
	if err := s.Push("onPlayerEnter", enter); err != nil {
		log.Error(err.Error())
		return err
	}

	_, err := playerWithSession(s)
	if err != nil {
		log.Error(err)
		return err
	}

	if err := d.playerJoin(s, true); err != nil {
		log.Error(err)
	}

	d.prepare.ready(s.UID())
	d.syncDeskStatus()
	d.checkStart()

	return nil
}

func (d *Desk) loseCoin() {
	cardCount := requireCardCount(d.opts.Mode)
	p, err := d.playerWithId(d.creator)
	if err != nil {
		d.logger.Errorf("扣除玩家房卡错误，没有找到玩家，CreatorID=%d", d.creator)
		return
	}
	p.loseCoin(int64(cardCount))
}

//认输
func (d *Desk) onGiveUp(s *session.Session) error {
	if d.status() != DeskStatusPlaying {
		d.logger.Debug("当前非游戏状态")
		return nil
	}

	p, err := playerWithSession(s)
	if err != nil {
		d.logger.Debug("玩家为空")
		return nil
	}

	//状态检测
	if d.status() != DeskStatusPlaying {
		d.logger.Debug("当前非游戏状态")
		return nil
	}

	//回合检测
	if p.camp != d.curCamp {
		d.logger.Errorf("不是此玩家的回合")
		return nil
	}

	//翻牌通知
	p.chOperation <- &protocol.OpChoosed{
		OpType: opGiveup,
	}
	return nil
}

//翻牌
func (d *Desk) onOpenPiece(s *session.Session, index int) error {
	p, err := playerWithSession(s)
	if err != nil {
		d.logger.Debug("玩家为空")
		return nil
	}

	//状态检测
	if d.status() != DeskStatusPlaying {
		d.logger.Debug("当前非游戏状态")
		return nil
	}

	//回合检测
	if p.camp != d.curCamp {
		d.logger.Errorf("不是此玩家的回合")
		return nil
	}

	//翻牌
	piece := d.gameCtx.Open(index)
	if piece == 0 {
		return nil
	}

	//翻牌相应
	s.Response(&protocol.PieceOpenResponse{Code: 0, Index: index, Piece: piece})

	//翻牌广播
	d.group.Broadcast("onOpenPiece", &protocol.PieceOpenNotify{Uid: s.UID(), Index: index, Piece: piece})

	//翻牌通知
	p.chOperation <- &protocol.OpChoosed{
		OpType: opOpen,
	}

	return nil
}

//移动
func (d *Desk) onMovePiece(s *session.Session, msg *protocol.PieceMoveRequest) error {
	p, err := playerWithSession(s)
	if err != nil {
		d.logger.Debug("玩家为空")
		return nil
	}

	//状态检测
	if d.status() != DeskStatusPlaying {
		d.logger.Debug("当前非游戏状态")
		return nil
	}

	//回合检测
	if p.camp != d.curCamp {
		d.logger.Errorf("不是此玩家的回合")
		return nil
	}

	if d.gameCtx.GetCamp(msg.IndexSrc) != p.camp {
		d.logger.Errorf("不是自己的棋子")
		return nil
	}

	//移动
	bMove := d.gameCtx.Move(msg.IndexSrc, msg.IndexDest)

	ret := 1
	if bMove {
		ret = 0
	}

	//响应广播操作
	s.Response(&protocol.PieceMoveResponse{Code: ret, Pieces: d.gameCtx.Current})
	if bMove {
		d.group.Broadcast("onMovePiece", &protocol.PieceMoveNotify{Uid: s.UID(), IndexSrc: msg.IndexSrc, IndexDest: msg.IndexDest})
	}

	//移动通知
	if bMove {
		p.chOperation <- &protocol.OpChoosed{
			OpType: opMove,
		}
	}

	return nil
}

//吃牌
func (d *Desk) onEatPiece(s *session.Session, msg *protocol.PieceEatRequest) error {
	p, err := playerWithSession(s)
	if err != nil {
		d.logger.Debug("玩家为空")
		return nil
	}

	if d.status() != DeskStatusPlaying {
		d.logger.Debug("当前非游戏状态")
		return nil
	}

	//回合检测
	if p.camp != d.curCamp {
		d.logger.Errorf("不是此玩家的回合")
		return nil
	}

	if d.gameCtx.GetCamp(msg.IndexSrc) != p.camp {
		d.logger.Errorf("不是自己的棋子")
		return nil
	}

	ret := d.gameCtx.Eat(msg.IndexSrc, msg.IndexDest)
	//1吃 2被吃 3同归 0失败

	s.Response(&protocol.PieceEatResponse{Code: ret, Pieces: d.gameCtx.Current})
	if ret != 0 {
		d.group.Broadcast("onEatPiece", &protocol.PieceEatNotify{Uid: s.UID(), Code: ret, IndexSrc: msg.IndexSrc, IndexDest: msg.IndexDest})
	}

	//吃牌通知
	if ret != 0 {
		p.chOperation <- &protocol.OpChoosed{
			OpType: opEat,
		}
	}

	return nil
}

//表情
func (d *Desk) onShowEnjoy(s *session.Session, msg *protocol.PlayEjoyReq) error {
	if d.status() != DeskStatusPlaying {
		d.logger.Debug("当前非游戏状态")
		return nil
	}
	d.group.Broadcast("onShowEnjoy", &protocol.PlayEjoyNotify{Uid: s.UID(), Index: msg.Index})
	return nil
}
