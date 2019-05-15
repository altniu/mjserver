package protocol

type EnterDeskInfo struct {
	DeskPos  int    `json:"pos"`
	Uid      int64  `json:"uid"`
	Nickname string `json:"name"`
	IsReady  bool   `json:"isReady"`
	Sex      int    `json:"sex"`
	IsExit   bool   `json:"isExit"`
	HeadUrl  string `json:"headURL"`
	Score    int    `json:"score"`
	IP       string `json:"ip"`
}

type PlayerEnterDesk struct {
	Data []EnterDeskInfo `json:"data"`
}

type ExitRequest struct {
	IsDestroy bool `json:"isDestroy"`
}

type ExitResponse struct {
	Code int `json:"Code"` // 0 离开成功 1 离开失败已经开始游戏 2房间已经解散
}

type DeskBasicInfo struct {
	DeskID string `json:"deskId"`
	Title  string `json:"title"`
	Desc   string `json:"desc"`
	Mode   int    `json:"mode"`
}

type DeskPlayerData struct {
	Uid int64 `json:"acId"`
}

type SyncDesk struct {
	Status  int32            `json:"status"` //1,2,3,4,5
	Players []DeskPlayerData `json:"players"`
}

type DeskOptions struct {
	Mode int `json:"mode"`
}

type CreateDeskRequest struct {
	Version  string       `json:"version"` //客户端版本
	DeskOpts *DeskOptions `json:"options"` // 游戏额外选项
}

type CreateDeskResponse struct {
	Code      int       `json:"code"`
	Error     string    `json:"error"`
	TableInfo TableInfo `json:"tableInfo"`
}

type ReConnect struct {
	Uid     int64  `json:"uid"`
	Name    string `json:"name"`
	HeadUrl string `json:"headUrl"`
	Sex     int    `json:"sex"`
}

type DeskListRequest struct {
	Player int64 `json:"player"`
	Offset int   `json:"offset"`
	Count  int   `json:"count"`
}

type Desk struct {
	Id           int64  `json:"id"`
	Creator      int64  `json:"creator"`
	DeskNo       string `json:"desk_no"`
	Mode         int    `json:"mode"`
	Player0      int64  `json:"player0"`
	Player1      int64  `json:"player1"`
	PlayerName0  string `json:"player_name0"`
	PlayerName1  string `json:"player_name1"`
	CreatedAt    int64  `json:"created_at"`
	CreatedAtStr string `json:"created_at_str"`
	Extras       string `json:"extras"`
}

type DeskListResponse struct {
	Code  int    `json:"code"`
	Total int64  `json:"total"` //总数量
	Data  []Desk `json:"data"`
}

type DeleteDeskByIDRequest struct {
	ID string `json:"id"` //房间ID
}
type DeskByIDRequest struct {
	ID int64 `json:"id"` //房间ID
}

type DeskByIDResponse struct {
	Code int   `json:"code"`
	Data *Desk `json:"data"`
}

type UnCompleteDeskResponse struct {
	Exist     bool      `json:"exist"`
	TableInfo TableInfo `json:"tableInfo"`
}

type ClientInitCompletedRequest struct {
	IsReEnter bool `json:"isReenter"`
}

type CampInfo struct {
	Uid     int64  `json:"uid"`
	Name    string `json:"name"`
	HeadUrl string `json:"headurl"`
	Camp    int    `json:"camp"`
}

type DuanPai struct {
	Pieces []int      `json:"pieces"` //棋盘
	Camps  []CampInfo `json:"camps"`  //阵营
}

type RoundInfo struct {
	Uid       int64 `json:"uid"`       //提示的玩家
	Camp      int   `json:"camp"`      //当前出牌的阵营
	TimeStamp int64 `json:"timestamp"` //提示的时间戳
}
