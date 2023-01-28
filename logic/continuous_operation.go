package logic

import (
	"strings"
	"sync/atomic"
	"time"

	"github.com/openark/golib/log"
	"github.com/github/my-manager/config"
	"github.com/github/my-manager/process"
	"github.com/github/my-manager/raft"
	"github.com/github/my-manager/util"
)

const (
	yieldAfterUnhealthyDuration = 5 * config.HealthPollSeconds * time.Second
	fatalAfterUnhealthyDuration = 30 * config.HealthPollSeconds * time.Second
)

var isElectedNode int64 = 0

func IsLeader() bool {
	if oraft.IsRaftEnabled() {
		return oraft.IsLeader()
	}
	return atomic.LoadInt64(&isElectedNode) == 1
}

func IsLeaderOrActive() bool {
	if oraft.IsRaftEnabled() {
		return oraft.IsPartOfQuorum()
	}
	return atomic.LoadInt64(&isElectedNode) == 1
}

func RaftNodesStatusCheck() {
	var info string
	nodeInfo := config.Config.RaftBind + config.Config.ListenAddress + " " + "ApiEndpoint" + ":" + config.Config.ApiEndpoint
	alertApi := config.Config.RaftNodesStatusAlertProcess
	if alertApi == "" {
		nodeInfo = nodeInfo + " " + "RaftNodesStatusAlertProcess is null"
		log.Error(nodeInfo)
		return
	}
	health, err := process.HealthTest()

	if err != nil {
		nodeInfo = nodeInfo + " " + "raft cluster HealthTest:" + err.Error()
		log.Error(nodeInfo)
		alertApi = strings.Replace(alertApi, "{msg}", nodeInfo, -1)
		err := util.RunCommandNoOutput(alertApi)
		if err != nil {
			log.Errorf("run RaftNodesStatusAlertProcess failed:%s", err.Error())
		}
		return
	}
	if !health.Healthy {
		info = "raft node is not health"
		info = nodeInfo + " " + info
		alertApi = strings.Replace(alertApi, "{msg}", info, -1)
		err := util.RunCommandNoOutput(alertApi)
		if err != nil {
			log.Errorf("run RaftNodesStatusAlertProcess failed:%s", err.Error())
		}
		return
	}
	if len(health.AvailableNodes) != len(config.Config.RaftNodes) {
		info = "raft cluster AvailableNodes is less than RaftNodes"
		info = nodeInfo + " " + info
		alertApi = strings.Replace(alertApi, "{msg}", info, -1)
		err := util.RunCommandNoOutput(alertApi)
		if err != nil {
			log.Errorf("run RaftNodesStatusAlertProcess failed:%s", err.Error())
		}
		return
	}
	return
}

func LeaderDomainCheck() error {
	var localIp string
	var lookupIp []string
	var err error

	if config.Config.RaftLeaderDomain == "" {
		return nil
	} else {
		if len(config.Config.SwithDomainProcess) == 0 {
			log.Errorf("RaftLeaderDomain is set but SwithDomainProcess is null")
			return nil
		}
	}

	localIp, err = util.GetLocalIP()
	if err != nil {
		log.Errorf("Run LeaderDomainCheck GetLocalIP err :%s", err.Error())
		return nil
	}

	runAddDomain := func() error {
                for _, value := range config.Config.SwithDomainProcess {
			if value == "" {
				continue
			}
			value = strings.Replace(value, "{domain}", config.Config.RaftLeaderDomain, -1)
			value = strings.Replace(value, "{ip}", localIp, -1)
			_, err := util.RunCommandOutput(value)
			if err != nil {
				log.Errorf("run %s err :%s", value, err.Error())
				return err
			}
		}
		return nil
	}

	lookupIp, err = util.LookupHost(config.Config.RaftLeaderDomain)
	if err != nil || len(lookupIp) == 0 {
		if er := runAddDomain(); er != nil {
			return er
		}
	}

	if len(lookupIp) > 0 {
		if localIp != lookupIp[0] {
			if er := runAddDomain(); er != nil {
				return er
			}
		}
	}

	return nil
}

type OutScripts struct {
	Url                string
	Method             string
	Key                string
	Param              string
	RunIntervalSeconds string
	OutputFlag         string
	Script             string
	TickTime           <-chan time.Time
}

func RunOutScript(outscript *OutScripts) {
	for _ = range outscript.TickTime {
		runFun := func() {
			if outscript.OutputFlag == "0" {
				err := util.RunCommandNoOutput(outscript.Script)
				if err != nil {
					log.Errorf("run cmd %s failed: %s", outscript.Script, err.Error())
				}
			} else {
				_, err := util.RunCommandOutput(outscript.Script)
				if err != nil {
					log.Errorf("run cmd %s failed: %s", outscript.Script, err.Error())
				}
			}
		}
		if oraft.IsRaftEnabled() {
			if oraft.IsLeader() {
				runFun()
			}
		} else {
			runFun()
		}
	}
}

// ContinuousOperation starts an asynchronuous infinite discovery process where instances are
// periodically investigated and their status captured
func ContinuousOperation() {
	log.Infof("continuous operation: setting up")

	//run outSideScripts from config
	{
		scripts := config.Config.Processes
		outProcessScripts := make([]*OutScripts, 0)
		for _, vmap := range scripts {
			if vmap["runIntervalSeconds"] != "" {
				runIntervalSeconds := util.ConvStrToUInt(vmap["runIntervalSeconds"])
				if runIntervalSeconds == 0 {
					runIntervalSeconds = 60
				}
				outScripts := &OutScripts{
					RunIntervalSeconds: vmap["runIntervalSeconds"],
					Script:             vmap["script"],
					OutputFlag:         vmap["outputFlag"],
					TickTime:           time.Tick(time.Duration(runIntervalSeconds) * time.Second),
				}
				outProcessScripts = append(outProcessScripts, outScripts)
			}
		}
		for _, script := range outProcessScripts {
			go RunOutScript(script)
		}
	}

	healthTick := time.Tick(config.HealthPollSeconds * time.Second)
	domainCheckTick := time.Tick(time.Duration(config.Config.DomainCheckIntervalSeconds) * time.Second)
	caretakingTick := time.Tick(time.Minute)
	raftNodesStatusCheckTick := time.Tick(time.Duration(config.Config.RaftNodesStatusCheckIntervalSeconds) * time.Second)

	if config.Config.RaftEnabled {
		if err := oraft.Setup(NewCommandApplier(), NewSnapshotDataCreatorApplier(), process.ThisHostname); err != nil {
			log.Fatale(err)
		}
		go oraft.Monitor()
	}

	log.Infof("continuous operation: starting")
	for {
		select {
		case <-healthTick:
			go func() {
				onHealthTick()
			}()
		case <-domainCheckTick:
			LeaderDomainCheck()
			if oraft.IsLeader() {
				LeaderDomainCheck()
			}
		case <-caretakingTick:
			//if IsLeaderOrActive() {
			if oraft.IsLeader() {
				go process.ExpireNodesHistory()
				go process.ExpireAvailableNodes()
			}
		case <-raftNodesStatusCheckTick:
			if oraft.IsRaftEnabled() {
				RaftNodesStatusCheck()
			}
		}
	}

}

func onHealthTick() {
	wasAlreadyElected := IsLeader()

	if oraft.IsRaftEnabled() {
		if oraft.IsLeader() {
			atomic.StoreInt64(&isElectedNode, 1)
		} else {
			atomic.StoreInt64(&isElectedNode, 0)
		}

		if process.SinceLastGoodHealthCheck() > yieldAfterUnhealthyDuration {
			log.Errorf("Heath test is failing for over %+v seconds. raft yielding", yieldAfterUnhealthyDuration.Seconds())
			oraft.Yield()
		}
		if process.SinceLastGoodHealthCheck() > fatalAfterUnhealthyDuration {
			log.Error("Node is unable to register health. Please check database connnectivity.")
		}
	}

	if !oraft.IsRaftEnabled() {
		myIsElectedNode, err := process.AttemptElection()
		if err != nil {
			log.Errore(err)
		}

		if myIsElectedNode {
			atomic.StoreInt64(&isElectedNode, 1)
		} else {
			atomic.StoreInt64(&isElectedNode, 0)
		}
		if !myIsElectedNode {
			if electedNode, _, err := process.ElectedNode(); err == nil {
				log.Infof("Not elected as active node; active node: %v; polling", electedNode.Hostname)
			} else {
				log.Infof("Not elected as active node; active node: Unable to determine: %v; polling", err)
			}
		}
	}

	if !IsLeaderOrActive() {
		return
	}

	if !wasAlreadyElected {
		// Just turned to be leader!
		go process.RegisterNode(process.ThisNodeHealth)
	}
}
