# Use the offical Golang image to create a build artifact.
# This is based on Debian and sets the GOPATH to /go.
# https://hub.docker.com/_/golang
FROM --platform=${BUILDPLATFORM} golang:1.16.2-alpine as builder

RUN apk add --no-cache gcc libc-dev git

WORKDIR /src/gatling-service

ARG version=develop
ENV VERSION="${version}"

# Force the go compiler to use modules
ENV GO111MODULE=on
ENV BUILDFLAGS=""
ENV GOPROXY=https://proxy.golang.org
ARG TARGETOS
ARG TARGETARCH

# Copy `go.mod` for definitions and `go.sum` to invalidate the next layer
# in case of a change in the dependencies
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

ARG debugBuild

# set buildflags for debug build
RUN if [ ! -z "$debugBuild" ]; then export BUILDFLAGS='-gcflags "all=-N -l"'; fi

# Copy local code to the container image.
COPY . .

# Build the command inside the container.
# (You may fetch or manage dependencies here, either manually or with a tool like "godep".)
# -ldflags '-linkmode=external'
RUN CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH}  go build $BUILDFLAGS -v -o gatling-service

# Use a Docker multi-stage build to create a lean production image.
# https://docs.docker.com/develop/develop-images/multistage-build/#use-multi-stage-builds
FROM adoptopenjdk/openjdk11:alpine
ENV ENV=production
ENV GATLING_VERSION 3.6.0

# Install extra packages
# See https://github.com/gliderlabs/docker-alpine/issues/136#issuecomment-272703023

RUN    apk update && apk upgrade \
	&& apk add ca-certificates wget bash \
	&& update-ca-certificates \
	&& rm -rf /var/cache/apk/*

ARG version=develop
ENV VERSION="${version}"

# Copy the binary to the production image from the builder stage.
COPY --from=builder /src/gatling-service/gatling-service /gatling-service

EXPOSE 8080

# required for external tools to detect this as a go binary
ENV GOTRACEBACK=all

# create directory for gatling install
RUN mkdir -p /opt/gatling/

# install gatling
RUN mkdir -p /tmp/downloads && \
  wget -q -O /tmp/downloads/gatling-$GATLING_VERSION.zip \
  https://repo1.maven.org/maven2/io/gatling/highcharts/gatling-charts-highcharts-bundle/$GATLING_VERSION/gatling-charts-highcharts-bundle-$GATLING_VERSION-bundle.zip && \
  mkdir -p /tmp/archive && cd /tmp/archive && \
  unzip /tmp/downloads/gatling-$GATLING_VERSION.zip && \
  mv /tmp/archive/gatling-charts-highcharts-bundle-$GATLING_VERSION/* /opt/gatling/ && \
  sed -i 's~_CLASSPATH="~_CLASSPATH="/opt/gatling/lib/*:~' /opt/gatling/bin/gatling.sh && \
  rm -rf /tmp/*

ENV PATH /opt/gatling/bin:$PATH

# KEEP THE FOLLOWING LINES COMMENTED OUT!!! (they will be included within the travis-ci build)
#build-uncomment ADD MANIFEST /
#build-uncomment COPY entrypoint.sh /
#build-uncomment ENTRYPOINT ["/entrypoint.sh"]

# Run the web service on container startup.
CMD ["/gatling-service"]
