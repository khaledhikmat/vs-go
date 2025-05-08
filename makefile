BUILD_DIR="./build"
DIST_DIR="./dist"
PROJECT_DIR := $(shell pwd)

clean_build:
	if [ -d $(BUILD_DIR) ]; then \
		rm -rf $(BUILD_DIR); \
	fi

clean_dist:
	if [ -d $(DIST_DIR) ]; then \
		rm -rf $(DIST_DIR); \
	fi

test: 
	echo "invoking test cases"

build: clean_dist clean_build test
	CGO_ENABLED=1 GOOS=linux GOARCH=amd64 go build -o "$(BUILD_DIR)/vs-go" .

clean:
	sudo docker images | grep 'vs-go' | awk '{print $3}' | xargs -r docker rmi -f
	sudo docker system prune -a

dockerize_cpu_base:
	docker buildx build --platform linux/amd64 -t khaledhikmat/cpu-opencv-gocv:latest -f ./Dockerfile_cpu_opencv_gocv .

dockerize_cpu:
	docker buildx build --platform linux/amd64 -t khaledhikmat/vs-go-cpu:latest -f ./Dockerfile_cpu_opencv_gocv_app .

dockerize_gpu_base:
	docker buildx build --platform linux/amd64 -t khaledhikmat/vs-go-gpu:latest -f ./Dockerfile_gpu_opencv_gocv .

dockerize_gpu:
	docker buildx build --platform linux/amd64 -t khaledhikmat/vs-go-gpu:latest -f ./Dockerfile_gpu_opencv_gocv_app .

push-2-hub: 
	docker login
	docker push khaledhikmat/vs-go-cpu:latest
	docker push khaledhikmat/vs-go-gpu:latest

run:
	rm -f ./recordings/*.*
	rm -f ./settings/*-stats.json
	rm -f ./settings/errors.json
	go run main.go

# unfortunately, the docker run command --network=host option does not work on MacOS or Windows.
# it works on Linux only.
start_cpu:
	rm -f ./recordings/*.*
	rm -f ./settings/*-stats.json
	rm -f ./settings/errors.json
	docker run --platform linux/amd64 --rm \
		--network=host \
		-v $(PROJECT_DIR)/recordings:/app/recordings \
		-v $(PROJECT_DIR)/settings:/app/settings \
		khaledhikmat/vs-go-cpu:latest

start_cpu_it:
	rm -f ./recordings/*.*
	rm -f ./settings/*-stats.json
	rm -f ./settings/errors.json
	docker run --platform linux/amd64 -it --rm \
		--network=host \
		-v $(PROJECT_DIR)/recordings:/app/recordings \
		-v $(PROJECT_DIR)/settings:/app/settings \
		khaledhikmat/vs-go-cpu:latest /bin/bash

# unfortunately, the docker run command does not support --gpus all on MacOS
start_gpu:
	rm -f ./recordings/*.*
	rm -f ./settings/*-stats.json
	docker run --gpus all --platform linux/amd64 --rm \
		--network=host \
		-v $(PROJECT_DIR)/recordings:/app/recordings \
		-v $(PROJECT_DIR)/settings:/app/settings \
		khaledhikmat/vs-go-cpu:latest

