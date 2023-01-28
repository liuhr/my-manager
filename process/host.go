package process

import (
	"os"

	"github.com/openark/golib/log"
)

var ThisHostname string

func init() {
	var err error
	ThisHostname, err = os.Hostname()
	if err != nil {
		log.Fatalf("Cannot resolve self hostname; required. Aborting. %+v", err)
	}
}
