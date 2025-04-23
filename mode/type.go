package mode

import (
	"context"
	"log/slog"

	"github.com/khaledhikmat/vs-go/model"
	"github.com/khaledhikmat/vs-go/pipeline"
	"github.com/khaledhikmat/vs-go/service/config"
	"github.com/khaledhikmat/vs-go/service/data"
	"github.com/khaledhikmat/vs-go/service/lgr"
	"github.com/khaledhikmat/vs-go/service/orphan"
)

type Processor func(canxCtx context.Context,
	cfgSvc config.IService,
	dataSvc data.IService,
	orphanSvc orphan.IService,
	streamers []pipeline.Streamer,
	alerter pipeline.Alerter) error

func procStats(datasvc data.IService, stats interface{}) {
	switch stats := stats.(type) {
	case model.AgentsManagerStats:
		procAgentsManagerStats(datasvc, stats)
	case model.AgentStats:
		procAgentStats(datasvc, stats)
	case model.FramerStats:
		procFramerStats(datasvc, stats)
	case model.StreamerStats:
		procStreamerStats(datasvc, stats)
	default:
		lgr.Logger.Error(
			"unknown stats type",
			slog.Any("stats", stats),
		)
	}
}

func procAgentsManagerStats(datasvc data.IService, stats model.AgentsManagerStats) {
	err := datasvc.NewAgentsManagerStats(stats)
	if err != nil {
		lgr.Logger.Error(
			"failed to store agents manager stats",
			slog.Any("stats", stats),
			slog.Any("error", err),
		)
	}
}

func procAgentStats(datasvc data.IService, stats model.AgentStats) {
	err := datasvc.NewAgentStats(stats)
	if err != nil {
		lgr.Logger.Error(
			"failed to store agent stats",
			slog.Any("stats", stats),
			slog.Any("error", err),
		)
	}
}

func procFramerStats(datasvc data.IService, stats model.FramerStats) {
	err := datasvc.NewFramerStats(stats)
	if err != nil {
		lgr.Logger.Error(
			"failed to store framer stats",
			slog.Any("stats", stats),
			slog.Any("error", err),
		)
	}
}

func procStreamerStats(datasvc data.IService, stats model.StreamerStats) {
	err := datasvc.NewStreamerStats(stats)
	if err != nil {
		lgr.Logger.Error(
			"failed to store streamer stats",
			slog.Any("stats", stats),
			slog.Any("error", err),
		)
	}
}

func procError(datasvc data.IService, err interface{}) {
	errTemp := datasvc.NewError(err)
	if errTemp != nil {
		lgr.Logger.Error(
			"failed to store error",
			slog.Any("error", errTemp),
		)
	}
}
