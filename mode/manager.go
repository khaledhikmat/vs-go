package mode

import (
	"context"
	"log/slog"
	"math/rand"
	"time"

	"github.com/khaledhikmat/vs-go/model"
	"github.com/khaledhikmat/vs-go/pipeline"
	"github.com/khaledhikmat/vs-go/service/config"
	"github.com/khaledhikmat/vs-go/service/data"
	"github.com/khaledhikmat/vs-go/service/lgr"
	"github.com/khaledhikmat/vs-go/service/orphan"
	"golang.org/x/xerrors"
)

type agent struct {
	Camera model.Camera
	CanxFn context.CancelFunc
}

// The agents manager is responsible for running the agents
func Manager(canxCtx context.Context, cfgSvc config.IService, dataSvc data.IService, orphanSvc orphan.IService, streamers []pipeline.Streamer, alerter pipeline.Alerter) error {
	// Subscribe to the orphan service to receive orphaned cameras
	orphanStream, err := orphanSvc.Subscribe()
	if err != nil {
		return err
	}

	// Create an error stream
	errorStream := make(chan interface{})
	defer close(errorStream)

	// Create agents manager stats stream
	statsStream := make(chan interface{})
	defer close(statsStream)

	// Create an alerter stream using a simple alerter
	// Alerter functions must comply with Alerter signature (check pipeline/type.go)
	// So you can use any other alerter but the base library provides a simple one
	alertStream := alerter(canxCtx, cfgSvc, errorStream, statsStream)

	// Register one or more streamers (but you can use any other streamer)
	// Streamer functions must comply with Streamer signature (check pipeline/type.go)
	// So you can use any other streamer. Please see sample streamers in pipeline/streamers.go.

	// Store running agents and manager stats in memory (convert to OTEL)
	var agentsManagerStartTime = time.Now().Unix()
	var runningAgents = map[string]agent{}

	// OTEL stats
	agentsManagerStats := model.AgentsManagerStats{
		TotalRunningAgentsUptime: agentsManagerStartTime,
	}

	// Wait for cancellation, timeout or orphaned cameras
	for {
		select {
		case <-canxCtx.Done():
			lgr.Logger.Info(
				"agents manager context cancelled",
			)
			goto resume

		case orphanedCameras := <-orphanStream:
			agentsManagerStats.TotalOrphanedRequests++
			unAccomodatedCameras := []model.Camera{}

			// Run each camera's agent using configured streamers
			for _, camera := range orphanedCameras {
				if len(runningAgents) >= cfgSvc.GetMaxAgentsPerPod() {
					unAccomodatedCameras = append(unAccomodatedCameras, camera)
					continue
				}

				// Create a child context for the agent
				// to allow us to cancel an agent
				// without cancelling the main context
				agentCanxCtx, agentCanxFn := context.WithCancel(canxCtx)

				var agentStartErr error

				go func() {
					agentStartErr = pipeline.Agent(agentCanxCtx, cfgSvc, dataSvc, errorStream, statsStream, alertStream, camera, streamers)
					if agentStartErr != nil {
						procError(dataSvc, model.GenError("agents_manager",
							agentStartErr,
							map[string]interface{}{},
							"error starting agent for camera: %s",
							camera.Name))
					}
				}()

				// If the agent fails to start, make sure to cancel the agent's context
				if agentStartErr != nil {
					// Cancel the agent's context
					agentCanxFn()
					continue
				}

				// Store the agent in memory
				runningAgents[camera.ID] = agent{
					Camera: camera,
					CanxFn: agentCanxFn,
				}
			}

			// If there are unaccommodated cameras, let it be known
			if len(unAccomodatedCameras) > 0 {
				lgr.Logger.Debug(
					"agents pod could not accommodate these cameras.",
					slog.Int("runningAgents", len(runningAgents)),
					slog.Int("maxAgentsPerPod", cfgSvc.GetMaxAgentsPerPod()),
					slog.Int("unAccomodatedAgents", len(unAccomodatedCameras)),
				)
			}

			if len(runningAgents) >= cfgSvc.GetMaxAgentsPerPod() {
				agentsManagerStats.TotalOrphanedRequestUnsubscriptions++
				// Unsubscribe from the orphan service so that we don't get more cameras
				// We want to make sure that we don't consume events that may deprive
				// other agent pods from getting camera requests
				err = orphanSvc.Unsubscribe()
				if err != nil {
					lgr.Logger.Error(
						"error unsubscribing from orphan service",
						slog.Any("error", xerrors.New(err.Error())),
					)
				}
			}

		case <-time.After(time.Duration(time.Duration(cfgSvc.GetAgentsManagerPeriodicTimeout()) * time.Second)):
			// Monitor my running agents to see if they need to be stopped (due to exclusion)
			// Convert runningAgents to runningAgentIDs
			runningAgentIDs := make([]string, 0, len(runningAgents))
			for id := range runningAgents {
				runningAgentIDs = append(runningAgentIDs, id)
			}

			// Retrieve cameras from the data service
			cameras, err := dataSvc.RetrieveCamerasByIDs(runningAgentIDs)
			if err != nil {
				lgr.Logger.Error(
					"error subscribing to orphan service",
					slog.Any("error", xerrors.New(err.Error())),
				)
				continue
			}

			// I think it is better to centralize the logic in the agents manager
			// as opposed to having each agent monitor its own commands
			// Go through the running agents and see if they can be excluded
			for _, camera := range cameras {
				if camera.Excluded {
					lgr.Logger.Debug(
						"camera is in exclusion list",
						slog.String("cameraID", camera.ID),
					)
					removeRandomAgent(runningAgents)
				}
			}

			if len(runningAgents) < cfgSvc.GetMaxAgentsPerPod() {
				// If we have less than the max agents, we can re-subscribe to the orphan service
				// Re-subscribe to the orphan service so that we can get more cameras
				agentsManagerStats.TotalOrphanedRequestSubscriptions++
				_, err = orphanSvc.Subscribe()
				if err != nil {
					lgr.Logger.Error(
						"error subscribing to orphan service",
						slog.Any("error", xerrors.New(err.Error())),
					)
				}
			}

			agentsManagerStats.TotalRunningAgentsUptime = time.Now().Unix() - agentsManagerStartTime
			agentsManagerStats.TotalRunningAgents += int64(len(runningAgents))
			if agentsManagerStats.TotalRunningAgentsUptime > 0 {
				uptimeInMinutes := float64(agentsManagerStats.TotalRunningAgentsUptime) / 60.0
				agentsManagerStats.AvgRunningAgentsPerMin = float64(agentsManagerStats.TotalRunningAgents) / uptimeInMinutes
			} else {
				agentsManagerStats.AvgRunningAgentsPerMin = 0.0 // Avoid division by zero
			}

			// Send the stats to OTEL
			procStats(dataSvc, agentsManagerStats)

		case s := <-statsStream:
			procStats(dataSvc, s)

		case e := <-errorStream:
			procError(dataSvc, e)
		}
	}

	// Wait in a non-blocking way for `waitOnShutdown` for all the go routines to exit
	// This is needed because the go routines may need to report errors as they are existing
resume:
	lgr.Logger.Info(
		"agents manager is waiting for all go routines to exit",
	)

	// The only way to exit the main function is to wait for the shutdown
	// duration
	timer := time.NewTimer(time.Duration(cfgSvc.GetModeMaxShutdownTime()) * time.Second)
	defer timer.Stop()

	for {
		select {
		case <-timer.C:
			// Timer expired, proceed with shutdown
			lgr.Logger.Info(
				"agents manager shutdown waiting period expired. Exiting now",
				slog.Duration("period", time.Duration(cfgSvc.GetModeMaxShutdownTime())*time.Second),
			)

			return nil

		case s := <-statsStream:
			procStats(dataSvc, s)

		case e := <-errorStream:
			procError(dataSvc, e)
		}
	}
}

// Randomly remove an agent from runningAgents
func removeRandomAgent(runningAgents map[string]agent) {
	// Seed the random number generator
	rand.Seed(time.Now().UnixNano())

	// Get all keys from the map
	keys := make([]string, 0, len(runningAgents))
	for key := range runningAgents {
		keys = append(keys, key)
	}

	// If there are no agents, return
	if len(keys) == 0 {
		return
	}

	// Pick a random key
	randomKey := keys[rand.Intn(len(keys))]

	// Cancel the agent's context
	runningAgents[randomKey].CanxFn()

	// Remove the agent from the map
	delete(runningAgents, randomKey)

	// Log the removal
	lgr.Logger.Debug(
		"removed a random agent",
		slog.String("cameraID", randomKey),
	)
}
