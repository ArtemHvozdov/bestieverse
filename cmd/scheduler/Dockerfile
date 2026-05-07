FROM golang:1.22-alpine AS builder

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o /app/scheduler ./cmd/scheduler

FROM alpine:latest

WORKDIR /app
COPY --from=builder /app/scheduler .
COPY content/ ./content/
COPY assets/ ./assets/

CMD ["./scheduler"]
