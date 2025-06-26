package main

import (
	"fmt"
	"log"
	"os"

	"github.com/rabbitmq/amqp091-go"
)

type ChatCompletionRequest struct {
	Model    string `json:"model"`
	Messages []struct {
		Role    string `json:"role"`
		Content string `json:"content"`
	} `json:"messages"`
	Stream    bool   `json:"stream"`
	MaxTokens int    `json:"max_tokens,omitempty"`
	Username  string `json:"username"`
	UID       string `json:"uid"`
}

func main() {
	if len(os.Args) < 2 {
		log.Fatalf("Usage: %s <uid>", os.Args[0])
	}
	uuid := os.Args[1]
	queueName := fmt.Sprintf("response_%s", uuid)

	conn, err := amqp091.Dial(getEnvOrDefault("RABBITMQ_URL", "amqp://guest:guest@localhost:5672/"))
	if err != nil {
		log.Fatalf("Failed to connect to RabbitMQ: %v", err)
	}
	defer conn.Close()

	ch, err := conn.Channel()
	if err != nil {
		log.Fatalf("Failed to open a channel: %v", err)
	}
	defer ch.Close()

	msgs, err := ch.Consume(
		queueName, // queue
		"",        // consumer
		true,      // auto-ack
		false,     // exclusive
		false,     // no-local
		false,     // no-wait
		nil,       // args
	)
	if err != nil {
		log.Fatalf("Failed to register a consumer: %v", err)
	}

	log.Printf("Waiting for messages from queue: %s", queueName)
	for msg := range msgs {
		fmt.Printf("Received message: %s\n", string(msg.Body))
	}
}

func getEnvOrDefault(key, defaultValue string) string {
	val := os.Getenv(key)
	if val == "" {
		return defaultValue
	}
	return val
}
