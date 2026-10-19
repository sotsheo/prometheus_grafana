FROM golang:1.22-alpine AS builder

WORKDIR /app

COPY go.mod ./
COPY main.go ./

RUN CGO_ENABLED=0 GOOS=linux go build -o alert_logger main.go

FROM alpine:3.19

WORKDIR /app

COPY --from=builder /app/alert_logger .

# Create log files if not present
RUN touch alerts.log requests.log

EXPOSE 5001

CMD ["./alert_logger"]
