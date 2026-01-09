package main

import (
	"distributed-kv/common"
	"flag"
	"fmt"
	"hash/crc32"
	"log"
	"net"
	"net/http"
	"net/rpc"
	"sort"
	"strconv"
	"strings"
	"sync"
)

// ConsistentHash handles the ring logic.
type ConsistentHash struct {
	replicas int               // Virtual nodes per physical node
	keys     []int             // Sorted hash ring
	hashMap  map[int]string    // Map hash -> physical node
	mu       sync.RWMutex
}

func NewConsistentHash(replicas int) *ConsistentHash {
	return &ConsistentHash{
		replicas: replicas,
		hashMap:  make(map[int]string),
	}
}

func (c *ConsistentHash) Add(nodes ...string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	for _, node := range nodes {
		for i := 0; i < c.replicas; i++ {
			hash := int(crc32.ChecksumIEEE([]byte(strconv.Itoa(i) + node)))
			c.keys = append(c.keys, hash)
			c.hashMap[hash] = node
		}
	}
	sort.Ints(c.keys)
}

// GetN returns the 'n' distinct physical nodes responsible for the key.
func (c *ConsistentHash) GetN(key string, n int) []string {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if len(c.keys) == 0 {
		return nil
	}

	hash := int(crc32.ChecksumIEEE([]byte(key)))
	
	// Binary search for appropriate replica
	idx := sort.Search(len(c.keys), func(i int) bool {
		return c.keys[i] >= hash
	})

	if idx == len(c.keys) {
		idx = 0
	}

	uniqueNodes := make(map[string]bool)
	var nodes []string

	// Walk the ring clockwise
	for len(nodes) < n && len(uniqueNodes) < len(c.hashMap)/c.replicas {
		node := c.hashMap[c.keys[idx]]
		if !uniqueNodes[node] {
			uniqueNodes[node] = true
			nodes = append(nodes, node)
		}
		idx = (idx + 1) % len(c.keys)
	}
	
	return nodes
}

type Master struct {
	workers []string // Keep for reference
	ring    *ConsistentHash
	mode    string
}

// getReplicas returns the addresses of the workers that should store this key.
func (m *Master) getReplicas(key string) []string {
	// Determine RF based on worker count
	rf := 2
	if len(m.workers) >= 3 {
		rf = 3
	}
	return m.ring.GetN(key, rf)
}

// Put delegates to the specific strategy.
func (m *Master) Put(args *common.PutArgs, reply *common.PutReply) error {
	switch m.mode {
	case "async":
		return m.putAsync(args)
	case "chain":
		return m.putChain(args)
	case "quorum":
		return m.putQuorum(args)
	case "sync":
		fallthrough
	default:
		return m.putSync(args)
	}
}

// putSync: Write to all replicas, wait for all.
func (m *Master) putSync(args *common.PutArgs) error {
	replicas := m.getReplicas(args.Key)
	var wg sync.WaitGroup
	errChan := make(chan error, len(replicas))

	for _, addr := range replicas {
		wg.Add(1)
		go func(workerAddr string) {
			defer wg.Done()
			if err := callWorker(workerAddr, "KV.Put", args, &common.PutReply{}); err != nil {
				errChan <- err
			}
		}(addr)
	}
	wg.Wait()
	close(errChan)

	if err := <-errChan; err != nil {
		return err // Fail if ANY replica fails (Strict Sync)
	}
	return nil
}

// putAsync: Write to Primary (wait), others in background.
func (m *Master) putAsync(args *common.PutArgs) error {
	replicas := m.getReplicas(args.Key)
	primaryAddr := replicas[0]
	
	// Write to Primary
	if err := callWorker(primaryAddr, "KV.Put", args, &common.PutReply{}); err != nil {
		return fmt.Errorf("primary write failed: %v", err)
	}

	// Replicate to others in background
	for i := 1; i < len(replicas); i++ {
		go func(workerAddr string) {
			callWorker(workerAddr, "KV.Put", args, &common.PutReply{})
		}(replicas[i])
	}
	return nil
}

// putQuorum: Write to all, succeed if Majority (N/2 + 1) ack.
func (m *Master) putQuorum(args *common.PutArgs) error {
	replicas := m.getReplicas(args.Key)
	required := (len(replicas) / 2) + 1
	
	successChan := make(chan bool, len(replicas))
	
	for _, addr := range replicas {
		go func(workerAddr string) {
			if err := callWorker(workerAddr, "KV.Put", args, &common.PutReply{}); err == nil {
				successChan <- true
			} else {
				successChan <- false
			}
		}(addr)
	}

	successCount := 0
	failCount := 0
	for i := 0; i < len(replicas); i++ {
		if <-successChan {
			successCount++
		} else {
			failCount++
		}
		if successCount >= required {
			return nil
		}
		if failCount > (len(replicas) - required) {
			return fmt.Errorf("quorum failed: %d/%d success", successCount, len(replicas))
		}
	}
	return fmt.Errorf("quorum failed")
}

// putChain: Write to Head, Head forwards to next...
func (m *Master) putChain(args *common.PutArgs) error {
	replicas := m.getReplicas(args.Key)
	head := replicas[0]
	
	// Construct the chain string: "w2,w3"
	var chain []string
	for i := 1; i < len(replicas); i++ {
		chain = append(chain, replicas[i])
	}
	
	args.ForwardTo = strings.Join(chain, ",")
	return callWorker(head, "KV.Put", args, &common.PutReply{})
}

// Get delegates to strategy.
func (m *Master) Get(args *common.GetArgs, reply *common.GetReply) error {
	if m.mode == "quorum" {
		return m.getQuorum(args, reply)
	}
	// For Chain, Sync, Async -> Read from Tail (Chain) or Failover (Sync/Async)
	if m.mode == "chain" {
		return m.getChain(args, reply)
	}
	return m.getFailover(args, reply)
}

// getFailover: Try replicas one by one.
func (m *Master) getFailover(args *common.GetArgs, reply *common.GetReply) error {
	replicas := m.getReplicas(args.Key)
	var lastErr error
	for _, addr := range replicas {
		if err := callWorker(addr, "KV.Get", args, reply); err == nil {
			return nil
		} else {
			lastErr = err
		}
	}
	return fmt.Errorf("all replicas failed: %v", lastErr)
}

// getChain: Read from the Tail (Last replica).
func (m *Master) getChain(args *common.GetArgs, reply *common.GetReply) error {
	replicas := m.getReplicas(args.Key)
	tail := replicas[len(replicas)-1]
	return callWorker(tail, "KV.Get", args, reply)
}

// getQuorum: Read from Majority, check for agreement.
func (m *Master) getQuorum(args *common.GetArgs, reply *common.GetReply) error {
	replicas := m.getReplicas(args.Key)
	required := (len(replicas) / 2) + 1
	
	type result struct {
		val   string
		found bool
		err   error
	}
	resChan := make(chan result, len(replicas))

	for _, addr := range replicas {
		go func(workerAddr string) {
			r := &common.GetReply{}
			if err := callWorker(workerAddr, "KV.Get", args, r); err != nil {
				resChan <- result{err: err}
			} else {
				resChan <- result{val: r.Value, found: r.Found}
			}
		}(addr)
	}

	counts := make(map[string]int)
	// count successes not strictly needed if we just check consensus, but good for debugging
	
	for i := 0; i < len(replicas); i++ {
		res := <-resChan
		if res.err == nil && res.found {
			counts[res.val]++
		}
	}

	for val, count := range counts {
		if count >= required {
			reply.Value = val
			reply.Found = true
			return nil
		}
	}
	return fmt.Errorf("quorum read failed: no consensus found")
}

func callWorker(addr string, method string, args interface{}, reply interface{}) error {
	client, err := rpc.Dial("tcp", addr)
	if err != nil {
		return err
	}
	defer client.Close()
	return client.Call(method, args, reply)
}

func main() {
	mode := flag.String("mode", "sync", "Replication mode: sync, async, chain, quorum")
	flag.Parse()

	args := flag.Args()
	if len(args) < 2 {
		fmt.Println("Usage: master -mode=<mode> <masterPort> <worker1> ...")
		return
	}
	masterPort := args[0]
	workerAddrs := args[1:]

	// Initialize Consistent Hash Ring
	ring := NewConsistentHash(20) // 20 virtual nodes per worker
	ring.Add(workerAddrs...)

	master := &Master{
		workers: workerAddrs, 
		ring:    ring,
		mode:    *mode,
	}
	rpc.RegisterName("KV", master)
	rpc.HandleHTTP()

	// HTTP Gateway
	http.HandleFunc("/put", func(w http.ResponseWriter, r *http.Request) {
		key, val := r.URL.Query().Get("key"), r.URL.Query().Get("value")
		if key == "" || val == "" {
			http.Error(w, "missing params", 400)
			return
		}
		if err := master.Put(&common.PutArgs{Key: key, Value: val}, &common.PutReply{}); err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		fmt.Fprintf(w, "OK (Mode: %s)\n", master.mode)
	})

	http.HandleFunc("/get", func(w http.ResponseWriter, r *http.Request) {
		key := r.URL.Query().Get("key")
		reply := &common.GetReply{}
		if err := master.Get(&common.GetArgs{Key: key}, reply); err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		if !reply.Found {
			http.Error(w, "Not Found", 404)
			return
		}
		fmt.Fprintf(w, "%s\n", reply.Value)
	})
	
	// Start HTTP Server for Client
	go http.ListenAndServe(":8080", nil)

	l, _ := net.Listen("tcp", ":"+masterPort)
	log.Printf("Master started on %s with %d workers (Mode: %s)", masterPort, len(workerAddrs), *mode)
	for {
		conn, _ := l.Accept()
		go rpc.ServeConn(conn)
	}
}