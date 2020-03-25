# Run command below to build binary.
#	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -a -ldflags '-w -s' -o main server/main.go

FROM frolvlad/alpine-glibc:alpine-3.7_glibc-2.26

# Installing Go
ENV GO_VERSION 1.13.8
ENV GO_OS linux
ENV GO_ARCH amd64

ENV PATH $GOPATH/bin:/opt/go/bin:$PATH
ENV GOPATH /go

RUN mkdir -p $GOPATH/src $GOPATH/bin ; \
    apk add --no-cache ca-certificates ; \
    apk add --no-cache git ; \
    mkdir -p /opt ; \
    wget -q -O - https://dl.google.com/go/go$GO_VERSION.$GO_OS-$GO_ARCH.tar.gz \
        | tar -C /opt/ -zxf -

RUN mkdir -p $GOPATH/src/app

WORKDIR $GOPATH/src/app

COPY . .

# For Timezone data

# Build
RUN sh ./build.sh

EXPOSE 9001

ENTRYPOINT ["./main", "-addr=:9001"]
