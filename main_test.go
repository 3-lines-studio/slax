package main

import (
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

func TestParseJobRejectsUnsafePath(t *testing.T) {
	data := []byte(`{"team_id":"..","event":{"type":"app_mention","channel":"C1","text":"hello","ts":"1.1"}}`)
	if _, ok := parseJob(data); ok {
		t.Fatal("unsafe event was accepted")
	}
}

func TestRunAXUsesThreadSession(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell fixture")
	}
	dir := t.TempDir()
	ax := filepath.Join(dir, "ax")
	script := "#!/bin/sh\nprintf '%s\\n' \"$*\"\n"
	if err := os.WriteFile(ax, []byte(script), 0755); err != nil {
		t.Fatal(err)
	}
	cfg := config{axPath: ax, workdir: "/work", sessions: filepath.Join(dir, "sessions")}
	reply, err := runAX(cfg, job{teamID: "T1", channel: "C1", threadTS: "1.1", prompt: "fix it"})
	if err != nil {
		t.Fatal(err)
	}
	want := "--session " + filepath.Join(dir, "sessions", "T1", "C1", "1.1.jsonl") + " -C /work fix it"
	if reply != want {
		t.Fatalf("got %q, want %q", reply, want)
	}
}
