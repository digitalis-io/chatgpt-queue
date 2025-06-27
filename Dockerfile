# Start from the official Golang image for building
FROM golang:1.23-alpine AS builder

WORKDIR /app

# Copy go mod and sum files
COPY go.mod go.sum ./
RUN go mod download

# Copy the source code
COPY main.go ./

# Build the binary
RUN go build -o chatgpt-queue main.go

# Use a minimal image for running
FROM alpine:latest

WORKDIR /app

# Copy the binary from the builder
COPY --from=builder /app/chatgpt-queue .

# Set environment variables as needed (can be overridden at runtime)
ENV RABBITMQ_URL=amqp://guest:guest@localhost:5672/
ENV RABBITMQ_QUEUE=chat_incoming
ENV OPENAI_API_KEY=dummy-token
ENV OPENAI_URL=http://localhost:11434/v1
ENV OPENAI_MODEL=deepseek-r1:8b
ENV LOG_LEVEL=info

# Expose no ports (RabbitMQ and LLM are external)
ENTRYPOINT ["./chatgpt-queue"]
