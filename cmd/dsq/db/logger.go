package db

import (
    "github.com/go-xorm/core"
    log "github.com/sirupsen/logrus"
)

//go-xorm的日志

type Logger struct {
    *log.Entry
    level core.LogLevel
}

//设置日志级别
func (l *Logger) SetLevel(level core.LogLevel) {
    l.level = level
}

func (l *Logger) Level() core.LogLevel {
    return l.level
}

func (l *Logger) ShowSQL(show ...bool) {}
func (l *Logger) IsShowSQL() bool      { return false }
