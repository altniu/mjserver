package dsq

import (
	"fmt"
	"math/rand"
	"time"
)

func init() {
	rand.Seed(time.Now().Unix())
}

// 棋子id定义
// 8 象 7 狮 6虎 5豹 4狼 3狗 2猫 1鼠  16 象 15 狮 14虎 13豹 12狼 11狗 10猫 9鼠  0吃掉的值 -1是没有翻开的值

const chessCount = 16

type (
	Dsq struct {
		Origin  []int
		Current []int
	}
)

func NewDsq() *Dsq {
	return &Dsq{
		Origin:  make([]int, chessCount),
		Current: make([]int, chessCount),
	}
}

//初始化
func (self *Dsq) Init() {
	for i, _ := range self.Origin {
		self.Origin[i] = i + 1
	}

	for j, _ := range self.Current {
		self.Current[j] = -1
	}
}

//发牌洗牌
func (self *Dsq) Ready() {
	s := rand.New(rand.NewSource(time.Now().Unix()))
	for i := range self.Origin {
		j := s.Intn(chessCount)
		self.Origin[i], self.Origin[j] = self.Origin[j], self.Origin[i]
	}
}

// 输出整个棋盘
func (self *Dsq) DumpOrigin() string {
	ret := fmt.Sprintf("%v", self.Origin)
	fmt.Println(ret)
	return ret
}

// 输出整个棋盘
func (self *Dsq) DumpCurrent() string {
	ret := fmt.Sprintf("%v", self.Current)
	fmt.Println(ret)
	return ret
}

//翻牌
func (self *Dsq) Open(index int) int {
	if index >= chessCount {
		fmt.Println("open index error", index)
		return 0
	}
	piece := self.Origin[index]
	self.Current[index] = piece
	return piece
}

//移动
//return 1: 正产移动 0:移动失败
func (self *Dsq) Move(indexSrc int, indexDest int) bool {
	if indexSrc >= chessCount || indexDest >= chessCount {
		fmt.Println("open index error", indexSrc, indexDest)
		return false
	}

	if self.Current[indexSrc] == -1 {
		return false
	}

	if self.Current[indexDest] != 0 {
		return false
	}

	if self.isLegalPiece(indexSrc) == false {
		return false
	}

	self.Current[indexDest] = self.Current[indexSrc]
	self.Current[indexSrc] = 0

	return true
}

//1吃 2被吃 3同归 0失败
func (self *Dsq) Eat(indexSrc int, indexDest int) int {
	if indexSrc >= chessCount || indexDest >= chessCount {
		fmt.Println("open index error", indexSrc, indexDest)
		return 0
	}

	if self.isLegalPiece(indexSrc) == false {
		return 0
	}

	if self.isLegalPiece(indexDest) == false {
		return 0
	}

	myself := self.getCamp(indexSrc)
	enemy := self.getCamp(indexDest)
	if myself == enemy {
		return 0
	}

	srcPiece := self.Current[indexSrc]
	destPiece := self.Current[indexDest]

	if srcPiece <= 8 {
		destPiece = destPiece - 8
	} else {
		srcPiece = srcPiece - 8
	}

	if srcPiece == destPiece {
		fmt.Println("同归", indexSrc, indexDest, self.Current[indexSrc], self.Current[indexDest])
		self.Current[indexSrc] = 0
		self.Current[indexDest] = 0
		return 3
	}

	bLess := self.isLess(srcPiece, destPiece)
	if bLess {
		fmt.Println("被吃", indexSrc, indexDest, self.Current[indexSrc], self.Current[indexDest])
		self.Current[indexSrc] = 0
		return 2
	} else {
		fmt.Println("吃", indexSrc, indexDest, self.Current[indexSrc], self.Current[indexDest])
		self.Current[indexDest] = self.Current[indexSrc]
		self.Current[indexSrc] = 0
		return 1
	}
}

// 检测结束和输赢返回阵营
// true 结束 0:平局 1:A 阵营 2:B阵营
func (self *Dsq) CheckOver() (bool, int) {
	campA := 0
	campB := 0
	for _, v := range self.Current {
		if v == -1 {
			return false, -1
		}
		if v >= 1 && v <= 8 {
			campA = campA + 1
		} else if v >= 9 && v <= 16 {
			campB = campB + 1
		}
	}
	if campA == 0 && campB == 0 {
		return true, 0
	}

	if campA > 0 && campB == 0 {
		return true, 1
	}

	if campA == 0 && campB > 0 {
		return true, 2
	}
	return false, -1
}

// 是否小
func (self *Dsq) isLess(srcPiece int, destPiece int) bool {
	if srcPiece == 8 && srcPiece == 1 {
		return true //象 鼠
	}
	if srcPiece == 1 && destPiece == 8 {
		return false //鼠 象
	}
	return srcPiece < destPiece
}

// 是否合法棋子
func (self *Dsq) isLegalPiece(index int) bool {
	if index >= chessCount {
		return false
	}

	piece := self.Current[index]
	if piece >= 1 && piece <= 16 {
		return true
	}

	return false
}

// 获取阵营
func (self *Dsq) getCamp(index int) int {
	piece := self.Current[index]
	if piece <= 8 {
		return 1
	}
	if piece > 8 {
		return 2
	}
	return 0
}

// 是否不同阵营
func (self *Dsq) isLegalCamp(index int) bool {
	if index >= chessCount {
		return false
	}

	piece := self.Current[index]
	if piece >= 1 && piece <= 16 {
		return true
	}

	return false
}
