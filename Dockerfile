FROM --platform=${BUILDPLATFORM} golang as build-env

ARG TARGETPLATFORM
ARG BUILDPLATFORM
ARG TARGETOS
ARG TARGETARCH

RUN apt-get install -yq --no-install-recommends git

# Copy source + vendor
COPY . /go/src/github.com/vincejv/fnp-bot
WORKDIR /go/src/github.com/vincejv/fnp-bot

# Compile go binaries
ENV GOPATH=/go
RUN CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} GO111MODULE=on go build -v -a -ldflags "-s -w" -o /go/bin/fnp-bot .

# Build final image from alpine
FROM --platform=${TARGETPLATFORM} alpine:latest
RUN apk --update --no-cache add curl && rm -rf /var/cache/apk/*
COPY --from=build-env /go/bin/fnp-bot /usr/bin/fnp-bot

# Create a group and user
RUN addgroup -S fnp-bot && adduser -S fnp-bot -G fnp-bot
USER fnp-bot

ENTRYPOINT ["fnp-bot"]

EXPOSE 8095