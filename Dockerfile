FROM golang:1.19 as builder
ADD . /go/src/github.com/hur/gitea-pr-resource
WORKDIR /go/src/github.com/hur/gitea-pr-resource
RUN curl -sL https://taskfile.dev/install.sh | sh
RUN ./bin/task build

FROM alpine:3.11 as resource
COPY --from=builder /go/src/github.com/hur/gitea-pr-resource/build /opt/resource
RUN apk add --update --no-cache \
    git \
    openssh \
    && chmod +x /opt/resource/*
COPY scripts/askpass.sh /usr/local/bin/askpass.sh

FROM resource
LABEL MAINTAINER=hur