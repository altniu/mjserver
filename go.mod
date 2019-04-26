module github.com/lonng/nanoserver

replace (
	golang.org/x/crypto => github.com/golang/crypto v0.0.0-20190219172222-a4c6cb3142f2
	golang.org/x/net => github.com/golang/net v0.0.0-20190213061140-3a22650c66bd
	golang.org/x/sys => github.com/golang/sys v0.0.0-20190220154126-629670e5acc5
	golang.org/x/text => github.com/golang/text v0.3.0
	google.golang.org/appengine v1.4.0 => github.com/golang/appengine v1.4.0
)

require (
	github.com/BurntSushi/toml v0.3.1 // indirect
	github.com/chanxuehong/rand v0.0.0-20180830053958-4b3aff17f488 // indirect
	github.com/go-delve/delve v1.2.0 // indirect
	github.com/go-sql-driver/mysql v1.4.1
	github.com/go-xorm/core v0.6.2
	github.com/go-xorm/xorm v0.7.1
	github.com/gorilla/mux v1.7.0
	github.com/gorilla/websocket v1.4.0 // indirect
	github.com/lonng/nano v0.4.0
	github.com/lonng/nex v1.4.1
	github.com/mdempsky/gocode v0.0.0-20190203001940-7fb65232883f // indirect
	github.com/pborman/uuid v1.2.0
	github.com/pkg/errors v0.8.1
	github.com/sirupsen/logrus v1.3.0
	github.com/spf13/viper v1.3.1
	github.com/urfave/cli v1.20.0
	github.com/xxtea/xxtea-go v0.0.0-20170828040851-35c4b17eecf6
	golang.org/x/crypto v0.0.0-20181203042331-505ab145d0a9
	golang.org/x/net v0.0.0-20180724234803-3673e40ba225
	golang.org/x/text v0.3.0
	gopkg.in/chanxuehong/wechat.v2 v2.0.0-20180924084534-7e0579cb5377
)
