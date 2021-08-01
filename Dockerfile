FROM golang:1.16-alpine as base 

ENV GO111MODULE=on
# ENV GIN_MODE=release

WORKDIR $GOPATH/src/github.com/manishiitg/actions

RUN apk add \
    gstreamer \
    gstreamer-dev \
    gst-plugins-base \
    gst-plugins-base-dev \
    gst-plugins-good \
    gst-plugins-bad \
    gst-plugins-ugly

FROM base as build

RUN apk add \
build-base \
pkgconfig

COPY . $GOPATH/src/github.com/manishiitg/actions
RUN cd $GOPATH/src/github.com/manishiitg/actions && go mod download

RUN GOOS=linux go build -o /actions .

FROM jrottenberg/ffmpeg:4.1-alpine as ffmpeg-stage

FROM base 
RUN apk --no-cache add ca-certificates

COPY --from=ffmpeg-stage /usr/local /usr/local
COPY --from=build /actions /usr/local/bin/actions
RUN mkdir -p /go/src/github.com/manishiitg/actions/tracktortp/
COPY ./tracktortp/subscribe.sdp /go/src/github.com/manishiitg/actions/tracktortp/

RUN mkdir -p /go/src/github.com/manishiitg/actions/cgfs
RUN mkdir -p /go/src/github.com/manishiitg/actions/cgfs/gcloud

COPY ./cfgs/gcloud/steady-datum-291915-9c9286662fbf.json /go/src/github.com/manishiitg/actions/cfgs/gcloud/steady-datum-291915-9c9286662fbf.json

RUN apk add \
    gstreamer \
    gstreamer-dev \
    gst-plugins-base \
    gst-plugins-base-dev \
    gst-plugins-good \
    gst-plugins-bad \
    gst-plugins-ugly

# COPY config.toml /configs/sfu.toml

ENTRYPOINT ["/usr/local/bin/actions","server"]
CMD [""]
