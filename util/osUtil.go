package util

import (
	//"bytes"
	//"context"
	"fmt"
	"io/ioutil"
	"net"
	"os"
	"os/exec"
	"strings"
	"syscall"
	"time"

	"github.com/openark/golib/log"
)

var (
	timeout  = 10 * time.Second
	EmptyEnv = []string{}
)

func init() {
	osPath := os.Getenv("PATH")
	os.Setenv("PATH", fmt.Sprintf("%s:/usr/sbin:/usr/bin:/sbin:/bin", osPath))
}

// CommandRun executes some text as a command. This is assumed to be
// text that will be run by a shell so we need to write out the
// command to a temporary file and then ask the shell to execute
// it, after which the temporary file is removed.
func RunCommandOutput(commandText string, arguments ...string) (string, error) {
	// show the actual command we have been asked to run
	log.Infof("CommandRun(%v,%+v)", commandText, arguments)

	cmd, shellScript, err := generateShellScript(commandText, arguments...)
	defer os.Remove(shellScript)
	if err != nil {
		return "", log.Errore(err)
	}

	var waitStatus syscall.WaitStatus

	log.Infof("CommandRun/running: %s", strings.Join(cmd.Args, " "))
	cmdOutput, err := cmd.CombinedOutput()
	log.Infof("CommandRun: %s\n", string(cmdOutput))
	if err != nil {
		// Did the command fail because of an unsuccessful exit code
		if exitError, ok := err.(*exec.ExitError); ok {
			waitStatus = exitError.Sys().(syscall.WaitStatus)
			log.Errorf("CommandRun: failed. exit status %d", waitStatus.ExitStatus())
		}

		return "", log.Errore(fmt.Errorf("(%s) %s", err.Error(), cmdOutput))
	}

	// Command was successful
	waitStatus = cmd.ProcessState.Sys().(syscall.WaitStatus)
	log.Infof("CommandRun successful. exit status %d", waitStatus.ExitStatus())

	return strings.Replace(string(cmdOutput), "\n", "", -1), nil
}

// generateShellScript generates a temporary shell script based on
// the given command to be executed, writes the command to a temporary
// file and returns the exec.Command which can be executed together
// with the script name that was created.
func generateShellScript(commandText string, arguments ...string) (*exec.Cmd, string, error) {
	commandBytes := []byte(commandText)
	tmpFile, err := ioutil.TempFile("", "manager-process-cmd-")
	if err != nil {
		return nil, "", log.Errorf("generateShellScript() failed to create TempFile: %v", err.Error())
	}
	// write commandText to temporary file
	ioutil.WriteFile(tmpFile.Name(), commandBytes, 0640)
	shellArguments := append([]string{}, tmpFile.Name())
	shellArguments = append(shellArguments, arguments...)

	cmd := exec.Command("bash", shellArguments...)
	//cmd.Env = env

	return cmd, tmpFile.Name(), nil
}

//no output
func RunCommandNoOutput(commandText string) error {
	cmd, tmpFileName, err := execCmd(commandText)
	if err != nil {
		return err
	}
	defer os.Remove(tmpFileName)
	err = cmd.Start()
	if err != nil {
		return err
	}
	err = cmd.Wait()
	return err
}

func execCmd(commandText string) (*exec.Cmd, string, error) {
	commandBytes := []byte(commandText)
	tmpFile, err := ioutil.TempFile("", "manager-cmd-")
	if err != nil {
		return nil, "", log.Errore(err)
	}
	ioutil.WriteFile(tmpFile.Name(), commandBytes, 0644)
	log.Debugf("execCmd: %s", commandText)
	return exec.Command("bash", tmpFile.Name()), tmpFile.Name(), nil
}

func GetLocalIP() (ipv4 string, err error) {
	var (
		addrs   []net.Addr
		addr    net.Addr
		ipNet   *net.IPNet // IP地址
		isIpNet bool
	)
	if addrs, err = net.InterfaceAddrs(); err != nil {
		return
	}
	for _, addr = range addrs {
		// 这个网络地址是IP地址: ipv4, ipv6
		if ipNet, isIpNet = addr.(*net.IPNet); isIpNet && !ipNet.IP.IsLoopback() {
			// 跳过IPV6
			if ipNet.IP.To4() != nil {
				ipv4 = ipNet.IP.String() // 192.168.1.1
				return
			}
		}
	}
	return
}


func LookupHost(name string) (addrs []string, err error) {
	addr, err := net.LookupHost(name)
	return addr,err
}
