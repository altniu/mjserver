package whitelist

import (
	"regexp"
	"sync"
)

var (
	lock sync.RWMutex //读写锁 过个goroutine同时读取，写的时候只能一个写不能读
	ips  = map[string]*regexp.Regexp{}
)

func Setup(list []string) error {
	lock.Lock() //读写锁只能一个写
	defer lock.Unlock()

	for _, ip := range list {
		re, err := regexp.Compile(ip) //编译正则表达式
		if err != nil {
			return err
		}
		ips[ip] = re
	}

	return nil
}

//VerifyIP check the ip is a legal ip or not
func VerifyIP(ip string) bool {
	lock.RLock()
	defer lock.RUnlock()

	for _, r := range ips {
		if r.MatchString(ip) { //测试匹配ip是否在白名单里
			return true
		}
	}
	return false
}

func RegisterIP(ip string) error {
	lock.Lock() ////读写锁只能一个写
	defer lock.Unlock()

	_, ok := ips[ip]
	if ok {
		return nil
	}

	re, err := regexp.Compile(ip)
	if err != nil {
		return err
	}
	ips[ip] = re
	return nil
}

func RemoveIP(ip string) {
	lock.Lock() //读写锁只能一个写
	defer lock.Unlock()

	delete(ips, ip) //删除map[sting]*regexp.Regexp{} 根据key删除
}

func IPList() []string {
	lock.RLock()
	defer lock.RUnlock()

	list := []string{}
	for ip := range ips {
		list = append(list, ip) //append 添加
	}

	return list
}

func ClearIPList() {
	lock.Lock() //读写锁只能一个写
	defer lock.Unlock()

	ips = map[string]*regexp.Regexp{}
}
