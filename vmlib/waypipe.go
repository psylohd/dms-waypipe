package vmlib

import (
	"fmt"
	"os/exec"
	"syscall"
	"time"
)

type LaunchResult struct {
	PID         int
	WindowTitle string
}

func LaunchApp(vmName string, execCmd string, vmCfg VMConfig) (*LaunchResult, error) {
	dom, err := GetDomainInfo(vmName)
	if err != nil {
		return nil, fmt.Errorf("GetDomainInfo: %w", err)
	}

	if dom.State != "running" {
		return nil, fmt.Errorf("VM %s is not running (state: %s)", vmName, dom.State)
	}

	args := buildWaypipeArgs(vmName, execCmd, dom, vmCfg)

	cmd := exec.Command("waypipe", args...)
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Setsid: true,
	}
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("waypipe start: %w", err)
	}

	pid := cmd.Process.Pid

	time.Sleep(2 * time.Second)

	return &LaunchResult{
		PID:         pid,
		WindowTitle: fmt.Sprintf("[VM:%s] %s", vmName, execCmd),
	}, nil
}

func buildWaypipeArgs(vmName, execCmd string, dom *DomainInfo, vmCfg VMConfig) []string {
	var args []string

	sshTarget := vmCfg.SSHTarget
	if sshTarget == "" {
		host := vmCfg.SSHHost
		if host == "" {
			host = vmName
		}
		user := vmCfg.SSHUser
		if user == "" {
			user = "sec"
		}
		port := ""
		if vmCfg.SSHPort != 0 && vmCfg.SSHPort != 22 {
			port = fmt.Sprintf(":%d", vmCfg.SSHPort)
		}
		sshTarget = fmt.Sprintf("%s@%s%s", user, host, port)
	}

	args = append(args, "--title-prefix", fmt.Sprintf("[VM:%s]", vmName))
	if vmCfg.WaypipeFlags != "" {
		args = append(args, vmCfg.WaypipeFlags)
	}
	args = append(args, "ssh", sshTarget)
	args = append(args, execCmd)

	return args
}
