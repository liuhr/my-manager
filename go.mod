module github.com/github/my-manager

go 1.15

require (
	github.com/armon/go-metrics v0.4.1 // indirect
	github.com/codegangsta/inject v0.0.0-20150114235600-33e0aa1cb7c0 // indirect
	github.com/go-martini/martini v0.0.0-20170121215854-22fa46961aab
	github.com/go-sql-driver/mysql v1.7.0
	github.com/hashicorp/go-msgpack v1.1.6 // indirect
	github.com/hashicorp/raft v0.0.0-00010101000000-000000000000
	github.com/howeyc/gopass v0.0.0-20190910152052-7cb4b85ec19c
	github.com/martini-contrib/auth v0.0.0-20150219114609-fa62c19b7ae8
	github.com/martini-contrib/gzip v0.0.0-20151124214156-6c035326b43f
	github.com/martini-contrib/render v0.0.0-20150707142108-ec18f8345a11
	github.com/openark/golib v0.0.0-20210531070646-355f37940af8
	github.com/outbrain/golib v0.0.0-20200503083229-2531e5dbcc71
	github.com/oxtoacart/bpool v0.0.0-20190530202638-03653db5a59c // indirect
	github.com/patrickmn/go-cache v2.1.0+incompatible
	golang.org/x/crypto v0.0.0-20200728195943-123391ffb6de // indirect
	golang.org/x/sys v0.0.0-20200819171115-d785dc25833f // indirect
)

replace github.com/hashicorp/raft => github.com/openark/raft v0.0.0-20170918052300-fba9f909f7fe
