APP_NAME=chatgpt-queue

.PHONY: build run docker-build docker-run

build:
	go build -o $(APP_NAME) main.go restapi.go types.go

run: build
	./$(APP_NAME)

docker-build:
	docker build -t $(APP_NAME):latest .

docker-run:
	docker run --rm -e RABBITMQ_URL -e RABBITMQ_QUEUE -e OPENAI_API_KEY -e OPENAI_URL -e OPENAI_MODEL -e LOG_LEVEL $(APP_NAME):latest