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

ABS_PATH=$(realpath "$LOCAL_DIR")

echo "📦 Ensuring Docker volume exists: $VOL_NAME"
docker volume create "$VOL_NAME" > /dev/null

echo "🔄 Rsyncing data from $ABS_PATH -> $VOL_NAME..."
# We spin up an alpine container, install rsync on the fly, and use it to mirror the directory.
# This allows you to run this script multiple times to sync live changes safely!
docker run --rm \
    -v "$ABS_PATH:/source:ro" \
    -v "$VOL_NAME:/dest" \
    alpine sh -c "apk add --no-cache rsync >/dev/null && rsync -av --delete /source/ /dest/"

echo "✅ Success! All data is synchronized into the Docker volume '$VOL_NAME'."
