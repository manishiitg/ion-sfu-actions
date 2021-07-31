
## Tools to work with Pion SFU ##


This is code sample of some actions to be performed with ion-sfu using apis and command line both.

- Load Testing   (go run main.go laodtest)
- RTMP to Track  (go run main.go rtmptotrack)
- Save to Webm   (go run main.go savetodisk)
- Track to RTMP Publish   (go run main.go stream)
- API Server     (go run main.go server)


This is not ready for production at all right now, rather its more experimental basis

I am using a slightly modified version ion-sdk-go https://github.com/manishiitg/ion-sdk-go 

Testing Videos
================
https://test-videos.co.uk/bigbuckbunny/webm-vp9

https://commons.wikimedia.org/wiki/File:Big_Buck_Bunny_4K.webm


Stream
=========

netstat -nau | awk -F'[[:space:]]+|:' 'NR>2 && $5>=6000 && $5<=7000'


ffmpeg -protocol_whitelist file,udp,rtp -i subscribe.sdp -c:v libx264 -preset veryfast -b:v 3000k -maxrate 3000k -bufsize 6000k -pix_fmt yuv420p -g 50 -c:a aac -b:a 160k -ac 2 -ar 44100 -f flv rtmp://bom01.contribute.live-video.net/app/live_666332364_5791UvimKkDZW8edq8DAi4011wc4cR

ffmpeg -protocol_whitelist file,rtp,udp,https,tls,tcp -i subscribe.sdp -c:v libx264 -pix_fmt yuv420p -c:a aac -ar 16k -ac 1 -preset ultrafast -tune zerolatency -f flv rtmp://bom01.contribute.live-video.net/app/live_666332364_5791UvimKkDZW8edq8DAi4011wc4cR



RTMP TO WEBRTC
==============

ffmpeg -i '$RTMP_URL' -an -vcodec libvpx -cpu-used 5 -deadline 1 -g 10 -error-resilient 1 -auto-alt-ref 1 -f rtp rtp://127.0.0.1:5004 -vn -c:a libopus -f rtp rtp:/127.0.0.1:5006


ffmpeg -re -stream_loop 400 -i /var/tmp/big_buck_bunny_360p_10mb.mp4 -c:v libx264 -preset veryfast -b:v 3000k -maxrate 3000k -bufsize 6000k -pix_fmt yuv420p -g 50 -c:a aac -b:a 160k -ac 2 -f flv rtmp://localhost:1935/live/rfBd56ti2SMtYvSgD5xAV0YU99zampta7Z7S575KLkIZ9PYk

ffmpeg -i rtmp://localhost:1935/live/movie -an -vcodec libvpx -cpu-used 5 -deadline 1 -g 10 -error-resilient 1 -auto-alt-ref 1 -f rtp rtp://127.0.0.1:3004 -vn -c:a libopus -f rtp rtp://127.0.0.1:3006


ffmpeg -re -stream_loop 100 -i demo.flv -c copy -f flv rtmp://localhost:1935/live/rfBd56ti2SMtYvSgD5xAV0YU99zampta7Z7S575KLkIZ9PYk




export GO111MODULE=on



=======
GStreamer install ubuntu 20.10

apt-get install libgstreamer1.0-dev libgstreamer-plugins-base1.0-dev libgstreamer-plugins-bad1.0-dev gstreamer1.0-plugins-base gstreamer1.0-plugins-good gstreamer1.0-plugins-bad gstreamer1.0-plugins-ugly gstreamer1.0-libav gstreamer1.0-tools gstreamer1.0-x gstreamer1.0-alsa gstreamer1.0-gl gstreamer1.0-gtk3 gstreamer1.0-qt5 gstreamer1.0-pulseaudio

