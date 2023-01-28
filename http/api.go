package http

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"

	"github.com/go-martini/martini"
	"github.com/martini-contrib/auth"
	"github.com/martini-contrib/render"

	"github.com/github/my-manager/config"
	"github.com/github/my-manager/process"
	"github.com/github/my-manager/raft"
	"github.com/github/my-manager/util"
)

var apiSynonyms = map[string]string{}

// APIResponseCode is an OK/ERROR response code
type APIResponseCode int

var registeredPaths = []string{}

const (
	ERROR APIResponseCode = iota
	OK
)

type HttpAPI struct {
	URLPrefix string
}

var API HttpAPI = HttpAPI{}

func (this *APIResponseCode) MarshalJSON() ([]byte, error) {
	return json.Marshal(this.String())
}

func (this *APIResponseCode) String() string {
	switch *this {
	case ERROR:
		return "ERROR"
	case OK:
		return "OK"
	}
	return "unknown"
}

// HttpStatus returns the respective HTTP status for this response
func (this *APIResponseCode) HttpStatus() int {
	switch *this {
	case ERROR:
		return http.StatusInternalServerError
	case OK:
		return http.StatusOK
	}
	return http.StatusNotImplemented
}

// APIResponse is a response returned as JSON to various requests.
type APIResponse struct {
	Code    APIResponseCode
	Message string
	Details interface{}
}

func Respond(r render.Render, apiResponse *APIResponse) {
	r.JSON(apiResponse.Code.HttpStatus(), apiResponse)
}

// A configurable endpoint that can be for regular status checks or whatever.  While similar to
// Health() this returns 500 on failure.  This will prevent issues for those that have come to
// expect a 200
// It might be a good idea to deprecate the current Health() behavior and roll this in at some
// point
func (this *HttpAPI) StatusCheck(params martini.Params, r render.Render, req *http.Request) {
	health, err := process.HealthTest()
	if err != nil {
		r.JSON(500, &APIResponse{Code: ERROR, Message: fmt.Sprintf("Application node is unhealthy %+v", err), Details: health})
		return
	}
	Respond(r, &APIResponse{Code: OK, Message: fmt.Sprintf("Application node is healthy"), Details: health})
}

func (this *HttpAPI) registerSingleAPIRequest(m *martini.ClassicMartini, path string, handler martini.Handler, allowProxy bool) {
	registeredPaths = append(registeredPaths, path)
	fullPath := fmt.Sprintf("%s/api/%s", this.URLPrefix, path)

	//if allowProxy && config.Config.RaftEnabled {
	//	m.Get(fullPath, raftReverseProxy, handler)
	//} else {
	m.Get(fullPath, handler)
	//}
}

func (this *HttpAPI) getSynonymPath(path string) (synonymPath string) {
	pathBase := strings.Split(path, "/")[0]
	if synonym, ok := apiSynonyms[pathBase]; ok {
		synonymPath = fmt.Sprintf("%s%s", synonym, path[len(pathBase):])
	}
	return synonymPath
}

func (this *HttpAPI) registerAPIRequestInternal(m *martini.ClassicMartini, path string, handler martini.Handler, allowProxy bool) {
	this.registerSingleAPIRequest(m, path, handler, allowProxy)

	if synonym := this.getSynonymPath(path); synonym != "" {
		this.registerSingleAPIRequest(m, synonym, handler, allowProxy)
	}
}

func (this *HttpAPI) registerAPIRequestNoProxy(m *martini.ClassicMartini, path string, handler martini.Handler) {
	this.registerAPIRequestInternal(m, path, handler, false)
}

func (this *HttpAPI) GetAppVersion(params martini.Params, r render.Render, req *http.Request) {
	version := config.NewAppVersion()
	if version != "" {
		r.JSON(200, &APIResponse{Code: OK, Details: version})
		return
	}
	Respond(r, &APIResponse{Code: ERROR, Message: "can not find version"})
	return
}

func (this *HttpAPI) registerAPIRequest(m *martini.ClassicMartini, path string, handler martini.Handler) {
	this.registerAPIRequestInternal(m, path, handler, true)
}

func (this *HttpAPI) CommonRequest(params martini.Params, r render.Render, req *http.Request) {
    var findFlag bool
    var script string
    var param string
    var outputFlag string
    process := config.Config.Processes
    if len(process) < 1 {
        err := fmt.Errorf("scripts in Processes is null")
        r.JSON(500, &APIResponse{Code: ERROR, Message: err.Error()})
        return
    }
    defer req.Body.Close()
    body, err := ioutil.ReadAll(req.Body)
    if err != nil {
        r.JSON(500, &APIResponse{Code: ERROR, Message: err.Error()})
        return
    }
    initInfo := string(body)
    initInfoList := strings.Split(initInfo, "&")
    if len(initInfoList) == 0 {
        err := fmt.Errorf("parameter can not be null")
        r.JSON(500, &APIResponse{Code: ERROR, Message: err.Error()})
        return
    }
    for _, v := range initInfoList {
        var dat map[string]string
        err := json.Unmarshal([]byte(v),&dat)
        if err != nil {
            r.JSON(500, &APIResponse{Code: ERROR, Message: err.Error() + " " + "Unmarshal params failed"})
            return
        }
        for _,vmap := range process {
            if v, ok := dat["key"]; ok {
                if vmap["key"] == v {
                    script = vmap["script"]
                    param = vmap["param"]
                    outputFlag = vmap["outputFlag"]
                    findFlag = true
                    break
                }
            } else {
                r.JSON(500, &APIResponse{Code: ERROR, Message: "comman api must add 'key' param"})
                return
            }
            
        }

        if !findFlag {
            continue
        } 

        for _, prm := range strings.Split(param, ",") {
                if v, ok := dat[prm];ok {
                    script = strings.Replace(script, fmt.Sprintf("{%s}", prm), v, -1)
                }
        }

        if len(script) == 0 {
            continue
        }
        if outputFlag == "1" {
            row, err := util.RunCommandOutput(script)
            if err != nil {
                r.JSON(500, &APIResponse{Code: ERROR, Message: err.Error()})
                return
            }
            r.JSON(200, &APIResponse{Code: OK, Details: row})
            return    
        } else {
            err := util.RunCommandNoOutput(script)
            if err != nil {
                r.JSON(500, &APIResponse{Code: ERROR, Message: err.Error()})
                return
            }
            r.JSON(200, &APIResponse{Code: OK, Details: ""})
            return
        }            
    }

    if !findFlag || len(script) == 0 {
        r.JSON(500, &APIResponse{Code: ERROR, Message: "find no script to run"})
        return
    } 
    r.JSON(500, &APIResponse{Code: ERROR, Details: fmt.Sprintf("script :%s outputFlag:%s", script, outputFlag)})
    return
}

// RaftFollowerHealthReport is initiated by followers to report their identity and health to the raft leader.
func (this *HttpAPI) RaftFollowerHealthReport(params martini.Params, r render.Render, req *http.Request, user auth.User) {
	if !oraft.IsRaftEnabled() {
		Respond(r, &APIResponse{Code: ERROR, Message: "raft-state: not running with raft setup"})
		return
	}
	err := oraft.OnHealthReport(params["authenticationToken"], params["raftBind"], params["raftAdvertise"])
	if err != nil {
		Respond(r, &APIResponse{Code: ERROR, Message: fmt.Sprintf("Cannot create snapshot: %+v", err)})
		return
	}
	r.JSON(http.StatusOK, "health reported")
}

// RegisterRequests makes for the de-facto list of known API calls
func (this *HttpAPI) RegisterRequests(m *martini.ClassicMartini) {
	var apiEndpoint string
	this.registerAPIRequestNoProxy(m, "raft-follower-health-report/:authenticationToken/:raftBind/:raftAdvertise", this.RaftFollowerHealthReport)
	this.registerAPIRequest(m, "version", this.GetAppVersion)
	if config.Config.ApiEndpoint != "" {
		apiEndpoint = config.Config.ApiEndpoint
	} else {
		apiEndpoint = config.DefaultApiEndpoint
	}
	m.Post(apiEndpoint, this.CommonRequest)

	// Configurable status check endpoint
	if config.Config.StatusEndpoint == config.DefaultStatusAPIEndpoint {
		this.registerAPIRequestNoProxy(m, "status", this.StatusCheck)
	} else {
		m.Get(config.Config.StatusEndpoint, this.StatusCheck)
	}
}
