package db

import (
    "fmt"
    "strings"
)

// Build data source name
func BuildDBDSN(username string, password string, host string, port int, dbname, args string) string {
    return fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?%s", username, password, host, port, dbname, args)
}

// 给定列, 返回起始时间条件SQL语句, [begin, end)
func RangeCondition(column string, begin, end int64) string {
    return fmt.Sprintf("(`%s` BETWEEN %d AND %d)", column, begin, end)
}

func ChannelCondition(c []string) string {
    return fmt.Sprintf("`channel` IN('%s')", strings.Join(c, "','"))
}

func EqIntCondition(col string, v int) string {
    return fmt.Sprintf("`%s`=%d", col, v)
}

func EqInt64Condition(col string, v int64) string {
    return fmt.Sprintf("`%s`=%d", col, v)
}

func LtInt64Condition(col string, v int64) string {
    return fmt.Sprintf("`%s`<%d", col, v)
}

func Combined(cond ...string) string {
    return strings.Join(cond, " AND ")
}

func Insert(bean interface{}) error {
    _, err := database.Insert(bean)
    return err
}
