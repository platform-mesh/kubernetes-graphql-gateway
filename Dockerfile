FROM golang:1.24-alpine AS builder
WORKDIR /app
COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags '-w -s' main.go

FROM alpine

WORKDIR /app


COPY --from=builder  /app/main .

USER 1001:1001

ENTRYPOINT ["./main"]
CMD ["gateway"]
