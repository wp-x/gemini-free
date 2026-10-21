package generation

import (
	"fmt"
	"strings"
)

const (
	ArgonMode      = 8
	ArgonThinking  = 80
	ThinkingLow    = 1
	ThinkingHigh   = 2
	ThinkingMedium = 4
	FlashMode      = 1
	ProMode        = 3
	AutoMode       = 4
	FlashLiteMode  = 6
	DefaultThink   = 4
	DefaultModel   = "gemini-flash-lite"
	ArgonModelID   = "95a4c0abcba24364"
)

func Models() []Model {
	models := []Model{
		{ID: "gemini-flash-lite", Mode: FlashLiteMode, Think: DefaultThink,
			Description: "轻量路由，匿名问答已实测", Verification: "匿名问答与流式通过，2026-10-21"},
		{ID: "gemini-4-argon", Mode: ArgonMode, Think: DefaultThink, Account: true,
			Description: "Argon 高思考档位", Extra: map[int]int{ArgonThinking: ThinkingHigh}},
		{ID: "gemini-4-argon-low", Mode: ArgonMode, Think: DefaultThink, Account: true,
			Description: "Argon 低思考档位", Extra: map[int]int{ArgonThinking: ThinkingLow}},
		{ID: "gemini-4-argon-medium", Mode: ArgonMode, Think: DefaultThink, Account: true,
			Description: "Argon 中思考档位", Extra: map[int]int{ArgonThinking: ThinkingMedium}},
		{ID: "gemini-4-argon-high", Mode: ArgonMode, Think: DefaultThink, Account: true,
			Description: "Argon 高思考档位", Extra: map[int]int{ArgonThinking: ThinkingHigh}},
		{ID: "gemini-3.5-flash-thinking", Mode: 2, Think: 0, Description: "思考路由"},
		{ID: "gemini-3.1-pro", Mode: ProMode, Think: DefaultThink, Account: true, Description: "Pro 路由，严格校验"},
		{ID: "gemini-3.1-pro-enhanced", Mode: ProMode, Think: DefaultThink, Account: true,
			Description: "Pro 增强请求", Extra: map[int]int{31: 2, ArgonThinking: 3}},
		{ID: "gemini-auto", Mode: AutoMode, Think: DefaultThink, Description: "上游自动选择"},
		{ID: "gemini-3.5-flash-thinking-lite", Mode: 5, Think: 0, Description: "动态思考路由"},
	}
	for _, version := range []string{"3.8", "3.7", "3.6", "3.5"} {
		models = append(models, Model{ID: "gemini-" + version + "-flash", Mode: FlashMode,
			Think: DefaultThink, Description: "历史 Flash 别名，实际路由可能为 6",
			Verification: "编号 1 请求曾返回编号 6，不代表版本已确认"})
	}
	return models
}

func Resolve(request Request) (Model, error) {
	for _, model := range Models() {
		if request.Model == model.ID {
			return withEffort(model, request.ReasoningEffort)
		}
	}
	return Model{}, fmt.Errorf("unknown model %q", request.Model)
}

func withEffort(model Model, effort string) (Model, error) {
	if effort == "" {
		return model, nil
	}
	if model.Mode != ArgonMode {
		return Model{}, fmt.Errorf("reasoning_effort is supported only for Argon")
	}
	levels := map[string]int{"low": ThinkingLow, "medium": ThinkingMedium, "high": ThinkingHigh}
	level, ok := levels[strings.ToLower(effort)]
	if !ok {
		return Model{}, fmt.Errorf("invalid reasoning_effort %q", effort)
	}
	model.Extra = map[int]int{ArgonThinking: level}
	return model, nil
}
