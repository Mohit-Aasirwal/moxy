package main

import (
	"os/exec"
	"strings"
	"testing"
	"time"
)

// A simplistic test executing our binary
func TestEndToEnd(t *testing.T) {
	// Let's spawn mesh init
	initCmd := exec.Command("./moxy", "init")
	err := initCmd.Run()
	if err != nil {
		t.Logf("Initialize might already exist, continuing: %v", err)
	}

	initCmd2 := exec.Command("./moxy", "init", "-i", "sender_id.json")
	initCmd2.Run()
	
	// Because pubsub background joining is asynchronous and blocking
	// we will run join in background, test send natively, then terminate it.
	portArg := "-p=8015"
	joinCmd := exec.Command("./moxy", "join", "test-room", portArg)
	
	// Start Join Command
	err = joinCmd.Start()
	if err != nil {
		t.Fatalf("Failed to start join cmd: %v", err)
	}

	defer func() {
		// Cleanly kill listening node
		joinCmd.Process.Kill()
	}()

	// Wait for peer to initialize listener and join topic
	time.Sleep(2 * time.Second)

	// We can't easily capture mDNS networking cross-processes in an automated test without
	// building mock readers or reading stdout concurrently. Let's ensure Send runs without panic.
	sendCmd := exec.Command("./moxy", "send", "Hello Test Room", "-r=test-room", "-p=8016", "-i=sender_id.json")
	out, err := sendCmd.CombinedOutput()
	
	if err != nil {
		t.Fatalf("Send command failed: %v, output: %s", err, string(out))
	}

	if !strings.Contains(string(out), "Message Sent") {
		t.Errorf("Send didn't confirm message. Got: %s", string(out))
	}
}
