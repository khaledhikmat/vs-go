The `video-sureveillance` backend is written in Golang. It is a combination of API Endpoints and background processing.

This implemenation is very generic and provides a general framework for implementing a Multi-streamers type of applications. 

The best use of this is as a base library for other video surveillance implementions where the main design principles need to be applied. 

## Architecture

 I will detail (a little later) a very generous description of how this library is implemented and where/how you can extend it.

In the meantime, please note the following:
- The cameras database is provided as a JSON file hard-coded in the config service as a file in `../settings/cameras.json` file. Here is a sample:

```json
[
  {
    "id": "camera_1",
    "vmsId": "uuid-1234-5678-90ab-cdef12366623",
    "name": "camera_1",
    "rtspUrl": "rtsp://admin:password@192.168.1.310:554/cam/realmonitor?channel=1\u0026subtype=0",
    "framerType": "random",
    "excluded": false,
    "agentId": "75008eef-9ca3-458a-8ca0-6df6535724bd",
    "startupTime": 1745179677,
    "lastHeartbeat": 1745181027,
    "uptime": 1350
  },
  {
    "id": "camera_2",
    "vmsId": "uuid-1234-5678-90ab-cdef12345678",
    "name": "camera_2",
    "rtspUrl": "rtsp://admin:pwd@192.168.1.xxx:554/cam/realmonitor?channel=1\u0026subtype=0",
    "framerType": "random",
    "excluded": false,
    "agentId": "",
    "startupTime": 0,
    "lastHeartbeat": 0,
    "uptime": 0
  }
]
```

- The recordings folder is hard coded in `../recordings` in the config service. This folder is used to record MP4 clips (if desired) and also to store alerted JPEG files.
- The framework creates a software agent for each camera which is responsible for pulling RTSP stream from the camera via a framer, running the RTSP stream via a pipeline that consists of one or more streamers and alerting, via an alerter, when a streamer detects an anomaly. Framers, streamers and alerters can be (and should be) overridden.    
- In order to build a complete video surveillance system, there are two mode processors: `agents-manager` and `agents-monitor`. These can run as separate processors, or, in Docker orchestrator such as K8s for example, they run as containers. 
- The `agents-manager` subscribes to an orphan service that streams orphan requests. The `agents-manager` instantiates as many agents as needed to satisfy the orphan requests. For reference, orphan requests are collections of cameras that do not have agents to them. 
- The `agents-manager` has a configuration that represents the max number of agents within a specific pod. Once this number is reached, the `agents-manager` unsubsrcibes from the orphan service so that it does not deprive other `agents-manager` pods from getting orphan requests.
- Orphan requests are received from the `agents-monitor` which runs in a separate process to monitor agents with no agents or abandoned agents. To do this, each agent is required to send a heartbeat signal every configurale number of secods to imply that it is well and running. The `agents-monitor` conside the agents that have not updated themselves in 5 minutes as abandoned.
- If you run the `agents-manager` locally, the provided orphan service simulates receiving orphan requests from a phantom `agents-monitor`. In a production setting, the `agents-manager` and tge `agents-monitor` are connected via a queue or a topic.
- The main focus of the `agents-manager` and `agents-monitor` is to provide an automatic failover and self-healing in case of agents failures. A production system must also provide a way to auto-scale `agents-manager` pods when the queued orphaned requests are not being processed (a condition where all `agents-managers` are fully occupied with max agents).       
- Agents can be stopped if the corresponding camera configuration (in the database) changes to excluded. The `agents-manager` detects this condition and stops the associated agent. This frees a slot in the agents pod. Therefore the `agents-manager` re-subscribes to the orphan service.  

## Sample main.go

This library provides a sample `main.go` file that can be used to bootstrap the video surveillance system. 

## Go Module

```bash
go mod init github.com/khaledhikmat/video-surveillance
go get -u gocv.io/x/gocv
go get -u github.com/joho/godotenv
```

## Pre-requisites

`OpenCV` must be installed on dev machine and in Docker on deployment. Please follow instructions [here](https://github.com/hybridgroup/gocv?tab=readme-ov-file#how-to-install).

## Run Locally

```bash
go run main.go
```

