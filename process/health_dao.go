package process

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/github/my-manager/config"
	"github.com/github/my-manager/db"
	"github.com/github/my-manager/raft"
	"github.com/github/my-manager/util"
	"github.com/openark/golib/log"
	"github.com/openark/golib/sqlutils"
)

// RegisterNode writes down this node in the node_health table
func WriteRegisterNode(nodeHealth *NodeHealth) (healthy bool, err error) {
	timeNow := time.Now()
	reportedAgo := timeNow.Sub(nodeHealth.LastReported)
	reportedSecondsAgo := int64(reportedAgo.Seconds())
	if reportedSecondsAgo > config.HealthPollSeconds*2 {
		// This entry is too old. No reason to persist it; already expired.
		return false, nil
	}
	nodeHealth.onceHistory.Do(func() {
		db.ExecDb(`
			insert ignore into node_health_history
				(hostname, token, first_seen_active, extra_info, command, app_version)
			values
				(?, ?, NOW(), ?, ?, ?)
			`,
			nodeHealth.Hostname, nodeHealth.Token, nodeHealth.ExtraInfo, nodeHealth.Command,
			nodeHealth.AppVersion,
		)
	})

	{
		sqlResult, err := db.ExecDb(`
			update node_health set
				last_seen_active = now() - interval ? second,
				extra_info = case when ? != '' then ? else extra_info end,
				app_version = ?,
				incrementing_indicator = incrementing_indicator + 1
			where
				hostname = ?
				and token = ?
			`,
			reportedSecondsAgo,
			nodeHealth.ExtraInfo, nodeHealth.ExtraInfo,
			nodeHealth.AppVersion,
			nodeHealth.Hostname, nodeHealth.Token,
		)
		if err != nil {
			return false, log.Errore(err)
		}
		rows, err := sqlResult.RowsAffected()
		if err != nil {
			return false, log.Errore(err)
		}
		if rows > 0 {
			return true, nil
		}
	}

	// Got here? The UPDATE didn't work. Row isn't there.
	{
		ip := ""
		dbBackend := ""
		raftPort := ""
		dbBackend = fmt.Sprintf("%s:%d", config.Config.BackendDbHosts,
			config.Config.BackendDbPort)
		hostPort := strings.Split(config.Config.RaftBind, ":")
		if len(hostPort) > 1 {
			raftPort = hostPort[1]
		} else {
			if config.Config.DefaultRaftPort != 0 {
				raftPort = fmt.Sprintf("%d", config.Config.DefaultRaftPort)
			}
		}

		ipList, err := util.LookupHost(nodeHealth.Hostname)
		if err != nil {
			log.Errorf("LookupHost %s err %s", nodeHealth.Hostname, err.Error())
		}
		if len(ipList) > 0 {
			ip = ipList[0]
		}

		sqlResult, err := db.ExecDb(`
			insert ignore into node_health
				(hostname, token, ip, raft_port, first_seen_active, last_seen_active, extra_info, command, app_version, db_backend)
			values (
				?, ?, ?, ?,
				now() - interval ? second, now() - interval ? second,
				?, ?, ?, ?)
			`,
			nodeHealth.Hostname, nodeHealth.Token, ip, raftPort,
			reportedSecondsAgo, reportedSecondsAgo,
			nodeHealth.ExtraInfo, nodeHealth.Command,
			nodeHealth.AppVersion, dbBackend,
		)
		if err != nil {
			return false, log.Errore(err)
		}
		rows, err := sqlResult.RowsAffected()
		if err != nil {
			return false, log.Errore(err)
		}
		if rows > 0 {
			return true, nil
		}
	}

	return false, nil
}

// ExpireAvailableNodes is an aggressive purging method to remove
// node entries who have skipped their keepalive for two times.
func ExpireAvailableNodes() {
	_, err := db.ExecDb(`
			delete
				from node_health
			where
				last_seen_active < now() - interval ? second
			`,
		config.HealthPollSeconds*5,
	)
	if err != nil {
		log.Errorf("ExpireAvailableNodes: failed to remove old entries: %+v", err)
	}
}

// ExpireNodesHistory cleans up the nodes history and is run by
// the active node.
func ExpireNodesHistory() error {
	_, err := db.ExecDb(`
			delete
				from node_health_history
			where
				first_seen_active < now() - interval ? hour
			`,
		config.Config.UnseenInstanceForgetHours,
	)
	return log.Errore(err)
}

func ReadAvailableNodes(onlyHttpNodes bool) (nodes [](*NodeHealth), err error) {
	extraInfo := ""
	peers := []string{}
	if onlyHttpNodes {
		extraInfo = string(ExecutionHttpMode)
	}
	peers, err = oraft.GetPeers()
	if err != nil {
		return nodes, log.Errore(err)
	}
	if len(peers) == 0 {
		return nodes, errors.New("raft peers is null")
	}

	whereCondition := ""
	if len(peers) == 1 {
		whereCondition = ""
	} else if len(peers) > 1 {
		whereCondition = "("
	}

	for i := 0; i < len(peers); i++ {
		hostPort := strings.Split(peers[i], ":")
		if len(hostPort) < 2 {
			continue
		}
		host := hostPort[0]
		if host == "127.0.0.1" {
			localIp, _ := util.GetLocalIP()
			if localIp != "" {
				host = localIp
			}
		}
		if i == 0 {
			whereCondition = whereCondition + fmt.Sprintf("(ip='%s' and raft_port='%s')", host, hostPort[1])
		} else if i == len(peers)-1 {
			whereCondition = whereCondition + " or " + fmt.Sprintf("(ip='%s' and raft_port='%s')", host, hostPort[1]) + ")"
		} else {
			whereCondition = whereCondition + " or " + fmt.Sprintf("(ip='%s' and raft_port='%s')", host, hostPort[1])
		}

	}

	sql := `select
					hostname, token, app_version, first_seen_active, last_seen_active, db_backend
				from
					node_health
				where
					last_seen_active > now() - interval ? second
					and ? in (extra_info, '') and %s
				order by
					hostname
			`
	query := fmt.Sprintf(sql, whereCondition)
	err = db.QueryDB(query, sqlutils.Args(config.HealthPollSeconds*2, extraInfo), func(m sqlutils.RowMap) error {
		nodeHealth := &NodeHealth{
			Hostname:        m.GetString("hostname"),
			Token:           m.GetString("token"),
			AppVersion:      m.GetString("app_version"),
			FirstSeenActive: m.GetString("first_seen_active"),
			LastSeenActive:  m.GetString("last_seen_active"),
			DBBackend:       m.GetString("db_backend"),
		}
		nodes = append(nodes, nodeHealth)
		return nil
	})
	return nodes, log.Errore(err)
}
