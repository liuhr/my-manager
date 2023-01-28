package process

import (
	"sync"
	"sync/atomic"
	"time"

	"github.com/github/my-manager/config"
	"github.com/github/my-manager/raft"
	"github.com/github/my-manager/util"
	"github.com/openark/golib/log"
	"github.com/patrickmn/go-cache"
)

var lastHealthCheckUnixNano int64
var lastGoodHealthCheckUnixNano int64
var LastContinousCheckHealthy int64

var lastHealthCheckCache = cache.New(config.HealthPollSeconds*time.Second, time.Second)

const (
	ExecutionHttpMode = "HttpMode"
)

var continuousRegistrationOnce sync.Once

func RegisterNode(nodeHealth *NodeHealth) (healthy bool, err error) {
	nodeHealth.Update()
	healthy, err = WriteRegisterNode(nodeHealth)
	atomic.StoreInt64(&lastHealthCheckUnixNano, time.Now().UnixNano())
	if healthy {
		atomic.StoreInt64(&lastGoodHealthCheckUnixNano, time.Now().UnixNano())
	}
	return healthy, err
}

type NodeHealth struct {
	Hostname        string
	Token           string
	AppVersion      string
	FirstSeenActive string
	LastSeenActive  string
	ExtraInfo       string
	Command         string
	DBBackend       string

	LastReported time.Time
	onceHistory  sync.Once
	onceUpdate   sync.Once
}

type HealthStatus struct {
	Healthy            bool
	Hostname           string
	Token              string
	IsActiveNode       bool
	ActiveNode         NodeHealth
	Error              error
	AvailableNodes     [](*NodeHealth)
	RaftLeader         string
	IsRaftLeader       bool
	RaftLeaderURI      string
	RaftAdvertise      string
	RaftHealthyMembers []string
}

func NewNodeHealth() *NodeHealth {
	return &NodeHealth{
		Hostname:   ThisHostname,
		Token:      util.ProcessToken.Hash,
		AppVersion: config.NewAppVersion(),
	}
}

var ThisNodeHealth = NewNodeHealth()

func (nodeHealth *NodeHealth) Update() *NodeHealth {
	nodeHealth.onceUpdate.Do(func() {
		nodeHealth.Hostname = ThisHostname
		nodeHealth.Token = util.ProcessToken.Hash
		nodeHealth.AppVersion = config.NewAppVersion()
	})
	nodeHealth.LastReported = time.Now()
	return nodeHealth
}

// HealthTest attempts to write to the backend database and get a result
func HealthTest() (health *HealthStatus, err error) {
	cacheKey := util.ProcessToken.Hash
	if healthStatus, found := lastHealthCheckCache.Get(cacheKey); found {
		return healthStatus.(*HealthStatus), nil
	}
	health = &HealthStatus{Healthy: false, Hostname: ThisHostname, Token: util.ProcessToken.Hash}
	defer lastHealthCheckCache.Set(cacheKey, health, cache.DefaultExpiration)
	if healthy, err := RegisterNode(ThisNodeHealth); err != nil {
		health.Error = err
		return health, log.Errore(err)
	} else {
		health.Healthy = healthy
	}

	if oraft.IsRaftEnabled() {
		health.ActiveNode.Hostname = oraft.GetLeader()
		health.IsActiveNode = oraft.IsLeader()
		health.RaftLeader = oraft.GetLeader()
		health.RaftLeaderURI = oraft.LeaderURI.Get()
		health.IsRaftLeader = oraft.IsLeader()
		health.RaftAdvertise = config.Config.RaftAdvertise
		health.RaftHealthyMembers = oraft.HealthyMembers()
	} else {
		if health.ActiveNode, health.IsActiveNode, err = ElectedNode(); err != nil {
			health.Error = err
			return health, log.Errore(err)
		}
	}
	health.AvailableNodes, err = ReadAvailableNodes(true)

	return health, nil

}

func SinceLastGoodHealthCheck() time.Duration {
	timeNano := atomic.LoadInt64(&lastGoodHealthCheckUnixNano)
	if timeNano == 0 {
		return 0
	}
	return time.Since(time.Unix(0, timeNano))
}

// ContinuousRegistration will continuously update the node_health
// table showing that the current process is still running.
func ContinuousRegistration(extraInfo string, command string) {
	ThisNodeHealth.ExtraInfo = extraInfo
	ThisNodeHealth.Command = command
	continuousRegistrationOnce.Do(func() {
		tickOperation := func() {
			healthy, err := RegisterNode(ThisNodeHealth)
			if err != nil {
				log.Errorf("ContinuousRegistration: RegisterNode failed: %+v", err)
			}
			if healthy {
				atomic.StoreInt64(&LastContinousCheckHealthy, 1)
			} else {
				atomic.StoreInt64(&LastContinousCheckHealthy, 0)
			}
			//LastContinousCheckHealthy 其他模块没有用到此变量，在orchestrator中这个变量当作监控metric值使用
		}
		// First one is synchronous
		tickOperation()

		go func() {
			registrationTick := time.Tick(config.HealthPollSeconds * time.Second)
			for range registrationTick {
				// We already run inside a go-routine so
				// do not do this asynchronously.  If we
				// get stuck then we don't want to fill up
				// the backend pool with connections running
				// this maintenance operation.
				tickOperation()
			}
		}()
	})
}
