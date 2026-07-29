package identityx

import (
	"strings"
	"testing"
	"time"
)

func TestFromProfileBuildsStableWorkerID(t *testing.T) {
	now := time.Unix(100, 0)
	profile := Profile{
		Role:      RoleWorker,
		Hostname:  "Crawler_Node_01.internal",
		Platform:  "linux/amd64",
		MachineID: "machine-1",
		MACAddrs:  []string{"aa:bb:cc:dd:ee:01"},
		Metadata:  map[string]string{"container_id": "container-123"},
	}

	first, err := FromProfile(profile, now)
	if err != nil {
		t.Fatalf("FromProfile() error = %v", err)
	}
	second, err := FromProfile(profile, now.Add(time.Hour))
	if err != nil {
		t.Fatalf("FromProfile() second error = %v", err)
	}

	if first.ID != second.ID {
		t.Fatalf("expected stable id, got %q and %q", first.ID, second.ID)
	}
	if !strings.HasPrefix(first.ID, "w-crawler-") {
		t.Fatalf("expected worker prefix and sanitized host, got %q", first.ID)
	}
	if len(first.ShortID) != 12 {
		t.Fatalf("expected 12-char short id, got %q", first.ShortID)
	}
}

func TestFromProfileBuildsFollowerID(t *testing.T) {
	identity, err := FromProfile(Profile{
		Role:      RoleFollower,
		Hostname:  "result-node",
		Platform:  "linux/amd64",
		MachineID: "machine-2",
	}, time.Unix(100, 0))
	if err != nil {
		t.Fatalf("FromProfile() error = %v", err)
	}

	if !strings.HasPrefix(identity.ID, "f-result-") {
		t.Fatalf("expected follower prefix, got %q", identity.ID)
	}
}

func TestFromProfileRejectsUnknownRole(t *testing.T) {
	_, err := FromProfile(Profile{Role: "master"}, time.Unix(100, 0))
	if err == nil {
		t.Fatal("expected error for unsupported role")
	}
}

func TestIsCompatibleAllowsSameMachineID(t *testing.T) {
	cached := &Identity{
		Role:      RoleWorker,
		MachineID: "machine-1",
		MACAddrs:  []string{"aa:bb:cc:dd:ee:01"},
	}
	current := &Identity{
		Role:      RoleWorker,
		MachineID: "machine-1",
		MACAddrs:  []string{"ff:ee:dd:cc:bb:aa"},
	}

	if !IsCompatible(cached, current) {
		t.Fatal("expected compatible identity when machine id matches")
	}
}

func TestIsCompatibleRejectsDifferentMACsWithoutMachineID(t *testing.T) {
	cached := &Identity{
		Role:     RoleWorker,
		MACAddrs: []string{"aa:bb:cc:dd:ee:01"},
	}
	current := &Identity{
		Role:     RoleWorker,
		MACAddrs: []string{"ff:ee:dd:cc:bb:aa"},
	}

	if IsCompatible(cached, current) {
		t.Fatal("expected incompatible identity when mac addresses differ")
	}
}
