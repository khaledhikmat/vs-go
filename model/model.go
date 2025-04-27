package model

import (
	"fmt"
	"runtime/debug"
)

type CustomError struct {
	Processor  string                 `json:"processor"`
	Inner      error                  `json:"innerError"`
	Message    string                 `json:"message"`
	StackTrace string                 `json:"stackTrace"`
	Misc       map[string]interface{} `json:"misc"`
}

func GenError(proc string, err error, misc map[string]interface{}, messagef string, args ...interface{}) CustomError {
	return CustomError{
		Processor:  proc,
		Inner:      err,
		Message:    fmt.Sprintf(messagef, args...),
		StackTrace: string(debug.Stack()),
		Misc:       misc,
	}
}

type Camera struct {
	ID            string `json:"id"`
	VMSIdentifier string `json:"vmsId"`
	Name          string `json:"name"`
	RtspURL       string `json:"rtspUrl"`
	FramerType    string `json:"framerType"`
	Excluded      bool   `json:"excluded"`
	AgentID       string `json:"agentId"`       // The agent id that is currently controlling this camera
	StartupTime   int64  `json:"startupTime"`   // The startup time of the agent
	LastHeartBeat int64  `json:"lastHeartbeat"` // The last heartbeat time of the agent
	Uptime        int64  `json:"uptime"`        // The uptime of the agent
}

type AlerterStats struct {
	Name      string `json:"name"`
	Alerts    int    `json:"alerts"`
	Errors    int    `json:"errors"`
	Uptime    int64  `json:"uptime"`
	Timestamp int64  `json:"timestamp"`
}

type StreamerStats struct {
	Name        string  `json:"name"`
	Worker      int     `json:"worker"`
	Camera      string  `json:"camera"`
	FPS         int     `json:"fps"`
	Frames      int     `json:"frames"`
	Errors      int     `json:"errors"`
	Uptime      int64   `json:"uptime"`
	AvgProcTime float64 `json:"avgProcTime"`
	Timestamp   int64   `json:"timestamp"`
}

type FramerStats struct {
	Name      string `json:"name"`
	Camera    string `json:"camera"`
	FPS       int    `json:"fps"`
	Frames    int    `json:"frames"`
	Errors    int    `json:"errors"`
	Uptime    int64  `json:"uptime"`
	Timestamp int64  `json:"timestamp"`
}

type AgentStats struct {
	ID        string `json:"id"`     // Agent ID
	Camera    string `json:"camera"` // Camera name
	Uptime    int64  `json:"uptime"` // Uptime of the agent
	Timestamp int64  `json:"timestamp"`
}

type AgentsManagerStats struct {
	TotalOrphanedRequests               int64   `json:"orphanedRequests"`
	TotalOrphanedRequestSubscriptions   int64   `json:"orphanedRequestSubscriptions"`
	TotalOrphanedRequestUnsubscriptions int64   `json:"orphanedRequestUnsubscriptions"`
	TotalRunningAgents                  int64   `json:"runningAgents"`
	TotalRunningAgentsUptime            int64   `json:"runningAgentsUptime"`
	AvgRunningAgentsPerMin              float64 `json:"avgRunningAgentsPerMin"`
	Timestamp                           int64   `json:"timestamp"`
}
