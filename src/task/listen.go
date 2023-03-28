package task

import (
	"github.com/assimon/luuu/config"
	"github.com/robfig/cron/v3"
)

func Start() {
	c := cron.New()
	// 汇率监听
	c.AddJob("@every 60s", UsdtRateJob{})
	// trc20钱包监听
	if config.BlockchainType == 1 {
		c.AddJob("@every 5s", ListenTrc20Job{})
	} else {
		c.AddJob("@every 15s", ListenTrc20Job{})
	}

	c.Start()
}
