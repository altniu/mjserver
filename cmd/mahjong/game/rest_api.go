package game

import (
    "github.com/lonng/nanoserver/protocol"
)

// 供web使用的一些游戏内的接口 通过chan传递操作

// 踢人
func Kick(uid int64) error {
    defaultPlayerManager.chKick <- uid
    return nil
}

// 广播
func BroadcastSystemMessage(message string) {
    defaultPlayerManager.group.Broadcast("onBroadcast", &protocol.StringMessage{Message: message})
}

// 重置
func Reset(uid int64) {
    defaultPlayerManager.chReset <- uid
}

// 充值
func Recharge(uid, coin int64) {
    defaultPlayerManager.chRecharge <- RechargeInfo{uid, coin}
}
