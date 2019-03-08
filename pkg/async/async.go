package async

import "github.com/sirupsen/logrus"

//panic抛出异常，然后在defer中调用recover()捕获异常
func pcall(fn func()) {
	defer func() {
		if err := recover(); err != nil {
			logrus.Errorf("aync/pcall: Error=%v", err)
		}
	}()

	fn()
}

func Run(fn func()) {
	go pcall(fn)
}
