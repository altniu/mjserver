package game

const (
	turnUnknown = 255 //最多可能只有4个方位
)

const (
	kCurPlayer = "player"
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
