package main

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/slack-go/slack"
)

type axisArtifact struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	MediaType string `json:"media_type"`
	Size      int64  `json:"size"`
	SessionID string `json:"-"`
}

func runAxis(cfg config, j job) (string, []axisArtifact, error) {
	session, err := axisSession(cfg, j)
	if err != nil {
		return "", nil, err
	}
	body := url.Values{"prompt": {j.prompt}}.Encode()
	request, err := axisRequest(cfg, http.MethodPost, "/sessions/"+url.PathEscape(session)+"/messages", strings.NewReader(body))
	if err != nil {
		return "", nil, err
	}
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		return "", nil, err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return "", nil, axisError(response)
	}
	var started struct {
		RunID string `json:"run_id"`
	}
	if err := json.NewDecoder(response.Body).Decode(&started); err != nil || started.RunID == "" {
		return "", nil, errors.New("Axis returned an invalid run")
	}
	return streamAxis(cfg, session, started.RunID)
}

func axisSession(cfg config, j job) (string, error) {
	path := filepath.Join(cfg.sessions, j.teamID, j.channel, j.threadTS+".axis")
	if data, err := os.ReadFile(path); err == nil {
		return strings.TrimSpace(string(data)), nil
	}
	requestBody, _ := json.Marshal(map[string]string{"bot_id": cfg.botID})
	request, err := axisRequest(cfg, http.MethodPost, "/api/projects/"+url.PathEscape(cfg.projectID)+"/sessions", strings.NewReader(string(requestBody)))
	if err != nil {
		return "", err
	}
	request.Header.Set("Content-Type", "application/json")
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		return "", err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusCreated {
		return "", axisError(response)
	}
	var session struct {
		ID string `json:"id"`
	}
	if err := json.NewDecoder(response.Body).Decode(&session); err != nil || session.ID == "" {
		return "", errors.New("Axis returned an invalid session")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return "", err
	}
	if err := os.WriteFile(path, []byte(session.ID+"\n"), 0600); err != nil {
		return "", err
	}
	return session.ID, nil
}

func streamAxis(cfg config, session, run string) (string, []axisArtifact, error) {
	request, err := axisRequest(cfg, http.MethodGet, "/runs/"+url.PathEscape(run)+"/events", nil)
	if err != nil {
		return "", nil, err
	}
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		return "", nil, err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return "", nil, axisError(response)
	}
	var reply strings.Builder
	var artifacts []axisArtifact
	eventType := "message"
	scanner := bufio.NewScanner(response.Body)
	scanner.Buffer(make([]byte, 64*1024), 2*1024*1024)
	for scanner.Scan() {
		line := scanner.Text()
		if value, ok := strings.CutPrefix(line, "event: "); ok {
			eventType = value
			continue
		}
		value, ok := strings.CutPrefix(line, "data: ")
		if !ok {
			continue
		}
		switch eventType {
		case "delta":
			var text string
			if json.Unmarshal([]byte(value), &text) == nil {
				reply.WriteString(text)
			}
		case "artifact":
			var item axisArtifact
			if json.Unmarshal([]byte(value), &item) == nil {
				item.SessionID = session
				artifacts = append(artifacts, item)
			}
		case "failure":
			var message string
			json.Unmarshal([]byte(value), &message)
			return "", nil, errors.New(message)
		}
		eventType = "message"
	}
	if err := scanner.Err(); err != nil {
		return "", nil, err
	}
	if strings.TrimSpace(reply.String()) == "" {
		return "", nil, errors.New("Axis returned an empty response")
	}
	return strings.TrimSpace(reply.String()), artifacts, nil
}

func uploadArtifact(cfg config, j job, item axisArtifact) error {
	request, err := axisRequest(cfg, http.MethodGet, "/api/sessions/"+url.PathEscape(item.SessionID)+"/artifacts/"+url.PathEscape(item.ID), nil)
	if err != nil {
		return err
	}
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return axisError(response)
	}
	file, err := os.CreateTemp("", "slaxi-artifact-*")
	if err != nil {
		return err
	}
	path := file.Name()
	defer os.Remove(path)
	if _, err := io.Copy(file, io.LimitReader(response.Body, 25*1024*1024+1)); err != nil {
		file.Close()
		return err
	}
	if err := file.Close(); err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	_, err = slack.New(cfg.botToken).UploadFileV2Context(ctx, slack.UploadFileV2Parameters{File: path, Channel: j.channel, ThreadTimestamp: j.threadTS, Filename: item.Name})
	return err
}

func axisRequest(cfg config, method, path string, body io.Reader) (*http.Request, error) {
	request, err := http.NewRequest(method, cfg.axisURL+path, body)
	if err != nil {
		return nil, err
	}
	if cfg.axisUsername != "" || cfg.axisPassword != "" {
		request.SetBasicAuth(cfg.axisUsername, cfg.axisPassword)
	}
	return request, nil
}

func axisError(response *http.Response) error {
	body, _ := io.ReadAll(io.LimitReader(response.Body, 4096))
	message := strings.TrimSpace(string(body))
	if message == "" {
		message = response.Status
	}
	return errors.New(message)
}
