FROM --platform=$BUILDPLATFORM golang:1.26@sha256:ec4debba7b371fb2eaa6169a72fc61ad93b9be6a9ae9da2a010cb81a760d36e7 AS builder

ARG TARGETOS
ARG TARGETARCH

WORKDIR /app
COPY . .
RUN CGO_ENABLED=0 GOOS=${TARGETOS:-linux} GOARCH=${TARGETARCH} go build -ldflags '-w -s' main.go


FROM gcr.io/distroless/static:nonroot@sha256:e3f945647ffb95b5839c07038d64f9811adf17308b9121d8a2b87b6a22a80a39
WORKDIR /
COPY --from=builder  /app/main /app/main
USER 65532:65532

ENTRYPOINT ["/app/main"]
