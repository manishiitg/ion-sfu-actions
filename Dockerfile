FROM golang:1.16-alpine as base 

ENV GO111MODULE=on
ENV GIN_MODE=release

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

FROM base 
RUN apk --no-cache add ca-certificates
COPY --from=build /actions /usr/local/bin/actions

# COPY config.toml /configs/sfu.toml

ENTRYPOINT ["/usr/local/bin/actions"]
CMD [""]
