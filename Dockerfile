# Build stage
FROM golang:1.25-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download
COPY . .

ARG SERVICE_DIR

RUN go build -o /main ./${SERVICE_DIR}

# Final stage
FROM alpine:latest

WORKDIR /root/
COPY --from=builder /main .

EXPOSE 8080

CMD ["./main"]
