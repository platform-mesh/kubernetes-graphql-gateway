FROM --platform=$BUILDPLATFORM golang:1.25-alpine AS builder
ARG TARGETARCH

WORKDIR /app
COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=${TARGETARCH} go build -ldflags '-w -s' main.go


FROM scratch
WORKDIR /app
COPY --from=builder  /app/main .
USER 1001:1001
ENTRYPOINT ["./main"]
CMD ["listener"]
