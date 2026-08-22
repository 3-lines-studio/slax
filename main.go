package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/coder/websocket"
)

type config struct {
	appToken string
	botToken string
	axPath   string
	workdir  string
	sessions string
	model    string
	baseURL  string
	system   string
}

type socketOpen struct {
	OK    bool   `json:"ok"`
	URL   string `json:"url"`
	Error string `json:"error"`
}

type envelope struct {
	EnvelopeID   string          `json:"envelope_id"`
	Type         string          `json:"type"`
	AcceptsReply bool            `json:"accepts_response_payload"`
	Payload      json.RawMessage `json:"payload"`
}

type eventPayload struct {
	TeamID         string          `json:"team_id"`
	Event          slackEvent      `json:"event"`
	Authorizations []authorization `json:"authorizations"`
}

type slackEvent struct {
	Type        string `json:"type"`
	Channel     string `json:"channel"`
	ChannelType string `json:"channel_type"`
	Text        string `json:"text"`
	TS          string `json:"ts"`
	ThreadTS    string `json:"thread_ts"`
	Subtype     string `json:"subtype"`
	BotID       string `json:"bot_id"`
}

type authorization struct {
	UserID string `json:"user_id"`
}

type job struct {
	teamID   string
	channel  string
	threadTS string
	prompt   string
}

type slackResponse struct {
	OK    bool   `json:"ok"`
	Error string `json:"error"`
}

type axEvent struct {
	Type     string      `json:"type"`
	Message  string      `json:"message"`
	Outcome  string      `json:"outcome"`
	Messages []axMessage `json:"messages"`
}

type axMessage struct {
	Role      string          `json:"Role"`
	Content   string          `json:"Content"`
	ToolCalls json.RawMessage `json:"ToolCalls"`
}

func main() {
	cfg, err := loadConfig()
	if err != nil {
		log.Fatal(err)
	}
	jobs := make(chan job, 64)
	seen := make(map[string]struct{})
	go work(cfg, jobs)
	for {
		if err := serve(cfg, jobs, seen); err != nil {
			log.Printf("slack: %v", err)
			time.Sleep(time.Second)
		}
	}
}

func loadConfig() (config, error) {
	cfg := config{
		appToken: os.Getenv("SLACK_APP_TOKEN"),
		botToken: os.Getenv("SLACK_BOT_TOKEN"),
		axPath:   os.Getenv("SLAX_AX_PATH"),
		workdir:  os.Getenv("SLAX_WORKDIR"),
		model:    os.Getenv("SLAX_MODEL"),
		baseURL:  os.Getenv("SLAX_BASE_URL"),
	}
	if cfg.appToken == "" {
		return cfg, errors.New("SLACK_APP_TOKEN is required")
	}
	if cfg.botToken == "" {
		return cfg, errors.New("SLACK_BOT_TOKEN is required")
	}
	if cfg.axPath == "" {
		cfg.axPath = "ax"
	}
	path, err := exec.LookPath(cfg.axPath)
	if err != nil {
		return cfg, fmt.Errorf("find ax: %w", err)
	}
	cfg.axPath = path
	if cfg.workdir == "" {
		cfg.workdir, err = os.Getwd()
		if err != nil {
			return cfg, fmt.Errorf("working directory: %w", err)
		}
	}
	systemFile := os.Getenv("SLAX_SYSTEM_FILE")
	if systemFile != "" {
		data, err := os.ReadFile(systemFile)
		if err != nil {
			return cfg, fmt.Errorf("read system prompt: %w", err)
		}
		cfg.system = strings.TrimSpace(string(data))
	}
	cfg.sessions = os.Getenv("SLAX_SESSION_DIR")
	if cfg.sessions == "" {
		root, err := os.UserConfigDir()
		if err != nil {
			return cfg, fmt.Errorf("config directory: %w", err)
		}
		cfg.sessions = filepath.Join(root, "slax", "sessions")
	}
	return cfg, nil
}

func serve(cfg config, jobs chan<- job, seen map[string]struct{}) error {
	ctx := context.Background()
	socketURL, err := openSocket(ctx, cfg)
	if err != nil {
		return err
	}
	conn, _, err := websocket.Dial(ctx, socketURL, nil)
	if err != nil {
		return fmt.Errorf("connect socket: %w", err)
	}
	defer conn.CloseNow()
	for {
		_, data, err := conn.Read(ctx)
		if err != nil {
			return fmt.Errorf("read socket: %w", err)
		}
		var env envelope
		if err := json.Unmarshal(data, &env); err != nil {
			continue
		}
		if env.EnvelopeID == "" {
			continue
		}
		ack, _ := json.Marshal(struct {
			EnvelopeID string `json:"envelope_id"`
		}{env.EnvelopeID})
		if err := conn.Write(ctx, websocket.MessageText, ack); err != nil {
			return fmt.Errorf("ack event: %w", err)
		}
		if _, ok := seen[env.EnvelopeID]; ok {
			continue
		}
		if len(seen) >= 10000 {
			clear(seen)
		}
		seen[env.EnvelopeID] = struct{}{}
		if env.Type != "events_api" {
			continue
		}
		j, ok := parseJob(env.Payload)
		if !ok {
			continue
		}
		select {
		case jobs <- j:
		default:
			log.Printf("drop event %s: queue full", env.EnvelopeID)
		}
	}
}

func openSocket(ctx context.Context, cfg config) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://slack.com/api/apps.connections.open", nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+cfg.appToken)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("open socket: %w", err)
	}
	defer resp.Body.Close()
	var result socketOpen
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&result); err != nil {
		return "", fmt.Errorf("decode socket response: %w", err)
	}
	if !result.OK {
		return "", fmt.Errorf("open socket: %s", result.Error)
	}
	return result.URL, nil
}

func parseJob(data []byte) (job, bool) {
	var payload eventPayload
	if json.Unmarshal(data, &payload) != nil {
		return job{}, false
	}
	event := payload.Event
	isMention := event.Type == "app_mention"
	isDM := event.Type == "message" && event.ChannelType == "im" && event.Subtype == "" && event.BotID == ""
	if !isMention && !isDM {
		return job{}, false
	}
	threadTS := payload.Event.ThreadTS
	if threadTS == "" {
		threadTS = payload.Event.TS
	}
	if !safePart(payload.TeamID) || !safePart(payload.Event.Channel) || !safePart(threadTS) {
		return job{}, false
	}
	prompt := payload.Event.Text
	if len(payload.Authorizations) > 0 {
		prompt = strings.ReplaceAll(prompt, "<@"+payload.Authorizations[0].UserID+">", "")
	}
	prompt = strings.TrimSpace(prompt)
	if prompt == "" {
		return job{}, false
	}
	return job{payload.TeamID, payload.Event.Channel, threadTS, prompt}, true
}

func safePart(value string) bool {
	if value == "" {
		return false
	}
	for _, r := range value {
		if r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '-' || r == '_' || r == '.' {
			continue
		}
		return false
	}
	return value != "." && value != ".."
}

func work(cfg config, jobs <-chan job) {
	for j := range jobs {
		reply, err := runAX(cfg, j)
		if err != nil {
			log.Printf("ax: %v", err)
			reply = "AX failed: " + err.Error()
		}
		if err := postMessage(cfg, j, reply); err != nil {
			log.Printf("reply: %v", err)
		}
	}
}

func runAX(cfg config, j job) (string, error) {
	session := filepath.Join(cfg.sessions, j.teamID, j.channel, j.threadTS+".jsonl")
	args := []string{"--events", "--session", session, "-C", cfg.workdir}
	if cfg.baseURL != "" {
		args = append(args, "-base", cfg.baseURL)
	}
	if cfg.model != "" {
		args = append(args, "-model", cfg.model)
	}
	if cfg.system != "" {
		args = append(args, "-system", cfg.system)
	}
	args = append(args, j.prompt)
	cmd := exec.Command(cfg.axPath, args...)
	cmd.Env = append(os.Environ(), "AX_SLACK_CHANNEL="+j.channel, "AX_SLACK_THREAD="+j.threadTS)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return "", err
	}
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Start(); err != nil {
		return "", err
	}
	decoder := json.NewDecoder(stdout)
	var result []axMessage
	var eventError string
	var outcome string
	for {
		var event axEvent
		err := decoder.Decode(&event)
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			_ = cmd.Wait()
			return "", fmt.Errorf("decode AX event: %w", err)
		}
		switch event.Type {
		case "result":
			result = event.Messages
		case "error":
			eventError = event.Message
		case "done":
			outcome = event.Outcome
		}
	}
	waitErr := cmd.Wait()
	if eventError != "" {
		return "", errors.New(eventError)
	}
	if waitErr != nil || outcome != "done" {
		message := strings.TrimSpace(stderr.String())
		if message == "" && waitErr != nil {
			message = waitErr.Error()
		}
		if message == "" {
			message = "AX ended with " + outcome
		}
		return "", errors.New(message)
	}
	for i := len(result) - 1; i >= 0; i-- {
		message := result[i]
		if message.Role != "assistant" || strings.TrimSpace(message.Content) == "" {
			continue
		}
		if len(message.ToolCalls) > 0 && string(message.ToolCalls) != "null" && string(message.ToolCalls) != "[]" {
			continue
		}
		return strings.TrimSpace(message.Content), nil
	}
	return "", errors.New("ax returned an empty response")
}

func postMessage(cfg config, j job, text string) error {
	form := url.Values{
		"channel":   {j.channel},
		"thread_ts": {j.threadTS},
		"text":      {text},
	}
	req, err := http.NewRequest(http.MethodPost, "https://slack.com/api/chat.postMessage", strings.NewReader(form.Encode()))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+cfg.botToken)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	var result slackResponse
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&result); err != nil {
		return err
	}
	if !result.OK {
		return errors.New(result.Error)
	}
	return nil
}
