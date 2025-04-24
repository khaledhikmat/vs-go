package pipeline

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/khaledhikmat/vs-go/service/config"
	"github.com/khaledhikmat/vs-go/service/lgr"
	"gocv.io/x/gocv"
)

func SimpleAlerter(canx context.Context, cfgSvc config.IService, _ chan interface{}, _ chan interface{}) chan AlertData {
	in := make(chan AlertData, 100)

	go func() {
		defer close(in)

		flush := func() {
			// TODO:
		}
		defer flush()

		for {
			select {
			case <-canx.Done():
				lgr.Logger.Info(
					"alerter context cancelled",
				)
				return

			case <-time.After(time.Duration(time.Duration(cfgSvc.GetAgentAlerterWebhookRetry()) * time.Second)):
				// TODO: Retry webhooks if failures

			case alert := <-in:
				// Add your alert logic here
				// Store the alerted frame as an image
				gocv.IMWrite(fmt.Sprintf("%s/%s_alerted_frame_%d.jpg", cfgSvc.GetRecordingsFolder(), alert.Camera.ID, time.Now().Unix()), alert.Mat)
				// Upload the alerted image to a cloud storage
				// Retrieve the video clip from VMS
				// Upload the alerted clip to a cloud storage
				// Send to a webhook
				lgr.Logger.Info(
					"alert detected",
					slog.String("camera", alert.Camera.Name),
					slog.String("label", alert.Label),
					slog.Float64("confidence", float64(alert.Confidence)),
					slog.Time("timestamp", alert.Timestamp),
				)

				payload := map[string]interface{}{
					"source":        alert.Camera.Name,
					"alertImageURL": "alertImageUrl",
					"alertVideoURL": "alertVideoUrl",
					"label":         alert.Label,
					"confidence":    alert.Confidence,
					"timestamp":     time.Now().Format(time.RFC3339),
				}
				lgr.Logger.Info(
					"alert payload",
					slog.Any("payload", payload),
				)

				// body, _ := json.Marshal(payload)
				// http.Post(webhookURL, "application/json", bytes.NewBuffer(body))

				time.Sleep(10 * time.Millisecond) // Simulate processing time
			}
		}
	}()

	return in
}
