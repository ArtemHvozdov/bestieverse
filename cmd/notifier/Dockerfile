FROM golang:1.25-alpine AS builder

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o /app/notifier ./cmd/notifier

FROM alpine:latest

WORKDIR /app
COPY --from=builder /app/notifier .
COPY content/ ./content/
COPY assets/ ./assets/

CMD ["./notifier"]
