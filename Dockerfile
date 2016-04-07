FROM golang:latest
RUN mkdir -p /go/src/github.com/elodina/stack-deploy
WORKDIR /go/src/github.com/elodina/stack-deploy
COPY . /go/src/github.com/elodina/stack-deploy
RUN GOGC=off go build
RUN cp /go/src/github.com/elodina/stack-deploy/stack-deploy /bin/
CMD ls /
