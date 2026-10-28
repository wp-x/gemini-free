package gemini

import (
	"fmt"
	"net/http"
)

const sessionPageStage = "Gemini 会话参数读取"

type SessionHTTPError struct {
	Stage      string
	StatusCode int
}

func (e *SessionHTTPError) Error() string {
	message := fmt.Sprintf("Google %s失败：HTTP %d", e.Stage, e.StatusCode)
	if e.StatusCode == http.StatusUnauthorized || e.StatusCode == http.StatusForbidden {
		return message + "，登录凭据未通过验证，请在浏览器确认登录后重新导入 Cookie"
	}
	if e.StatusCode == http.StatusTooManyRequests {
		return message + "，上游请求过于频繁，请稍后重试"
	}
	return message
}

func checkSessionStatus(response *http.Response, stage string) error {
	if response.StatusCode != http.StatusOK {
		return &SessionHTTPError{Stage: stage, StatusCode: response.StatusCode}
	}
	return nil
}
