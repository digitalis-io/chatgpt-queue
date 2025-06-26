package main

import openai "github.com/sashabaranov/go-openai"

type ChatCompletionRequest struct {
	Model     string                         `json:"model"`
	Messages  []openai.ChatCompletionMessage `json:"messages"`
	Stream    bool                           `json:"stream"`
	MaxTokens int                            `json:"max_tokens,omitempty"`
	Username  string                         `json:"username"`
	UID       string                         `json:"uid"`
}
