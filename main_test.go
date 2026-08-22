package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestParseJobUsesThread(t *testing.T) {
	data := []byte(`{"team_id":"T1","event":{"type":"app_mention","channel":"C1","text":"<@U1> fix it","ts":"1.1","thread_ts":"1.0"},"authorizations":[{"user_id":"U1"}]}`)
	job, ok := parseJob(data)
	if !ok {
		t.Fatal("event was rejected")
	}
	if job.teamID != "T1" || job.channel != "C1" || job.threadTS != "1.0" || job.prompt != "fix it" {
		t.Fatalf("unexpected job: %#v", job)
	}
}

func TestParseJobStartsThread(t *testing.T) {
	data := []byte(`{"team_id":"T1","event":{"type":"app_mention","channel":"C1","text":"hello","ts":"1.1"}}`)
	job, ok := parseJob(data)
	if !ok || job.threadTS != "1.1" {
		t.Fatalf("unexpected job: %#v", job)
	}
}

func TestParseJobAcceptsDirectMessage(t *testing.T) {
	data := []byte(`{"team_id":"T1","event":{"type":"message","channel":"D1","channel_type":"im","text":"hello","ts":"1.1"}}`)
	job, ok := parseJob(data)
	if !ok || job.channel != "D1" || job.threadTS != "1.1" || job.prompt != "hello" {
		t.Fatalf("unexpected job: %#v", job)
	}
}

func TestParseJobRejectsBotMessage(t *testing.T) {
	data := []byte(`{"team_id":"T1","event":{"type":"message","channel":"D1","channel_type":"im","text":"hello","ts":"1.1","bot_id":"B1"}}`)
	if _, ok := parseJob(data); ok {
		t.Fatal("bot message was accepted")
	}
}

func TestParseJobRejectsUnsafePath(t *testing.T) {
	data := []byte(`{"team_id":"..","event":{"type":"app_mention","channel":"C1","text":"hello","ts":"1.1"}}`)
	if _, ok := parseJob(data); ok {
		t.Fatal("unsafe event was accepted")
	}
}

func TestFormatMarkdown(t *testing.T) {
	input := "# Results\n\n| Name | Total |\n| --- | ---: |\n| Alpha | 12 |\n| Longer | 3 |\n\n**Done**: [details](https://example.com)"
	want := "*Results*\n\n```\n| Name   | Total |\n|--------|-------|\n| Alpha  | 12    |\n| Longer | 3     |\n```\n\n*Done*: <https://example.com|details>"
	if got := formatMarkdown(input); got != want {
		t.Fatalf("got:\n%s\nwant:\n%s", got, want)
	}
}

func TestFormatMarkdownPreservesCode(t *testing.T) {
	input := "```markdown\n# title\n**bold**\n```"
	if got := formatMarkdown(input); got != input {
		t.Fatalf("got %q, want %q", got, input)
	}
}

func TestAXEventAcceptsMessageObject(t *testing.T) {
	var event axEvent
	if err := json.Unmarshal([]byte(`{"type":"message","message":{"Role":"assistant","Content":"done"}}`), &event); err != nil {
		t.Fatal(err)
	}
}

func TestRunAXUsesConfiguredAgent(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell fixture")
	}
	dir := t.TempDir()
	ax := filepath.Join(dir, "ax")
	args := filepath.Join(dir, "args")
	context := filepath.Join(dir, "context")
	script := "#!/bin/sh\nprintf '%s' \"$*\" > " + args + "\nprintf '%s/%s' \"$AX_SLACK_CHANNEL\" \"$AX_SLACK_THREAD\" > " + context + "\nprintf '%s\\n' '{\"type\":\"result\",\"messages\":[{\"Role\":\"assistant\",\"Content\":\"done\"}]}' '{\"type\":\"done\",\"outcome\":\"done\"}'\n"
	if err := os.WriteFile(ax, []byte(script), 0755); err != nil {
		t.Fatal(err)
	}
	cfg := config{axPath: ax, workdir: "/work", sessions: filepath.Join(dir, "sessions"), baseURL: "https://api.example/v1", model: "model-a", system: "be direct"}
	reply, err := runAX(cfg, job{teamID: "T1", channel: "C1", threadTS: "1.1", prompt: "fix it"})
	if err != nil || reply != "done" {
		t.Fatalf("reply %q: %v", reply, err)
	}
	got, err := os.ReadFile(args)
	if err != nil {
		t.Fatal(err)
	}
	want := "--events --session " + filepath.Join(dir, "sessions", "T1", "C1", "1.1.jsonl") + " -C /work -base https://api.example/v1 -model model-a -system be direct fix it"
	if string(got) != want {
		t.Fatalf("got %q, want %q", got, want)
	}
	got, err = os.ReadFile(context)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "C1/1.1" {
		t.Fatalf("unexpected context: %q", got)
	}
}

func TestRunAXUsesThreadSession(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell fixture")
	}
	dir := t.TempDir()
	ax := filepath.Join(dir, "ax")
	args := filepath.Join(dir, "args")
	script := "#!/bin/sh\nprintf '%s' \"$*\" > " + args + "\nprintf '%s\\n' '{\"type\":\"result\",\"messages\":[{\"Role\":\"assistant\",\"Content\":\"done\"}]}' '{\"type\":\"done\",\"outcome\":\"done\"}'\n"
	if err := os.WriteFile(ax, []byte(script), 0755); err != nil {
		t.Fatal(err)
	}
	cfg := config{axPath: ax, workdir: "/work", sessions: filepath.Join(dir, "sessions")}
	reply, err := runAX(cfg, job{teamID: "T1", channel: "C1", threadTS: "1.1", prompt: "fix it"})
	if err != nil {
		t.Fatal(err)
	}
	if reply != "done" {
		t.Fatalf("unexpected reply: %q", reply)
	}
	got, err := os.ReadFile(args)
	if err != nil {
		t.Fatal(err)
	}
	want := "--events --session " + filepath.Join(dir, "sessions", "T1", "C1", "1.1.jsonl") + " -C /work fix it"
	if string(got) != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}
