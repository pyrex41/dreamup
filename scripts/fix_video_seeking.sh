#!/bin/bash

# Re-encode existing MP4 videos with faststart flag for seeking support
# This moves the moov atom to the beginning of the file for fast startup and seeking

MEDIA_DIR="./data/media"
TEMP_DIR=$(mktemp -d)

echo "🎬 Re-encoding videos with faststart flag for seeking support..."
echo "Media directory: $MEDIA_DIR"
echo "Temporary directory: $TEMP_DIR"
echo ""

count=0
for video in "$MEDIA_DIR"/*.mp4; do
    if [ ! -f "$video" ]; then
        echo "No MP4 files found in $MEDIA_DIR"
        exit 0
    fi

    basename=$(basename "$video")
    temp_output="$TEMP_DIR/$basename"

    echo "Processing: $basename"

    # Re-encode with faststart flag
    if ffmpeg -i "$video" -c copy -movflags faststart -y "$temp_output" 2>&1 | grep -q "error"; then
        echo "  ❌ Failed to process $basename"
    else
        # Replace original with fixed version
        mv "$temp_output" "$video"
        echo "  ✅ Fixed $basename"
        ((count++))
    fi
done

rm -rf "$TEMP_DIR"

echo ""
echo "✅ Re-encoding complete!"
echo "   Fixed: $count videos"
