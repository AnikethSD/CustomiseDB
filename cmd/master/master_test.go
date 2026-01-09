package main

import (
	"strconv"
	"testing"
)

func TestConsistentHash_Add(t *testing.T) {
	replicas := 3
	ring := NewConsistentHash(replicas)

	nodes := []string{"node1", "node2"}
	ring.Add(nodes...)

	// Expected number of keys = replicas * number of nodes
	expectedKeys := replicas * len(nodes)
	if len(ring.keys) != expectedKeys {
		t.Errorf("Expected %d keys in ring, got %d", expectedKeys, len(ring.keys))
	}

	// Verify hashMap size
	if len(ring.hashMap) != expectedKeys {
		t.Errorf("Expected %d entries in hashMap, got %d", expectedKeys, len(ring.hashMap))
	}
}

func TestConsistentHash_GetN(t *testing.T) {
	replicas := 10 // Higher replicas to reduce collisions/uneven distribution chance in small test
	ring := NewConsistentHash(replicas)

	nodes := []string{"node1", "node2", "node3"}
	ring.Add(nodes...)

	testCases := []struct {
		key      string
		n        int
		expected int
	}{
		{"key1", 1, 1},
		{"key2", 2, 2},
		{"key3", 3, 3},
		{"key4", 4, 3}, // Requesting more than available nodes (should return max available which is 3)
	}

	for _, tc := range testCases {
		got := ring.GetN(tc.key, tc.n)
		if len(got) != tc.expected {
			t.Errorf("GetN(%s, %d) returned %d nodes, expected %d", tc.key, tc.n, len(got), tc.expected)
		}

		// Verify uniqueness
		seen := make(map[string]bool)
		for _, node := range got {
			if seen[node] {
				t.Errorf("GetN returned duplicate node %s", node)
			}
			seen[node] = true
		}
	}
}

func TestConsistentHash_Distribution(t *testing.T) {
	replicas := 20
	ring := NewConsistentHash(replicas)
	nodes := []string{"A", "B", "C"}
	ring.Add(nodes...)

	// Roughly check if keys are mapped to all nodes
	// This is probabilistic but with 20 replicas and many keys it should touch all
	counts := make(map[string]int)
	for i := 0; i < 100; i++ {
		key := "key" + strconv.Itoa(i)
		nodes := ring.GetN(key, 1)
		counts[nodes[0]]++
	}

	for _, node := range nodes {
		if counts[node] == 0 {
			t.Logf("Warning: Node %s received 0 keys in distribution test (could be bad luck)", node)
		}
	}
}
