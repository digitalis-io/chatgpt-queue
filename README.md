<div align="center">
  <img src="logo.png" alt="ChatGPT Queue Worker Logo" />
</div>

# ChatGPT Queue Worker

This application acts as a bridge between RabbitMQ and any Large Language Model (LLM) server that supports the ChatGPT-compatible API (such as OpenAI, Ollama, DeepSeek, etc). It allows you to submit chat queries to a RabbitMQ queue and receive the responses via another queue, enabling asynchronous, decoupled, and scalable LLM-powered workflows.

## How It Works

1. **Submit a Query:**
   - A client sends a chat message to a RabbitMQ queue (default: `default-queue`).
   - The message must be a JSON object in the following format:

   Example:
   ```json
   {
     "model": "gpt-4o",
     "messages": [
       { "role": "user", "content": "What is the capital of France?" }
     ],
     "stream": true,
     "username": "alice",
     "uid": "123e4567-e89b-12d3-a456-426614174000"
   }
   ```

2. **Processing the Query:**
   - The worker consumes messages from the input queue.
   - For each message, it sends the prompt to the configured LLM server using the ChatGPT-compatible API.
   - The worker streams or collects the response.

3. **Returning the Response:**
   - The worker publishes the LLM's response to a dedicated RabbitMQ queue named `response_{key}` (e.g., `response_123e4567-e89b-12d3-a456-426614174000`).
   - Clients can consume from this queue to receive the answer.

## Features

- **Supports Any ChatGPT-Compatible API:**
  Configure the base URL and model via environment variables to use OpenAI, Ollama, DeepSeek, or any compatible server.

- **Asynchronous and Decoupled:**
  Clients and workers communicate only via RabbitMQ, allowing for scalable and distributed deployments.

- **Streaming Support:**
  The worker streams responses from the LLM and forwards them to the response queue in real time.

- **Configurable:**
  All connection details (RabbitMQ, LLM server, model, etc.) are set via environment variables.

## Environment Variables

| Variable           | Description                                      | Default                                 |
|--------------------|--------------------------------------------------|-----------------------------------------|
| `RABBITMQ_URL`     | RabbitMQ connection URL                          | `amqp://guest:guest@localhost:5672/`    |
| `RABBITMQ_QUEUE`   | Name of the input queue for chat requests        | `default-queue`                         |
| `OPENAI_API_KEY`   | API key for the LLM server (if required)         | `dummy-token`                           |
| `OPENAI_URL`       | Base URL for the LLM server                      | `http://localhost:11434/v1`             |
| `OPENAI_MODEL`     | Model name to use                                | `deepseek-r1:8b`                        |
| `LOG_LEVEL`        | Log level (`debug`, `info`, `warn`, `error`)     | `info`                                  |

## Usage

### 1. Start the Worker

```sh
go run main.go
```

The worker will listen for chat requests on the configured RabbitMQ queue.

### 2. Submit a Chat Request

You can use any RabbitMQ client to publish a message to the input queue.
Example using Python (pika):

```python
import pika, json, uuid

connection = pika.BlockingConnection(pika.ConnectionParameters('localhost'))
channel = connection.channel()

uid = str(uuid.uuid4())
message = {
    "model": "gpt-4o",
    "messages": [
        {"role": "user", "content": "Tell me a joke."}
    ],
    "stream": True,
    "username": "alice",
    "uid": uid
}
channel.basic_publish(
    exchange='',
    routing_key='default-queue',
    body=json.dumps(message)
)
print("Sent:", message)
```

### 3. Receive the Response

Consume from the queue named `response_{uid}` (replace `{uid}` with the UID you used):

```python
def callback(ch, method, properties, body):
    print("Response:", body.decode())

channel.basic_consume(
    queue=f'response_{uid}',
    on_message_callback=callback,
    auto_ack=True
)
print('Waiting for response...')
channel.start_consuming()
```

### 4. Customizing the LLM Server

Set the `OPENAI_URL` and `OPENAI_MODEL` environment variables to point to your preferred LLM server and model.

Example for Ollama:
```sh
export OPENAI_URL="http://localhost:11434/v1"
export OPENAI_MODEL="llama2"
```

## REST API Usage

The application exposes a REST API compatible with the OpenAI Chat Completions endpoint. When you submit a chat query, the API will stream the response back to you as it becomes available.

### 1. Submit a Chat Query and Stream the Response

Send a POST request to `/v1/chat/completions` with a JSON body. You may include `username` (optional) and `uid` (optional, will be generated if omitted):

```bash
curl --no-buffer -X POST 'http://localhost:8080/v1/chat/completions' \
  -H 'Content-Type: application/json' \
  -d '{
    "model": "gpt-4o",
    "messages": [
      { "role": "user", "content": "Tell me a short story about a brave knight." }
    ],
    "stream": true,
    "username": "alice"
  }'
```

- The response will be streamed as Server-Sent Events (SSE) with `Content-Type: text/event-stream`.
- Each chunk of the LLM's response will be sent as a new SSE event.
- You do **not** need to poll or call a separate endpoint to get the answer.

**Tip:** Use `curl --no-buffer` or a compatible HTTP client to see the streamed output in real time.

### 2. Health Check

A simple health check endpoint is available:

```bash
curl http://localhost:8080/ping
```

## Extending

- You can run multiple workers for scalability.
- You can use any language or tool to produce/consume messages, as long as it speaks RabbitMQ.

## License

Apache 2.0

---

**Questions?**
Open an issue or PR!
