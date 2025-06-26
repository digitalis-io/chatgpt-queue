package main

import (
	"fmt"
	"log"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/rabbitmq/amqp091-go"
)

func main() {
	conn, err := amqp091.Dial(getEnvOrDefault("RABBITMQ_URL", "amqp://guest:guest@localhost:5672/"))
	if err != nil {
		log.Fatalf("Failed to connect to RabbitMQ: %v", err)
	}
	defer conn.Close()

	if len(os.Args) > 1 {
		key := os.Args[1]
		consumeFromRabbitMQ(conn, key)
		return
	}
}

func getEnvOrDefault(key, defaultValue string) string {
	val := os.Getenv(key)
	if val == "" {
		return defaultValue
	}
	return val
}

func consumeFromRabbitMQ(conn *amqp091.Connection, key string) {
	responseQueue := fmt.Sprintf("response_%s", key)
	ch, err := conn.Channel()
	if err != nil {
		log.Fatalf("Failed to open a channel: %v", err)
	}
	defer ch.Close()

	msgs, err := ch.Consume(
		responseQueue, // queue
		"",            // consumer
		true,          // auto-ack
		false,         // exclusive
		false,         // no-local
		false,         // no-wait
		nil,           // args
	)
	if err != nil {
		log.Fatalf("Failed to register a consumer: %v", err)
	}

	var builder strings.Builder
	done := make(chan bool)

	// Handle Ctrl+C gracefully
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		for msg := range msgs {
			builder.Write(msg.Body)
		}
		done <- true
	}()

	select {
	case <-sigChan:
		// User interrupted
		ch.Close() // This will close the msgs channel
	case <-done:
		// Channel closed
	}

	fmt.Println(builder.String())
}
