package common

import "testing"

func TestPutArgs(t *testing.T) {
	args := PutArgs{
		Key:       "test-key",
		Value:     "test-value",
		ForwardTo: "worker-address",
	}

	if args.Key != "test-key" {
		t.Errorf("Expected Key 'test-key', got '%s'", args.Key)
	}
	if args.Value != "test-value" {
		t.Errorf("Expected Value 'test-value', got '%s'", args.Value)
	}
	if args.ForwardTo != "worker-address" {
		t.Errorf("Expected ForwardTo 'worker-address', got '%s'", args.ForwardTo)
	}
}

func TestGetArgs(t *testing.T) {
	args := GetArgs{
		Key: "test-key",
	}

	if args.Key != "test-key" {
		t.Errorf("Expected Key 'test-key', got '%s'", args.Key)
	}
}

func TestGetReply(t *testing.T) {
	reply := GetReply{
		Value: "test-value",
		Found: true,
	}

	if reply.Value != "test-value" {
		t.Errorf("Expected Value 'test-value', got '%s'", reply.Value)
	}
	if !reply.Found {
		t.Errorf("Expected Found true, got false")
	}
}
