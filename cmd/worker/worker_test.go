package main

import (
	"distributed-kv/common"
	"testing"
)

func TestKVWorker_Put_Get(t *testing.T) {
	worker := &KVWorker{
		data: make(map[string]string),
		port: "8000",
	}

	// Test Put
	putArgs := &common.PutArgs{
		Key:   "key1",
		Value: "value1",
	}
	putReply := &common.PutReply{}

	if err := worker.Put(putArgs, putReply); err != nil {
		t.Fatalf("Put failed: %v", err)
	}

	// Verify local storage
	if val, ok := worker.data["key1"]; !ok || val != "value1" {
		t.Errorf("Put failed to store value. Got %v, %v", val, ok)
	}

	// Test Get
	getArgs := &common.GetArgs{
		Key: "key1",
	}
	getReply := &common.GetReply{}

	if err := worker.Get(getArgs, getReply); err != nil {
		t.Fatalf("Get failed: %v", err)
	}

	if !getReply.Found {
		t.Errorf("Get returned Found=false")
	}
	if getReply.Value != "value1" {
		t.Errorf("Get returned value %s, expected value1", getReply.Value)
	}
}

func TestKVWorker_Get_NotFound(t *testing.T) {
	worker := &KVWorker{
		data: make(map[string]string),
		port: "8000",
	}

	getArgs := &common.GetArgs{
		Key: "non-existent",
	}
	getReply := &common.GetReply{}

	if err := worker.Get(getArgs, getReply); err != nil {
		t.Fatalf("Get failed: %v", err)
	}

	if getReply.Found {
		t.Errorf("Get returned Found=true for non-existent key")
	}
}
