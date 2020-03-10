FROM golang:1.13

RUN mkdir -p $GOPATH/src/app
WORKDIR $GOPATH/src/app

COPY . .

RUN go get
RUN go install

RUN go build -o main client.go hub.go main.go

EXPOSE 80

ENTRYPOINT ["./main", "-port=8081"]
