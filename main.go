package main

import (
	"github.com/panjf2000/gnet/v2"
	"github.com/panjf2000/gnet/v2/pkg/logging"
)

func main() {

	err := gnet.Run(&Handler{},
		"tcp//:1883",
		gnet.WithMulticore(true),
		gnet.WithReusePort(true),
		gnet.WithLogLevel(logging.DebugLevel),
	)
	if err != nil {
		logging.Fatalf("启动失败：%v", err)
		return
	}
}
