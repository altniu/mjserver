package game

import (
    "runtime"
    "strings"

    "github.com/lonng/nanoserver/cmd/dsq/protocol"
    "github.com/lonng/nanoserver/pkg/errutil"

    "github.com/lonng/nano/session"
)

const (
    ModeRoom  = 1 // 房间模式
    ModeBiSai = 2 // 比赛模式
)

func verifyOptions(opts *protocol.DeskOptions) bool {
    if opts == nil {
        return false
    }

    if opts.Mode != ModeBiSai && opts.Mode != ModeRoom {
        return false
    }
    return true
}

func requireCardCount(round int) int {
    if c, ok := consume[round]; ok {
        return c
    }
    return 0
}

func playerWithSession(s *session.Session) (*Player, error) {
    p, ok := s.Value(fieldPlayer).(*Player)
    if !ok {
        return nil, errutil.ErrPlayerNotFound
    }
    return p, nil
}

func stack() string {
    buf := make([]byte, 10000)
    n := runtime.Stack(buf, false)
    buf = buf[:n]

    s := string(buf)

    // skip nano frames lines
    const skip = 7
    count := 0
    index := strings.IndexFunc(s, func(c rune) bool {
        if c != '\n' {
            return false
        }
        count++
        return count == skip
    })
    return s[index+1:]
}
