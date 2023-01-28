package process

import (
	"github.com/github/my-manager/config"
	"github.com/github/my-manager/db"
	"github.com/github/my-manager/util"
	"github.com/openark/golib/log"
	"github.com/openark/golib/sqlutils"
)

// AttemptElection tries to grab leadership (become active node)
func AttemptElection() (bool, error) {
	{
		sqlResult, err := db.ExecDb(`
		insert ignore into active_node (
				anchor, hostname, token, first_seen_active, last_seen_active
			) values (
				1, ?, ?, now(), now()
			)
		`,
			ThisHostname, util.ProcessToken.Hash,
		)
		if err != nil {
			return false, log.Errore(err)
		}
		rows, err := sqlResult.RowsAffected()
		if err != nil {
			return false, log.Errore(err)
		}
		if rows > 0 {
			// We managed to insert a row
			return true, nil
		}
	}
	{
		// takeover from a node that has been inactive
		sqlResult, err := db.ExecDb(`
			update active_node set
				hostname = ?,
				token = ?,
				first_seen_active=now(),
				last_seen_active=now()
			where
				anchor = 1
			  and last_seen_active < (now() - interval ? second)
		`,
			ThisHostname, util.ProcessToken.Hash, config.ActiveNodeExpireSeconds,
		)
		if err != nil {
			return false, log.Errore(err)
		}
		rows, err := sqlResult.RowsAffected()
		if err != nil {
			return false, log.Errore(err)
		}
		if rows > 0 {
			// We managed to update a row: overtaking a previous leader
			return true, nil
		}
	}
	{
		// Update last_seen_active is this very node is already the active node
		sqlResult, err := db.ExecDb(`
			update active_node set
				last_seen_active=now()
			where
				anchor = 1
				and hostname = ?
				and token = ?
		`,
			ThisHostname, util.ProcessToken.Hash,
		)
		if err != nil {
			return false, log.Errore(err)
		}
		rows, err := sqlResult.RowsAffected()
		if err != nil {
			return false, log.Errore(err)
		}
		if rows > 0 {
			// Reaffirmed our own leadership
			return true, nil
		}
	}
	return false, nil
}

// ElectedNode returns the details of the elected node, as well as answering the question "is this process the elected one"?
func ElectedNode() (node NodeHealth, isElected bool, err error) {
	query := `
		select
			hostname,
			token,
			first_seen_active,
			last_seen_Active
		from
			active_node
		where
			anchor = 1
		`
	err = db.QueryDBRowsMap(query, func(m sqlutils.RowMap) error {
		node.Hostname = m.GetString("hostname")
		node.Token = m.GetString("token")
		node.FirstSeenActive = m.GetString("first_seen_active")
		node.LastSeenActive = m.GetString("last_seen_active")

		return nil
	})

	isElected = (node.Hostname == ThisHostname && node.Token == util.ProcessToken.Hash)
	return node, isElected, log.Errore(err)
}
