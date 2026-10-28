package gemini

import (
	"fmt"

	"gemini-free/internal/generation"
)

type RoutingError struct {
	RequestedMode     int
	ActualMode        int
	RequestedThinking int
	ActualThinking    int
}

func (e *RoutingError) Error() string {
	if e.RequestedMode != e.ActualMode {
		return fmt.Sprintf("模型路由不匹配：请求 mode %d，实际 mode %d；已拒绝使用其他模型的响应。请确认官网模型权限，并在登录会话中更新 Cookie 与同次导出的参数", e.RequestedMode, e.ActualMode)
	}
	return fmt.Sprintf("Argon 思考档位不匹配：请求 %s（%d），实际 %s（%d）；已拒绝降档响应。请在官网确认该档位可用，并在登录会话中更新 Cookie 与同次导出的参数", thinkingName(e.RequestedThinking), e.RequestedThinking, thinkingName(e.ActualThinking), e.ActualThinking)
}

func thinkingName(level int) string {
	switch level {
	case generation.ThinkingLow:
		return "低"
	case generation.ThinkingMedium:
		return "中"
	case generation.ThinkingHigh:
		return "高"
	default:
		return "未知"
	}
}
