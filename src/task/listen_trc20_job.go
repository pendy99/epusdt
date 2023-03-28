package task

import (
	"strings"
	"sync"

	"github.com/assimon/luuu/model/data"
	"github.com/assimon/luuu/model/service"
	"github.com/assimon/luuu/util/log"
)

type ListenTrc20Job struct {
}

var gListenTrc20JobLock sync.Mutex

func (r ListenTrc20Job) Run() {
	gListenTrc20JobLock.Lock()
	defer gListenTrc20JobLock.Unlock()
	walletAddress, err := data.GetAvailableWalletAddress()
	if err != nil {
		log.Sugar.Error(err)
		return
	}
	log.Sugar.Debug("walletAddress count = %d", len(walletAddress))
	if len(walletAddress) <= 0 {
		return
	}
	var wg sync.WaitGroup
	for _, address := range walletAddress {
		wg.Add(1)
		if strings.HasPrefix(address.Token, "0x") {
			log.Sugar.Debug("check erc20")
			go service.Erc20CallBack(address.Token, &wg)
		} else {
			log.Sugar.Debug("check trc20")
			go service.Trc20CallBack(address.Token, &wg)
		}
	}
	wg.Wait()
}
