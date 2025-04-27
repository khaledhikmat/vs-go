package pipeline

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/khaledhikmat/vs-go/model"
	"github.com/khaledhikmat/vs-go/service/lgr"
	"gocv.io/x/gocv"
)

func SimpleAlerter(canx context.Context, svcs ServicesFactory, errorStream chan interface{}, statsStream chan interface{}) chan AlertData {
	in := make(chan AlertData, 100)

	go func() {
		beginTime := time.Now().Unix()
		alerts := 0
		errors := 0

		defer close(in)

		flush := func() {
			// TODO:
		}
		defer flush()

		defer func() {
			endTime := time.Now().Unix()
			uptime := endTime - beginTime

			statsStream <- model.AlerterStats{
				Name:      "SimpleAlerter",
				Alerts:    alerts,
				Errors:    errors,
				Uptime:    uptime,
				Timestamp: time.Now().Unix(),
			}
		}()

		for {
			select {
			case <-canx.Done():
				lgr.Logger.Info(
					"alerter context cancelled",
				)
				return

			case <-time.After(time.Duration(time.Duration(svcs.CfgSvc.GetAgentAlerterPeriodicTimeout()) * time.Second)):
				// TODO: Retry webhooks if failures
				// Here we redo the ones that were originated from this alerter

				// Push stats
				statsStream <- model.AlerterStats{
					Name:      "simpleAlerter",
					Alerts:    alerts,
					Errors:    errors,
					Uptime:    time.Now().Unix() - beginTime,
					Timestamp: time.Now().Unix(),
				}

			case alert := <-in:
				alerts++

				alertImageURL := alert.FrameURL
				alertClipURL := alert.ClipURL
				// It is possible that the alert image and video URLs are already poupulated
				if alertClipURL == "" {
					var err error
					fn := fmt.Sprintf("%s/%s_alerted_frame_%d.jpg", svcs.CfgSvc.GetRecordingsFolder(), alert.Camera.ID, time.Now().Unix())
					// Store the alerted frame as an image
					gocv.IMWrite(fn, alert.Mat)
					// Store the alerted image possibly to a cloud storage
					alertImageURL, err = svcs.StorageSvc.StoreFile(fn)
					if err != nil {
						errors++
						errorStream <- model.GenError("simple_alerter",
							err,
							map[string]interface{}{},
							"error storing a clip %s",
							fn)
						continue
					}

					// Retrieve the video clip from VMS possibly to a cloud storage
					alertClipURL, err = svcs.VmsSvc.RetrieveClip(alert.Camera.ID, alert.Timestamp.Unix()-5, alert.Timestamp.Unix()+5)
					if err != nil {
						errors++
						errorStream <- model.GenError("simple_alerter",
							err,
							map[string]interface{}{},
							"error retrieving a clip from VMS %s",
							fn)
						continue
					}
				}

				// Send to a webhook
				payload := map[string]interface{}{
					"source":        alert.Camera.Name,
					"alertImageURL": alertImageURL,
					"alertClipURL":  alertClipURL,
					"label":         alert.Label,
					"confidence":    alert.Confidence,
					"timestamp":     time.Now().Format(time.RFC3339),
				}
				lgr.Logger.Info(
					"alert payload",
					slog.Any("payload", payload),
				)

				err := svcs.WebhookSvc.Post(payload)
				if err != nil {
					errors++
					errorStream <- model.GenError("simple_alerter",
						err,
						map[string]interface{}{},
						"error posting to webhook %v",
						payload)
					continue
				}
			}
		}
	}()

	return in
}
