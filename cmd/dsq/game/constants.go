package game

const (
	playerMax = 2
)

const (
	fieldDesk   = "desk"
	fieldPlayer = "player"
)

const (
	SUC  = 0
	FAIL = 1
)

type OpType int

const (
	opNull OpType = iota 
	opOpen,
	opMove,
	opEat,
)

var stringOpType = [...]string{
	opNull:  "无效操作"
	opOpen:  "翻盘",
	opMove:  "移动",
	opEat:   "吃牌",
}

var stringify = [...]string{
	DeskStatusCreate:       "创建",
	DeskStatusDuanPai:      "发牌",
	DeskStatusQiPai:        "齐牌",
	DeskStatusPlaying:      "游戏中",
	DeskStatusInterruption: "游戏终/中止",
	DeskStatusDestory:      "已销毁",
	DeskStatusCleaned:      "已清洗",
}

//错误提示
const (
	deskNotFoundMessage        = "您输入的房间号不存在, 请确认后再次输入"
	deskPlayerNumEnoughMessage = "您加入的房间已经满人, 请确认房间号后再次确认"
	versionExpireMessage       = "你当前的游戏版本过老，请更新客户端"
	deskCardNotEnoughMessage   = "金币不足"
	inRoomPlayerNowMessage     = "你当前正在房间中"
)

//错误码
const (
	eDeskPlayerNumEnough  = 1001
	eJoinVersionExpire    = 1002
	eDeskNotFoundResponse = 1003
	eReentryDesk          = 1004
	eCreateVersionExpire  = 1005
	eDeskCardNotEnough    = 1006
	ePlayerNotInDesk      = 1007
)

type Behavior int

const (
	BehaviorNone Behavior = iota
	BehaviorOpen
	BehaviorEat
	BehaviorMove
	BehaviorGiveUp
	BehaviorEnjoy
)

type DeskStatus int32

const (
	//创建桌子
	DeskStatusCreate DeskStatus = iota
	//发牌
	DeskStatusDuanPai
	//齐牌
	DeskStatusQiPai
	//游戏
	DeskStatusPlaying
	//游戏终/中止
	DeskStatusInterruption
	//已销毁
	DeskStatusDestory
	//已经清洗,即为下一轮准备好
	DeskStatusCleaned
)

var stringify = [...]string{
	DeskStatusCreate:       "创建",
	DeskStatusDuanPai:      "发牌",
	DeskStatusQiPai:        "齐牌",
	DeskStatusPlaying:      "游戏中",
	DeskStatusInterruption: "游戏终/中止",
	DeskStatusDestory:      "已销毁",
	DeskStatusCleaned:      "已清洗",
}

func (s DeskStatus) String() string {
	return stringify[s]
}
