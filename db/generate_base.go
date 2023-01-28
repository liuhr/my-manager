package db

// generateSQLBase is lists of SQL statements required to build the  backend db

var generateSQLBase = []string{
	`
		CREATE TABLE IF NOT EXISTS node_health (
		  hostname varchar(128) CHARACTER SET ascii NOT NULL,
		  token varchar(128) NOT NULL,
		  ip char(50) NOT NULL,
		  raft_port int(11) NOT NULL,
		  last_seen_active timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP,
		  extra_info varchar(128) CHARACTER SET utf8 NOT NULL,
		  command varchar(128) CHARACTER SET utf8 NOT NULL,
		  app_version varchar(64) NOT NULL DEFAULT '',
		  first_seen_active timestamp NOT NULL DEFAULT '1971-01-01 00:00:00',
		  db_backend varchar(255) NOT NULL DEFAULT '',
		  incrementing_indicator bigint(20) NOT NULL DEFAULT '0',
		  PRIMARY KEY (hostname, token),
		  KEY last_seen_active_idx (last_seen_active)
		) ENGINE=InnoDB DEFAULT CHARSET=ascii
	`,

	`
		CREATE TABLE IF NOT EXISTS node_health_history (
			history_id bigint unsigned not null auto_increment,
			hostname varchar(128) CHARACTER SET ascii NOT NULL,
			token varchar(128) NOT NULL,
			first_seen_active timestamp NOT NULL,
			extra_info varchar(128) CHARACTER SET utf8 NOT NULL,
			command varchar(128) CHARACTER SET utf8 NOT NULL,
			app_version varchar(64) NOT NULL DEFAULT '',
			PRIMARY KEY (history_id),
			UNIQUE KEY hostname_token_idx_node_health_history (hostname,token),
			KEY first_seen_active_idx_node_health_history (first_seen_active)
		) ENGINE=InnoDB DEFAULT CHARSET=ascii
	`,

	`
		CREATE TABLE IF NOT EXISTS active_node (
  			anchor tinyint(3) unsigned NOT NULL,
  			hostname varchar(128) NOT NULL,
  			token varchar(128) NOT NULL,
  			last_seen_active timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP,
  			first_seen_active timestamp NOT NULL DEFAULT '1971-01-01 00:00:00',
  			PRIMARY KEY (anchor)
		) ENGINE=InnoDB DEFAULT CHARSET=ascii
	`,
}
