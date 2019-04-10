package game

import (
	"fmt"

	"github.com/lonng/nano/session"
	"github.com/lonng/nanoserver/cmd/dsq/db"
	"github.com/lonng/nanoserver/cmd/dsq/db/model"
	"github.com/lonng/nanoserver/cmd/dsq/game/dsq"
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
	session     *session.Session //玩家session
	desk        *Desk            //当前桌
	turn        int              //当前玩家在桌上的方位
	ctx         *mahjong.Context
	chOperation chan *protocol.OpChoosed

	logger *log.Entry // 日志
}

func newPlayer(s *session.Session, uid int64, name, head, ip string, sex int) *Player {
	p := &Player{
		uid:         uid,
		name:        name,
		head:        head,
		ctx:         &mahjong.Context{Uid: uid},
		ip:          ip,
		sex:         sex,
		score:       1000,
		logger:      log.WithField(fieldPlayer, uid),
		chOperation: make(chan *protocol.OpChoosed, 1),
	}

	p.ctx.Reset()
	p.bindSession(s)
	p.syncCoinFromDB()

	return p
}

// 异步从数据库同步玩家数据
func (p *Player) syncCoinFromDB() {
	async.Run(func() {
		u, err := db.QueryUser(p.uid)
		if err != nil {
			p.logger.Errorf("玩家同步金币错误, Error=%v", err)
			return
		}

		p.coin = u.Coin
		if s := p.session; s != nil {
			s.Push("onCoinChange", &protocol.CoinChangeInformation{p.coin})
		}
	})
}

// 异步扣除玩家金币
func (p *Player) loseCoin(count int64, consume *model.CardConsume) {
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

		if err := db.Insert(consume); err != nil {
			p.logger.Errorf("新增消费数据错误，Error=%v Payload=%+v", err, consume)
		}

		if s := p.session; s != nil {
			s.Push("onCoinChange", &protocol.CoinChangeInformation{p.coin})
		}
	})
}

//设置玩家所在的桌子
func (p *Player) setDesk(d *Desk, turn int) {
	if d == nil {
		p.logger.Error("桌号为空")
		return
	}
	p.desk = d
	p.turn = turn
	p.ctx.DeskNo = string(d.roomNo)
	p.logger = log.WithFields(log.Fields{fieldDesk: p.desk.roomNo, fieldPlayer: p.uid})
}

func (p *Player) setIp(ip string) {
	p.ip = ip
}

//player 关联session session关联player
func (p *Player) bindSession(s *session.Session) {
	p.session = s
	p.session.Set(kCurPlayer, p) //"player"字段
}

func (p *Player) removeSession() {
	p.session.Remove(kCurPlayer)
	p.session = nil
}

func (p *Player) Uid() int64 {
	return p.uid
}

func (p *Player) duanPai(ids mahjong.Tiles) {
	p.onHand = mahjong.FromID(ids)
	p.logger.Debugf("游戏开局, 手牌数量=%d 手牌: %v", len(p.handTiles()), p.handTiles())
	if len(p.onHand) == 14 {
		p.ctx.NewDrawingID = p.onHand[13].Id
	}
}

func (p *Player) chuPai() int {
	var tid int

ctrl:
	p.hint([]protocol.Op{{Type: protocol.OptypeChu}}, p.tingTiles())
	select {
	case op, ok := <-p.chOperation:
		if !ok {
			return deskDissolved
		}

		if op.Type != protocol.OptypeChu {
			p.logger.Errorf("玩家操作异常，期待操作出牌，获取操作=%+v", op)
			goto ctrl
		}

		tid = op.TileID
		if tid < 0 {
			p.logger.Debugf("玩家读取到一个非法的麻将ID: ID=%+v", op)
		}

	case <-p.desk.die:
		return deskDissolved
	}

	// 删掉已经出过的牌
	for j, mj := range p.onHand {
		if mj.Id == tid {
			rest := make([]*mahjong.Tile, len(p.onHand)-1)
			copy(rest[:j], p.onHand[:j])
			copy(rest[j:], p.onHand[j+1:])
			p.onHand = rest
			break
		}
	}

	p.logger.Debugf("玩家出牌: 麻将=%v(%d) 新上手=%v 余牌=%v",
		mahjong.TileFromID(tid),
		tid,
		p.ctx.NewDrawingID,
		p.handTiles())

	p.action(protocol.OptypeChu, []int{tid})
	return tid
}

// 让玩家选择胡牌
// @param: hasHint 是否已经提示过玩家
func (p *Player) hu(tileID int, hasHint bool) int {
	// 真实玩家
	if !hasHint {
		p.hint([]protocol.Op{
			{Type: protocol.OptypeHu, TileIDs: []int{tileID}},
			{Type: protocol.OptypePass},
		})
	}
	select {
	case op, ok := <-p.chOperation:
		if !ok {
			return deskDissolved
		}

		p.ctx.SetPrevOp(op.Type)
		return op.Type

	case <-p.desk.die:
		return deskDissolved
	}

	p.logger.Debugf("玩家胡牌: 麻将=%d", tileID)
	return protocol.OptypeHu
}

func (p *Player) moPai() {
	id := p.desk.nextTile().Id
	mo := &protocol.MoPai{
		AccountID: p.Uid(),
		TileIDs:   []int{id},
	}

	//此时将此牌上手到ctx中,待做了明牌处理后再正式放入
	p.ctx.NewDrawingID = id

	record := &protocol.OpTypeDo{
		OpType:  protocol.OptyMoPai,
		Uid:     []int64{p.Uid()},
		TileIDs: []int{id},
	}

	// 保存快照
	p.desk.snapshot.PushAction(record)

	// 这里如果返回错误, 可能是有玩家的socket关掉了, 玩家掉线, 可能需要把玩家标记为托管
	if err := p.desk.group.Broadcast("onMoPai", mo); err != nil {
		log.Error(err)
	}

	// TODO: 确认海底捞是否是最后一个摸牌的, 其他人摸最后一张牌, 点炮是否算海底捞
	p.ctx.IsLastTile = p.desk.noMoreTile()

	//p.onHand = append(p.onHand, mahjong.TileFromID(id))
	p.logger.Debugf("玩家摸牌: 手牌=%+v 新上手=%d", p.handTiles(), p.ctx.NewDrawingID)
}

func (p *Player) canWin() bool {
	newTile := mahjong.TileFromID(p.ctx.NewDrawingID)
	que := p.ctx.Que
	// 打缺的牌不能胡
	if que == newTile.Suit+1 {
		return false
	}

	// 还没有打缺不能胡
	for _, t := range p.onHand {
		if que == t.Suit+1 {
			return false
		}
	}

	canWin := mahjong.CheckWin(p.handTiles().Indexes())

	p.logger.Infof("玩家计算是否可以胡牌: 手牌=%+v, 新上手=%v, 是否可以胡=%t",
		p.handTiles(), newTile, canWin)

	return canWin
}

// 玩家操作
func (p *Player) action(opType int, tiles mahjong.Tiles) {
	p.logger.Debugf("玩家选择: OpType=%d Tiles=%+v", opType, tiles)

	do := &protocol.OpTypeDo{
		Uid:     []int64{p.Uid()},
		OpType:  opType,
		TileIDs: tiles,
	}
	if err := p.desk.group.Broadcast(protocol.RouteTypeDo, do); err != nil {
		log.Error(err)
	}

	//添加操作记录
	p.desk.snapshot.PushAction(do)
}

func (p *Player) reset() {
	// 重置channel
	close(p.chOperation)
	p.chOperation = make(chan *protocol.OpChoosed, 1)
	p.ctx.Reset()
}

// 断线重连后，同步牌桌数据
// TODO: 断线重连，已和牌玩家显示不正常
func (p *Player) syncDeskData() error {
	desk := p.desk
	data := &protocol.SyncDesk{
		Status:    desk.status(),
		Players:   []protocol.DeskPlayerData{},
		ScoreInfo: []protocol.ScoreInfo{},
	}

	markerUid := int64(0)
	lastMoPaiUid := int64(0)
	for i, player := range desk.players {
		uid := player.Uid()
		if i == desk.bankerTurn {
			markerUid = uid
		}
		if i == desk.curTurn {
			lastMoPaiUid = uid
		}
		// 有可能已经有玩家和牌
		stats := desk.roundOverTilesForPlayer(player)
		playerData := protocol.DeskPlayerData{
			Uid:        uid,
			HandTiles:  stats.Tiles,
			PGTiles:    player.pgTiles().Ids(),
			ChuTiles:   player.chuTiles().Ids(),
			LatestTile: player.ctx.NewDrawingID,
			HuPai:      player.ctx.WinningID,
			HuType:     player.ctx.ResultType,
			IsHu:       desk.wonPlayers[uid],
			Que:        player.ctx.Que,
			Score:      player.score,
		}

		// 如果自己断线重连，并且在定缺中，则发回提示，使用负数表示定缺建议选项
		if p.Uid() == uid && p.ctx.Que < 1 {
			playerData.Que = -player.selectDefaultQue()
		}

		data.Players = append(data.Players, playerData)

		score := protocol.ScoreInfo{
			Uid:   uid,
			Score: player.score,
		}

		data.ScoreInfo = append(data.ScoreInfo, score)
	}
	data.MarkerUid = markerUid
	data.LastMoPaiUid = lastMoPaiUid
	data.RestCount = desk.remainTileCount()
	data.Dice1 = desk.dice.dice1
	data.Dice2 = desk.dice.dice2
	syncUid := p.Uid()
	if lastMoPaiUid == syncUid || p.desk.lastHintUid == syncUid {
		data.Hint = p.ctx.LastHint
	}
	data.LastTileId = p.desk.lastTileId
	data.LastChuPaiUid = p.desk.lastChuPaiUid
	p.logger.Debugf("同步房间数据: %+v", data)
	return p.session.Push("onSyncDesk", data)
}
