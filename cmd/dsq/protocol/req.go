package protocol

import (
	"github.com/lonng/nanoserver/pkg/constant"
)

type GetRankInfoRequest struct {
	IsSelf bool `json:"isself"`
	Start  int  `json:"start"`
	Len    int  `json:"len"`
}

type MailOperateRequest struct {
	MailIDs []int64 `json:"mailids"`
}

type ApplyForDailyMatchRequest struct {
	Arg1           int `json:"arg1"`
	DailyMatchType int `json:"dailyMatchType"`
	Multiple       int `json:"multiple"`
}

type JQToCoinRequest struct {
	Count int `json:"count"`
}

type BashiCoinOpRequest struct {
	Op   int `json:"op"`
	Coin int `json:"coin"`
}

type ReJoinDeskRequest struct {
	DeskNo string `json:"deskId"`
}

type ReJoinDeskResponse struct {
	Code  int    `json:"code"`
	Error string `json:"error"`
}

type ReEnterDeskRequest struct {
	DeskNo string `json:"deskId"`
}

type ReEnterDeskResponse struct {
	Code  int    `json:"code"`
	Error string `json:"error"`
}
type JoinDeskRequest struct {
	Version string `json:"version"`
	//AccountId int64         `json:"acId"`
	DeskNo string `json:"deskId"`
}

type TableInfo struct {
	DeskNo    string              `json:"deskId"`
	CreatedAt int64               `json:"createdAt"`
	Creator   int64               `json:"creator"`
	Title     string              `json:"title"`
	Desc      string              `json:"desc"`
	Status    constant.DeskStatus `json:"status"`
	Mode      int                 `json:"mode"`
}

type JoinDeskResponse struct {
	Code      int       `json:"code"`
	Error     string    `json:"error"`
	TableInfo TableInfo `json:"tableInfo"`
}

type DestoryDeskRequest struct {
	DeskNo string `json:"deskId"`
}

type CheckOrderReqeust struct {
	OrderID string `json:"orderid"`
}

type CheckOrderResponse struct {
	Code   int    `json:"code"`
	Error  string `json:"error"`
	FangKa int    `json:"fangka"`
}

type PlayerOfflineStatus struct {
	Uid     int64 `json:"uid"`
	Offline bool  `json:"offline"`
}

type CoinChangeInformation struct {
	Coin int64 `json:"coin"`
}

type OpChoosed struct {
	Type int `json:"type"`
}

//翻牌
type PieceOpenRequest struct {
	Index int `json:"index"`
}

type PieceOpenResponse struct {
	Code  int `json:"code"`
	Index int `json:"index"`
	Piece int `json:"piece"`
}

type PieceOpenNotify struct {
	Uid   int64 `json:"uid"`
	Index int   `json:"index"`
	Piece int   `json:"piece"`
}

//吃牌
type PieceEatRequest struct {
	IndexSrc  int `json:"indexSrc"`
	IndexDest int `json:"indexDest"`
}

type PieceEatResponse struct {
	Code   int   `json:"code"`
	Pieces []int `json:"pieces"`
}

type PieceEatNotify struct {
	Uid       int64 `json:"uid"`
	Code      int   `json:"code"`
	IndexSrc  int   `json:"indexSrc"`
	IndexDest int   `json:"indexDest"`
}

//移动
type PieceMoveRequest struct {
	IndexSrc  int `json:"indexSrc"`
	IndexDest int `json:"indexDest"`
}

type PieceMoveResponse struct {
	Code   int   `json:"code"`
	Pieces []int `json:"pieces"`
}

type PieceMoveNotify struct {
	Uid       int64 `json:"uid"`
	IndexSrc  int   `json:"indexSrc"`
	IndexDest int   `json:"indexDest"`
}

//投降
type GiveupRequest struct {
	DeskNo string `json:"deskId"`
}

// 游戏结算
type GameResult struct {
	Winner  int64 `json:"winner"`
	Coin    int64 `json:"coin"`
	Camp    int   `json:"camp"` // 0 平局 1  2
	Giveup  bool  `json:"giveup"`
	TimeOut bool  `json:"timeOut"`
}

//表情
type PlayEjoyReq struct {
	Index int `json:"index"`
}

//表情
type PlayEjoyNotify struct {
	Uid   int64 `json:"uid"`
	Index int   `json:"index"`
}
