package gemini

import (
	"bufio"
	"context"
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"gemini-free/internal/config"
	"gemini-free/internal/generation"
)

const streamEndpoint = "/_/BardChatUi/data/assistant.lamda.BardFrontendService/StreamGenerate"
const requestModulo = 1000000

type EngineOptions struct {
	Config   func() config.Config
	Sessions *Sessions
	Client   func(config.Config) (HTTPDoer, error)
}

type Engine struct{ options EngineOptions }

func New(options EngineOptions) *Engine { return &Engine{options: options} }

func (e *Engine) Generate(ctx context.Context, request generation.Request, emit func(generation.Event) error) (generation.Result, error) {
	model, err := generation.Resolve(request)
	if err != nil {
		return generation.Result{}, err
	}
	cfg := e.options.Config()
	if model.Account && cfg.Mode == config.Anonymous {
		return generation.Result{}, fmt.Errorf("此模型需要登录会话，当前为匿名模式")
	}
	ctx, cancel := context.WithTimeout(ctx, time.Duration(cfg.TimeoutSeconds)*time.Second)
	defer cancel()
	client, err := e.options.Client(cfg)
	if err != nil {
		return generation.Result{}, err
	}
	doc, err := e.session(ctx, cfg, client)
	if err != nil {
		return generation.Result{}, err
	}
	req, err := upstreamRequest(ctx, upstreamInput{request, model, cfg, doc})
	if err != nil {
		return generation.Result{}, err
	}
	response, err := client.Do(req)
	if err != nil {
		return generation.Result{}, fmt.Errorf("upstream request: %w", err)
	}
	defer response.Body.Close()
	if cfg.Mode == config.AccountMode {
		if err := e.options.Sessions.Receive(cfg.SelectedAccount, doc, response.Cookies()); err != nil {
			return generation.Result{}, err
		}
	}
	if response.StatusCode != http.StatusOK {
		return generation.Result{}, fmt.Errorf("upstream HTTP %d", response.StatusCode)
	}
	return readGeneration(response, model, emit)
}

func (e *Engine) session(ctx context.Context, cfg config.Config, client HTTPDoer) (Document, error) {
	if cfg.Mode == config.Anonymous {
		return Document{Build: cfg.Build}, nil
	}
	for _, account := range cfg.Accounts {
		if account.ID == cfg.SelectedAccount {
			return e.options.Sessions.Snapshot(ctx, SessionRequest{
				ID: account.ID, AuthUser: account.AuthUser, AutoRefresh: cfg.AutoRefresh, Client: client})
		}
	}
	return Document{}, fmt.Errorf("请选择有效的登录会话")
}

type upstreamInput struct {
	request  generation.Request
	model    generation.Model
	config   config.Config
	document Document
}

func upstreamRequest(ctx context.Context, input upstreamInput) (*http.Request, error) {
	body, err := payload(input.request.Prompt, input.model, input.document.XSRF)
	if err != nil {
		return nil, err
	}
	prefix := accountPrefix(input.document.AuthUser)
	query := url.Values{"bl": {input.document.Build}, "hl": {"en"}, "rt": {"c"},
		"_reqid": {strconv.FormatInt(time.Now().Unix()%requestModulo, 10)}}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, origin+prefix+streamEndpoint+"?"+query.Encode(), strings.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Origin", origin)
	req.Header.Set("Referer", origin+prefix+"/app")
	req.Header.Set("X-Same-Domain", "1")
	req.Header.Set("User-Agent", userAgent)
	if input.document.Cookie != "" {
		setAccountHeaders(req, input.document)
	}
	for name, value := range modelHeaders(input.model) {
		req.Header.Set(name, value)
	}
	return req, nil
}

func setAccountHeaders(request *http.Request, doc Document) {
	request.Header.Set("Cookie", doc.Cookie)
	if doc.AuthUser != "" {
		request.Header.Set("X-Goog-AuthUser", doc.AuthUser)
	}
	cookies, _ := cookieMap(doc.Cookie)
	if sapisid := cookies["SAPISID"]; sapisid != "" {
		timestamp := strconv.FormatInt(time.Now().Unix(), 10)
		hash := sha1.Sum([]byte(timestamp + " " + sapisid + " " + origin))
		request.Header.Set("Authorization", "SAPISIDHASH "+timestamp+"_"+hex.EncodeToString(hash[:]))
	}
}

func readGeneration(response *http.Response, model generation.Model, emit func(generation.Event) error) (generation.Result, error) {
	result := generation.Result{RequestedModel: model.ID}
	reader := bufio.NewReader(response.Body)
	for {
		line, readErr := reader.ReadString('\n')
		frames, err := parseLine(line)
		if err != nil {
			return result, err
		}
		for _, f := range frames {
			if err := validateRoute(model, f); err != nil {
				return result, err
			}
			result, err = consumeFrame(result, f, emit)
			if err != nil {
				return result, err
			}
		}
		if readErr == io.EOF {
			break
		}
		if readErr != nil {
			return result, fmt.Errorf("read upstream stream: %w", readErr)
		}
	}
	if result.Text == "" {
		return result, fmt.Errorf("upstream returned no answer")
	}
	return result, nil
}

func consumeFrame(result generation.Result, f frame, emit func(generation.Event) error) (generation.Result, error) {
	text := cleanText(f.text)
	if !strings.HasPrefix(text, result.Text) {
		return result, fmt.Errorf("upstream replaced emitted text; cannot represent as append-only stream")
	}
	delta := strings.TrimPrefix(text, result.Text)
	result.Text, result.ActualMode, result.ThinkingLevel = text, f.mode, f.thinking
	if delta != "" && emit != nil {
		return result, emit(generation.Event{Text: delta, ActualMode: f.mode, ThinkingLevel: f.thinking})
	}
	return result, nil
}

func cleanText(text string) string {
	code := regexp.MustCompile("(?s)```(?:python|javascript|text)\\?code_(?:reference|stdout)&code_event_index=\\d+\\n.*?```\\n?")
	cards := regexp.MustCompile(`http://googleusercontent\.com/card_content/\d+\n?`)
	return strings.TrimLeft(cards.ReplaceAllString(code.ReplaceAllString(text, ""), ""), "\r\n")
}
