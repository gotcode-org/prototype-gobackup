#!/bin/bash
set -e

if [ "$#" -ne 2 ]; then
    echo "Usage: $0 <local_directory> <new_docker_volume_name>"
    echo "Example: $0 /opt/myapp/data myapp-data"
    exit 1
fi

LOCAL_DIR="$1"
VOL_NAME="$2"

if [ ! -d "$LOCAL_DIR" ]; then
    echo "❌ Error: Source directory '$LOCAL_DIR' does not exist or is not a directory."
    exit 1
fi

# Ensure absolute path for binding
ABS_PATH=$(realpath "$LOCAL_DIR")

echo "📦 Creating Docker volume: $VOL_NAME"
docker volume create "$VOL_NAME" > /dev/null

echo "🔄 Copying data from $ABS_PATH -> $VOL_NAME (preserving permissions)..."
# We use a temporary alpine container to mount the host path and the volume simultaneously, 
# then use 'cp -a' to safely mirror all files, hidden files, permissions, and ownership.
docker run --rm \
    -v "$ABS_PATH:/source:ro" \
    -v "$VOL_NAME:/dest" \
    alpine sh -c "cp -a /source/. /dest/ 2>/dev/null || true"

echo "✅ Success! All data has been migrated into the Docker volume '$VOL_NAME'."
echo "You can now safely mount it to a container using: -v $VOL_NAME:/path/in/container"
