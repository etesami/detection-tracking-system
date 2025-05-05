# Video Detection and Tracking Microservice Pipeline

```bash
FILENAME="video_toronto.mp4"
# Download a sample file as a feed for rtsp server
yt-dlp "ytsearch1:walk in toronto" -f "bestvideo[height>=640][height<=720]" --max-filesize 200M --restrict-filenames -c -o $FILENAME

# Change the file name to source_toronto.mp4
mv $FILENAME.part $FILENAME

# Check if video is H.264
ffprobe -v error -select_streams v:0 -show_entries stream=codec_name -of csv=p=0 $FILENAME

# If H.264 then
ffmpeg -i $FILENAME -c copy -bsf:v h264_mp4toannexb output.ts

# If not, we need to re-encode
# Check if hardware acceleration is available (mac, intel)
ffmpeg -encoders | grep videotoolbox
# If available then:
# use -t 00:05:00 to limit the length of the video
# For 720p quality try 2000K to 4000K
ffmpeg -i $FILENAME -c:v h264_videotoolbox -f mpegts -b:v 3000k -t 10:00 output.ts


# If no hardware acceleration then
ffmpeg -i $FILENAME -t 00:05:00 -c:v libx264 -preset veryfast -crf 23 -f mpegts svideo.ts
```