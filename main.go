package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"

	"sync"

	"github.com/rabbitmq/amqp091-go"
	openai "github.com/sashabaranov/go-openai"
	"github.com/sirupsen/logrus"
)

var (
	mu  sync.Mutex
	log *logrus.Logger
)

func getEnvOrDefault(key, defaultValue string) string {
	val := os.Getenv(key)
	if val == "" {
		return defaultValue
	}
	return val
}

func consumeFromRabbitMQ() {
	rabbitURL := getEnvOrDefault("RABBITMQ_URL", "amqp://guest:guest@localhost:5672/")
	queueName := getEnvOrDefault("RABBITMQ_QUEUE", "default-queue")

	conn, err := amqp091.Dial(rabbitURL)
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
		mu.Lock()

		// Unmarshal JSON body as ChatCompletionRequest
		var payload ChatCompletionRequest
		if err := json.Unmarshal(msg.Body, &payload); err != nil {
			log.Printf("Failed to unmarshal message body: %v", err)
			mu.Unlock()
			continue
		}

		log.Printf("Received message: user=%s, uid=%s, model=%s", payload.Username, payload.UID, payload.Model)
		processMessage(payload)
		mu.Unlock()
	}
}

func publishResponseToRabbitMQ(key string, content string) {
	rabbitURL := getEnvOrDefault("RABBITMQ_URL", "amqp://guest:guest@localhost:5672/")
	responseQueue := fmt.Sprintf("response_%s", key)

	conn, err := amqp091.Dial(rabbitURL)
	if err != nil {
		log.Printf("Failed to connect to RabbitMQ for response: %v", err)
		return
	}
	defer conn.Close()

	ch, err := conn.Channel()
	if err != nil {
		log.Printf("Failed to open a channel for response: %v", err)
		return
	}
	defer ch.Close()

	_, err = ch.QueueDeclare(
		responseQueue, // name
		true,          // durable
		false,         // delete when unused
		false,         // exclusive
		false,         // no-wait
		nil,           // arguments
	)
	if err != nil {
		log.Printf("Failed to declare response queue: %v", err)
		return
	}

	err = ch.Publish(
		"",            // exchange
		responseQueue, // routing key (queue)
		false,         // mandatory
		false,         // immediate
		amqp091.Publishing{
			ContentType: "text/plain",
			Body:        []byte(content),
		},
	)
	if err != nil {
		log.Printf("Failed to publish response: %v", err)
	}
}

func processMessage(req ChatCompletionRequest) {
	token := getEnvOrDefault("OPENAI_API_KEY", "dummy-token")
	config := openai.DefaultConfig(token)

	config.BaseURL = getEnvOrDefault("OPENAI_URL", "http://localhost:11434/v1")
	c := openai.NewClientWithConfig(config)

	ctx := context.Background()

	chatReq := openai.ChatCompletionRequest{
		Model:     req.Model,
		MaxTokens: req.MaxTokens,
		Messages:  req.Messages,
		Stream:    req.Stream,
	}
	stream, err := c.CreateChatCompletionStream(ctx, chatReq)
	if err != nil {
		log.Errorf("ChatCompletionStream error: %v\n", err)
		return
	}
	defer stream.Close()

	for {
		response, err := stream.Recv()
		if errors.Is(err, io.EOF) {
			log.Info("\nStream finished")
			return
		}

		if err != nil {
			log.Errorf("\nStream error: %v\n", err)
			return
		}

		publishResponseToRabbitMQ(req.UID, response.Choices[0].Delta.Content)
	}
}

func init() {
	log = logrus.New()
	logLevel := getEnvOrDefault("LOG_LEVEL", "info")
	level, err := logrus.ParseLevel(logLevel)
	if err != nil {
		log.WithError(err).Errorf("Invalid log level '%s' provided. Defaulting to 'info'.", logLevel)
		log.SetLevel(logrus.InfoLevel)
	} else {
		log.SetLevel(level)
	}
}

func main() {
	// Start the REST API in a goroutine if you want both REST and queue worker
	go StartRestAPI()
	consumeFromRabbitMQ()
}
