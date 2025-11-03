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
COPY scripts/start.sh /start.sh
RUN chmod +x /start.sh

# Vector config
RUN mkdir -p /etc/vector
COPY vector/vector.toml /etc/vector/vector.toml

# Install Vector agent
RUN apk add --no-cache curl bash \
    && curl --proto '=https' --tlsv1.2 -sSfL https://sh.vector.dev | bash -s -- -y --prefix /usr/local

EXPOSE 8080

CMD ["/start.sh"]
