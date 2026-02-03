FROM --platform=$BUILDPLATFORM golang:1.25 AS builder

ARG TARGETOS
ARG TARGETARCH

WORKDIR /app
COPY . .
RUN CGO_ENABLED=0 GOOS=${TARGETOS:-linux} GOARCH=${TARGETARCH} go build -ldflags '-w -s' main.go


FROM gcr.io/distroless/static:nonroot
WORKDIR /
COPY --from=builder  /app/main /app/main
USER 65532:65532

ENTRYPOINT ["/app/main"]
