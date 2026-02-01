#!/bin/bash

# Build script for ASR Diarization Docker image

IMAGE_NAME="asr_diarization"
TAG="latest"

echo "Building Docker image: ${IMAGE_NAME}:${TAG}"

cd "$(dirname "$0")"

docker build -t ${IMAGE_NAME}:${TAG} .

if [ $? -eq 0 ]; then
    echo "✓ Build successful: ${IMAGE_NAME}:${TAG}"
else
    echo "✗ Build failed"
    exit 1
fi
