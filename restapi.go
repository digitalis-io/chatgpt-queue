package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/rabbitmq/amqp091-go"
)

func chatCompletionsHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req ChatCompletionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if req.Username == "" {
		req.Username = "anonymous"
	}
	if req.UID == "" {
		req.UID = uuid.NewString()
	}

	queueName := getEnvOrDefault("RABBITMQ_QUEUE", "default-queue")
	body, err := json.Marshal(req)
	if err != nil {
		http.Error(w, "Failed to marshal request", http.StatusInternalServerError)
		return
	}

	rabbitURL := getEnvOrDefault("RABBITMQ_URL", "amqp://guest:guest@localhost:5672/")
	conn, err := amqp091.Dial(rabbitURL)
	if err != nil {
		http.Error(w, "Failed to connect to RabbitMQ", http.StatusInternalServerError)
		return
	}
	defer conn.Close()

	ch, err := conn.Channel()
	if err != nil {
		http.Error(w, "Failed to open a channel", http.StatusInternalServerError)
		return
	}
	defer ch.Close()

	_, err = ch.QueueDeclare(
		queueName, // name
		true,      // durable
		false,     // delete when unused
		false,     // exclusive
		false,     // no-wait
		nil,       // arguments
	)
	if err != nil {
		http.Error(w, "Failed to declare queue", http.StatusInternalServerError)
		return
	}

	log.Debugf("Publishing message to queue %s", queueName)
	err = ch.Publish(
		"",        // exchange
		queueName, // routing key (queue)
		false,     // mandatory
		false,     // immediate
		amqp091.Publishing{
			ContentType: "application/json",
			Body:        body,
		},
	)
	if err != nil {
		http.Error(w, "Failed to publish message", http.StatusInternalServerError)
		return
	}

	responseQueueName := fmt.Sprintf("response_%s", req.UID)

	/* Reconnect as consumer */
	conn, err = amqp091.Dial(rabbitURL)
	if err != nil {
		http.Error(w, "Failed to connect to RabbitMQ", http.StatusInternalServerError)
		return
	}
	defer conn.Close()

	var chConsumer *amqp091.Channel
	var msgs <-chan amqp091.Delivery
	var consumeErr error
	for i := 0; i < 300; i++ {
		if chConsumer == nil || chConsumer.IsClosed() {
			chConsumer, consumeErr = conn.Channel()
			if consumeErr != nil {
				time.Sleep(1 * time.Second)
				continue
			}
		}
		msgs, consumeErr = chConsumer.Consume(
			responseQueueName, // queue
			"",                // consumer
			true,              // auto-ack
			false,             // exclusive
			false,             // no-local
			false,             // no-wait
			nil,               // args
		)
		if consumeErr == nil {
			break
		}
		if consumeErr.Error() == "channel/connection is not open" {
			chConsumer = nil // force reopen
			time.Sleep(1 * time.Second)
			continue
		}
		if amqpErr, ok := consumeErr.(*amqp091.Error); ok && amqpErr.Code == 404 {
			// Queue not found, wait and retry
			time.Sleep(1 * time.Second)
			continue
		}
		if consumeErr != nil {
			http.Error(w, consumeErr.Error(), http.StatusInternalServerError)
			return
		}
	}

	if chConsumer == nil {
		http.Error(w, "Failed to open a channel for consuming", http.StatusInternalServerError)
		return
	}
	defer chConsumer.Close()

	// Stream messages as they arrive
	if req.Stream {
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")
		flusher, ok := w.(http.Flusher)
		if !ok {
			http.Error(w, "Streaming unsupported!", http.StatusInternalServerError)
			return
		}

		for msg := range msgs {
			chunk := map[string]interface{}{
				"id":     req.UID,
				"object": "chat.completion.chunk",
				"choices": []map[string]interface{}{
					{"delta": map[string]string{"content": string(msg.Body)}, "index": 0, "finish_reason": nil},
				},
			}
			b, _ := json.Marshal(chunk)
			fmt.Fprintf(w, "data: %s\n\n", b)
			flusher.Flush()
		}
	} else {
		// Collect all messages from the response queue and build a compatible OpenAI response
		var completions []string
		for msg := range msgs {
			completions = append(completions, string(msg.Body))
			// Optionally, break on some end-of-stream marker or after a single message
			break // Remove this break if you want to collect multiple messages
		}
		// Build a minimal OpenAI-compatible response
		resp := map[string]interface{}{
			"id":     req.UID,
			"object": "chat.completion",
			"choices": []map[string]interface{}{
				{"message": map[string]string{"role": "assistant", "content": completions[0]}},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(resp)
	}
}

func StartRestAPI() {
	http.HandleFunc("/ping", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("pong"))
	})
	http.HandleFunc("/v1/chat/completions", chatCompletionsHandler)

	port := getEnvOrDefault("RESTAPI_PORT", "8080")
	log.Infof("REST API listening on :%s\n", port)

	err := http.ListenAndServe(":"+port, nil)
	if err != nil {
		log.Fatalf("Failed to start REST API: %v", err)
	}
}
