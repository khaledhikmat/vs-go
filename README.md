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
go mod init github.com/khaledhikmat/vs-go
go get -u gocv.io/x/gocv
go get -u github.com/joho/godotenv
go get github.com/natefinch/lumberjack
```

## Pre-requisites

`OpenCV` must be installed on dev machine and in Docker on deployment. Please follow instructions [here](https://github.com/hybridgroup/gocv?tab=readme-ov-file#how-to-install).

We provide a sample [installation script](install_opencv_ffmpeg.sh) that can be used for automatic installation. 

## Run Locally

*Make sure that the `./settings` folder contains `cameras.json` file.*

```bash
go run main.go
```

## Docker

We provide two sample Dockerfiles: one uses the OpenCV base image (i.e. `Dockerfile_auto`) and the other uses Ubuntu base image. You can expriment with each to see which ine fits better. 

## Merge and Tag

- Assuming we have a working branch i.e. `my-branch`
  - `git add --all`
  - `git commit -am "Major stuff..."`
  - `git push`
  - `git checkout main`
  - `git merge my-branch`
  - `git tag -a v1.0.0 -m "my great work"`
  - `git tag` to make sure is is created.
  - `git push --tags` to push tags to Github.

## Yolo5 Support

In order for a YOLO5 model to work with GoCV, one must use OpenCV's `ReadNet` but the model must be exported to `onnx` format:

```golang
net := gocv.ReadNet("yolov5s.onnx")
```

Here are the steps:

- Download YOLO5 weights file:

```bash
# download the yolo5 weight file from: 
https://github.com/ultralytics/yolov5/releases/download/v6.0/yolov5s.pt
```

- In order to export a trained model to different formats, Ultralytics provides [a utility](https://github.com/ultralytics/yolov5/blob/master/export.py) that can export to ONNX.

To produce an `ONNX` file, run this utility:

```bash
mkdir temp
cd temp
git clone https://github.com/ultralytics/yolov5
cd yolov5
# create Py env
python3 -m venv venv
# activate Py env
source venv/bin/activate
# deactivate Py env
deactivate
# install dependencies
pip3 install -r requirements.txt
# place the downloaded `yolov5s.pt` in the `yolo5` directory 
# export to ONNX
python3 export.py --weights yolov5s.pt --img 640 --batch 1 --include onnx
```

The output would be `yolov5s.onnx` which can be used used directly in Go via `gocv.ReadNet("yolov5s.onnx")`. 

## Issues

- `framer.go` causes a intermittent panic in this code block:

```go
for _, streamChan := range streamChannels {
    // WARNING: We need an extra check to make sure we don't send on c closed channel
    select {
    case streamChan <- FrameData{Mat: img.Clone(), Timestamp: time.Now()}:
      // Successfully sent to the channel
    case <-canxCtx.Done():
      // Context canceled, stop sending
      lgr.Logger.Info("rtspFramer context cancelled while sending!!")
    }
}
```

ّ I changed the streamers to pause for 2-3 seconds to allow the framer to complete its push of the frame. 

- Add support for OTEL.
- YOLO5 Detection.
- Support WebRTC streamer or broadcaster. It turned out this only works if we use the RTSP Go library to ingest RTSP frames because the frames are already H264-encoded. If we use the GoCV library, the frames arrive uncompressed and those cannot be sent to WebRTC Pion. However, to compress each frame is expensive and requires either an extenal process like `ffmpeg` or gpu-enabled machines. 
- Add support for clip detection:
  - Clips generated using GoCV can be large when compared to clips generated by RTSP Go native.
  - Add clip-generator streamer. Generate 5-second clips, store them locally and stream them.
  - Add clip-processor streamer to capture the 5-second clips, store them in S3 with 30-min TTL, delete local copy and invoke the model over API. If alert, stream to alerter.
- In short, GoCV is fit for detection while native RTSP in Go is fit for storage and WebRTC streaming.
- GoCV behavior:
  - GoCV uses OpenCV under the hood, and OpenCV allocates native memory (C/C++ level) for image frames, matrices, and intermediate buffers.
  - In Go, the garbage collector (GC) only tracks Go heap memory — it is completely unaware of the C/C++ memory that OpenCV is using.
  - Hence it is very important to pay attention to `img.Close()` to close the image to avoid memory leaks. All of these GoCV functions may leak memory: `gocv.Mat`, `gocv.VideoCapture`, `gocv.Window`.
  - When we stream to multiple detectors from the framer, we clone the image from the framer. Care must be taken to close the cloned image on the detector side. 
  - Running inside VS Code tends to aggregate the memory problems because it (i.e. VS Code) is running in its own Electron sandbox which uses a lot of memory.  

