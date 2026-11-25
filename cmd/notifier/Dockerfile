FROM golang:1.25-alpine AS builder

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /app/notifier ./cmd/notifier

FROM alpine:3.24
RUN addgroup -S -g 10001 app \
 && adduser -S -D -H -u 10001 -G app app

WORKDIR /app
COPY --from=builder /app/notifier .
COPY content/ ./content/
# COPY assets/ ./assets/

RUN mkdir -p /app/data && chown app:app /app/data

USER app
CMD ["./notifier"]
