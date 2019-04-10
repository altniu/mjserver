package game

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"sort"
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
	"github.com/lonng/nanoserver/pkg/room"
	"github.com/pborman/uuid"
	log "github.com/sirupsen/logrus"
)

const (
	illegalTurn   = -1
	ResultIllegal = 0
)

type Desk struct {
	group     *nano.Group // 组播通道
	roomNo    RoomNumber  // 房间号
	deskID    int64       // desk表的pk
	state     DeskStatus  // 状态
	creator   int64       // 创建玩家UID
	createdAt int64       // 创建时间
	players   []*Player   // 所有玩家

	die chan struct{}

	gameCtx *dsq.Dsq              //游戏上下文
	curTurn int                   //当前方位 1 2
	prepare *prepareContext       //准备相关状态
	dice    *dice                 //骰子
	opts    *protocol.DeskOptions //房间选项

	latestEnter *protocol.PlayerEnterDesk //最新的进入状态

	logger *log.Entry
}

func NewDesk(roomNo RoomNumber, opts *protocol.DeskOptions) *Desk {
	d := &Desk{
		group:   nano.NewGroup(uuid.New()),
		roomNo:  roomNo,
		state:   DeskStatusCreate,
		players: []*Player{},
		die:     make(chan struct{}),
		gameCtx: dsq.NewDsq(),
		curTurn: -1,
		prepare: newPrepareContext(),
		dice:    newDice(),
		logger:  log.WithField(fieldDesk, roomNo),
		opts:    opts,
	}
	return d
}

// 玩家数量
func (d *Desk) totalPlayerCount() int {
	return 2
}

// 棋子数量
func (d *Desk) totalTileCount() int {
	return 16
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

	// TODO: 改成异步
	if err := db.InsertDesk(desk); err != nil {
		return err
	}

	d.deskID = desk.Id
	return nil
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
			p = s.Value(kCurPlayer).(*Player)
			d.players = append(d.players, p)
			for i, p := range d.players {
				p.setDesk(d, i)
			}
		}
	}

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
			Score:    p.score,
			IP:       p.ip,
		})
	}
	d.group.Broadcast("onPlayerEnter", d.latestEnter)
}

func (d *Desk) checkStart() {
	s := d.status()
	if (s != DeskStatusCreate) && (s != DeskStatusCleaned) {
		d.logger.Infof("当前房间状态不对，不能开始游戏，当前状态=%s", s.String())
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

func (d *Desk) title() string {
	return strings.TrimSpace(fmt.Sprintf("房号: %s", d.roomNo))
}

// 描述, 参数表示是否显示额外选项
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

// 牌桌开始, 此方法只在开桌时执行, 非并行
func (d *Desk) start() {
	d.setStatus(DeskStatusDuanPai) //发牌

	var (
		totalPlayerCount = d.totalPlayerCount() // 玩家数量
		totalTileCount   = d.totalTileCount()   // 麻将数量
	)

	d.curTurn = rand.Intn(totalPlayerCount)

	//桌面基本信息
	basic := &protocol.DeskBasicInfo{
		DeskID: d.roomNo.String(),
		Title:  d.title(),
		Desc:   d.desc(true),
		Mode:   d.opts.Mode,
	}

	d.group.Broadcast("onDeskBasicInfo", basic)
	d.gameCtx.Ready()
	d.logger.Debugf("玩家数量=%d, 所有麻将=%v", totalPlayerCount)
	d.logger.Debugf("游戏开局, 麻将数量=%d 所有麻将: %v", len(d.gameCtx.Origin), d.gameCtx.Origin)

	for turn, player := range d.players {
		//player.duanPai(info[turn].OnHand)
	}

	//摇色子确定红蓝双方
	d.dice.random()
	d.dice.dice1

	duan := &protocol.DuanPai{}
	d.group.Broadcast("onDuanPai", duan)
}

// 齐牌状态之后开始游戏
func (d *Desk) qiPaiFinished(uid int64) error {
	if d.status() > DeskStatusDuanPai {
		d.logger.Debugf("当前牌桌状态: %s", d.status().String())
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

func (d *Desk) nextTurn() {
	d.curTurn++
	d.curTurn = d.curTurn % d.totalPlayerCount()
}

func (d *Desk) isRoundOver() bool {
	//中/终断表示本局结束
	s := d.status()
	if s == DeskStatusInterruption || s == DeskStatusDestory {
		return true
	}

	if d.noMoreTile() {
		return true
	}

	//只剩下一个人没有和牌结算
	return len(d.wonPlayers) == d.totalPlayerCount()-1
}

func (d *Desk) currentPlayer() *Player {
	return d.players[d.curTurn]
}

// 循环中的核心逻辑
// 1. 摸牌
// 2. 打牌
// 3. 检查是否输赢
func (d *Desk) play() {
	defer func() {
		if err := recover(); err != nil {
			d.logger.Errorf("Error=%v", err)
			println(stack())
		}
	}()

	d.setStatus(DeskStatusPlaying)
	d.logger.Debug("开始游戏")

	curPlayer := d.players[d.curTurn] //当前出牌玩家,初始为随机

MAIN_LOOP:
	for !d.isRoundOver() {
		// 切换到下一个玩家
		d.nextTurn()
		curPlayer = d.currentPlayer()
	}

	if d.status() == DeskStatusDestory {
		d.logger.Info("已经销毁(二人都离线或解散)")
		return
	}

	if d.status() != DeskStatusInterruption {
		d.setStatus(DeskStatusRoundOver)
	}

	d.roundOver()
}

func (d *Desk) roundOverTilesForPlayer(p *Player) *protocol.HandTilesInfo {
	uid := p.Uid()
	ids := p.handTiles().Ids()
	sps := []int{}

	// fixed: 将胡牌从手牌中移除
	winTileID := -1
	if d.wonPlayers[uid] {
		winTileID = p.ctx.WinningID
		for _, id := range ids {
			if id == winTileID {
				continue
			}
			sps = append(sps, id)
		}
	} else {
		sps = make([]int, len(ids))
		copy(sps, ids)
	}

	// 手牌
	tiles := &protocol.HandTilesInfo{
		Uid:    uid,
		Tiles:  sps,
		HuPai:  winTileID,
		IsTing: d.wonPlayers[uid] || p.isTing(),
	}

	return tiles
}

func (d *Desk) setStatus(s DeskStatus) {
	atomic.StoreInt32((*int32)(&d.state), int32(s))
}

func (d *Desk) status() DeskStatus {
	return DeskStatus(atomic.LoadInt32((*int32)(&d.state)))
}

func (d *Desk) roundOver() {
	stats := d.roundOverHelper()
	status := d.status()
	d.finalSettlement(stats)
}

func (d *Desk) clean() {
	d.state = DeskStatusCleaned
	d.isNewRound = true
	d.knownTiles = map[int]int{}
	d.allTiles = mahjong.Mahjong{}
	d.prepare.reset()

	//重置玩家状态
	for _, p := range d.players {
		d.roundStats[p.Uid()] = &history.Record{}
		p.reset()
	}
}

func (d *Desk) finalSettlement(isNormalFinished bool, ge *protocol.RoundOverStats) {
	d.logger.Debugf("本场游戏结束, 最后一局结算数据: %#v", ge)

	//发送单场统计
	err := d.group.Broadcast("onGameEnd", ddr)
	if err != nil {
		log.Error(err)
	}

	//桌子解散,更新桌面信息
	desk := &model.Desk{
		Id:      d.deskID,
		Creator: d.creator,
		DeskNo:  d.roomNo.String(),
	}

	for i := range d.players {
		p := d.players[i]
		uid := p.Uid()
		score := 0
		if r, ok := stats[uid]; ok {
			score = r.TotalScore
		}
		switch i {
		case 0:
			desk.Player0, desk.ScoreChange0, desk.PlayerName0 = uid, score, p.name
		case 1:
			desk.Player1, desk.ScoreChange1, desk.PlayerName1 = uid, score, p.name
		}
	}

	d.destroy()

	// 数据库异步更新
	async.Run(func() {
		if err = db.UpdateDesk(desk); err != nil {
			log.Error(err)
		}
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
		p.score = 1000
		p.turn = 0
		p.logger = log.WithField(fieldPlayer, p.uid)
		d.players[i] = nil
	}

	// 释放desk资源
	d.group.Close()
	d.prepare.reset()
	d.knownTiles = nil

	//删除桌子
	nano.Invoke(func() {
		defaultDeskManager.setDesk(d.roomNo, nil)
	})
}

func (d *Desk) onPlayerExit(s *session.Session, isDisconnect bool) {
	uid := s.UID()
	d.group.Leave(s)
	if isDisconnect {
		d.dissolve.updateOnlineStatus(uid, false)
	} else {
		restPlayers := []*Player{}
		for _, p := range d.players {
			if p.Uid() != uid {
				restPlayers = append(restPlayers, p)
			} else {
				p.reset()
				p.desk = nil
				p.score = 1000
				p.turn = 0
			}
		}
		d.players = restPlayers
	}

	//如果桌上已无玩家, destroy it
	if d.creator == uid && !isDisconnect {
		//if d.dissolve.offlineCount() == len(d.players) || (d.creator == uid && !isDisconnect) {
		d.logger.Info("所有玩家下线或房主主动解散房间")
		if d.dissolve.isDissolving() {
			d.dissolve.stop()
		}
		d.destroy()

		// 数据库异步更新
		async.Run(func() {
			desk := &model.Desk{
				Id:    d.deskID,
				Round: 0,
			}
			if err := db.UpdateDesk(desk); err != nil {
				log.Error(err)
			}
		})
	}
}

func (d *Desk) playerWithId(uid int64) (*Player, error) {
	for _, p := range d.players {
		if p.Uid() == uid {
			return p, nil
		}
	}

	return nil, errutil.ErrPlayerNotFound
}

func (d *Desk) onPlayerReJoin(s *session.Session) error {
	// 同步房间基本信息
	basic := &protocol.DeskBasicInfo{
		DeskID: d.roomNo.String(),
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
			Score:    p.score,
			IP:       p.ip,
			Offline:  !d.dissolve.isOnline(uid),
		})
	}
	if err := s.Push("onPlayerEnter", enter); err != nil {
		log.Error(err.Error())
		return err
	}

	p, err := playerWithSession(s)
	if err != nil {
		log.Error(err)
		return err
	}

	if err := d.playerJoin(s, true); err != nil {
		log.Error(err)
	}

	// 首局结束以后, 未点继续战斗, 此时强制退出游戏
	st := d.status()
	if st != DeskStatusCreate &&
		st != DeskStatusCleaned &&
		st != DeskStatusInterruption {
		if err := p.syncDeskData(); err != nil {
			log.Error(err)
		}
	} else {
		d.prepare.ready(s.UID())
		d.syncDeskStatus()
		// 必须在广播消息以后调用checkStart
		d.checkStart()
	}

	return nil
}

func (d *Desk) doDissolve() {
	if d.status() == DeskStatusDestory {
		d.logger.Debug("房间已经销毁")
		return
	}

	log.Debugf("房间: %s解散倒计时结束, 房间解散开始", d.roomNo)
	//如果不是在桌子刚创建时解散,需要进行退出处理
	if status := d.status(); status == DeskStatusCreate {
		d.group.Broadcast("onDissolve", &protocol.ExitResponse{
			IsExit:   true,
			ExitType: protocol.ExitTypeDissolve,
		})
		d.destroy()
	} else {
		d.setStatus(DeskStatusInterruption)
		d.roundOver()
	}
	d.logger.Debug("房间解散倒计时结束, 房间解散完成")
}

func (d *Desk) loseCoin() {
	cardCount := requireCardCount(d.opts.MaxRound)
	consume := &model.CardConsume{
		UserId:    d.creator,
		CardCount: cardCount,
		DeskId:    d.deskID,
		DeskNo:    d.roomNo.String(),
		ConsumeAt: time.Now().Unix(),
	}

	p, err := d.playerWithId(d.creator)
	if err != nil {
		d.logger.Errorf("扣除玩家房卡错误，没有找到玩家，CreatorID=%d", d.creator)
		return
	}
	p.loseCoin(int64(cardCount), consume)
}
