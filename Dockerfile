# syntax=docker/dockerfile:1
FROM golang:1.23 as builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=1 GOOS=linux GOARCH=amd64 go build -o myMemo ./...

FROM debian:bookworm-slim
RUN apt-get update && apt-get install -y --no-install-recommends ca-certificates && rm -rf /var/lib/apt/lists/*
WORKDIR /app
COPY --from=builder /app/myMemo /app/myMemo
COPY --from=builder /app/reminders.db /app/reminders.db
EXPOSE 8080
ENTRYPOINT ["/app/myMemo"]
