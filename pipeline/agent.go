package pipeline

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/khaledhikmat/vs-go/model"
	"github.com/khaledhikmat/vs-go/service/lgr"
)

func Agent(canxCtx context.Context,
	svcs ServicesFactory,
	errorStream chan interface{},
	statsStream chan interface{},
	alertStream chan AlertData,
	camera model.Camera,
	streamers []Streamer) error {
	agentID := uuid.NewString()
	lgr.Logger.Info(
		"agent starting....",
		slog.String("agentID", agentID),
		slog.String("camera", camera.Name),
		slog.String("frameType", camera.FramerType),
		slog.String("rtsp", camera.RtspURL),
		slog.Int("streamers", len(streamers)),
	)

	// OTEL stats
	var agentStartTime = time.Now().Unix()
	agentStats := model.AgentStats{
		ID:     agentID,
		Camera: camera.Name,
		Uptime: agentStartTime,
	}

	// Update the camera agent id
	err := svcs.DataSvc.UpdateCameraAgentID(camera.ID, agentID)
	if err != nil {
		return fmt.Errorf("error updating camera agent id: %w", err)
	}

	// Setup the stream channels
	streamChannels := []chan FrameData{}
	for _, streamer := range streamers {
		streamChannels = append(streamChannels, streamer(canxCtx, svcs, camera, errorStream, statsStream, alertStream))
	}

	// Start the agent frame capturer
	framer(canxCtx, svcs, camera, errorStream, statsStream, streamChannels)

	// Monitor cancellations and update heartbeats
	for {
		select {
		case <-canxCtx.Done():
			lgr.Logger.Info(
				"agent context cancelled",
			)
			return nil

		case <-time.After(time.Duration(time.Duration(svcs.CfgSvc.GetAgentsManagerPeriodicTimeout()) * time.Second)):
			// Update the agent heartbeat so that the agents monitor would know
			// that the agent is alive and kicking and does need to be re-scheduled
			err := svcs.DataSvc.UpdateCameraAgentHeartbeat(camera.ID)
			if err != nil {
				lgr.Logger.Error(
					"error updating camera agent heartbeat",
					slog.Any("error", err),
				)
			}

			agentStats.Uptime = time.Now().Unix() - agentStartTime

			// Send the stats to OTEL
			statsStream <- agentStats
		}
	}
}
