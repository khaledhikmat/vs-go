#!/bin/bash
set -e

# Update and install dependencies
sudo apt update
sudo apt install -y   build-essential cmake git pkg-config libgtk-3-dev   libavcodec-dev libavformat-dev libswscale-dev libv4l-dev   libx264-dev libxvidcore-dev libjpeg-dev libpng-dev libtiff-dev   gfortran openexr libatlas-base-dev python3-dev python3-numpy   libtbb2 libtbb-dev libdc1394-22-dev libgstreamer1.0-dev   libgstreamer-plugins-base1.0-dev ffmpeg

# Create build directory
mkdir -p ~/opencv_build && cd ~/opencv_build

# Clone OpenCV and OpenCV contrib
git clone https://github.com/opencv/opencv.git
git clone https://github.com/opencv/opencv_contrib.git

# Checkout a stable version (e.g., 4.8.0)
cd opencv
git checkout 4.8.0
cd ../opencv_contrib
git checkout 4.8.0
cd ../opencv

# Create build directory
mkdir -p build && cd build

# Configure build with FFMPEG, GStreamer, and contrib modules
cmake -D CMAKE_BUILD_TYPE=RELEASE \
      -D CMAKE_INSTALL_PREFIX=/usr/local \
      -D OPENCV_EXTRA_MODULES_PATH=../../opencv_contrib/modules \
      -D WITH_FFMPEG=ON \
      -D WITH_GSTREAMER=ON \
      -D WITH_TBB=ON \
      -D WITH_V4L=ON \
      -D BUILD_opencv_python3=ON \
      -D OPENCV_GENERATE_PKGCONFIG=ON \
      ..

# Compile OpenCV using all CPU cores
make -j$(nproc)

# Install
sudo make install
sudo ldconfig

echo "✅ OpenCV installed with FFMPEG and GStreamer support"
