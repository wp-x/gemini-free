package generation

import "context"

type Request struct {
	Model           string
	Prompt          string
	ReasoningEffort string
}

type Event struct {
	Text          string `json:"text"`
	ActualMode    int    `json:"actual_mode"`
	ThinkingLevel int    `json:"thinking_level"`
}

type Result struct {
	Text           string `json:"text"`
	RequestedModel string `json:"requested_model"`
	ActualMode     int    `json:"actual_mode"`
	ThinkingLevel  int    `json:"thinking_level"`
}

type Generator interface {
	Generate(context.Context, Request, func(Event) error) (Result, error)
}

type Model struct {
	ID           string      `json:"id"`
	Mode         int         `json:"mode"`
	Think        int         `json:"-"`
	Extra        map[int]int `json:"-"`
	Description  string      `json:"description"`
	Account      bool        `json:"requires_account"`
	Verification string      `json:"verification"`
}
