package main

import (
	"customise-db/common"
	"flag"
	"fmt"
	"log"
	"net"
	"net/rpc"
	"strings"
	"sync"
	"time"
)

// KVWorker holds the data and a mutex for thread safety.
type KVWorker struct {
	mu          sync.RWMutex
	data        map[string]string
	port        string
	maxKeys     int
	maxLoad     int
	reqCounter  int
	currentRate int
}

// Put RPC handler: Coordinates storage and replication.
func (w *KVWorker) Put(args *common.PutArgs, reply *common.PutReply) error {
	// 1. Storage Concern: Write to local memory (with limits)
	if err := w.writeLocal(args.Key, args.Value); err != nil {
		return err
	}

	// 2. Replication Concern: Forward if part of a chain
	if args.ForwardTo != "" {
		return w.forwardToNext(args)
	}
	return nil
}

// writeLocal handles the thread-safe writing to the map.
func (w *KVWorker) writeLocal(key, value string) error {
	w.mu.Lock()
	defer w.mu.Unlock()

	w.reqCounter++ // Count as 1 request

	// Check Limits
	if w.maxKeys > 0 && len(w.data) >= w.maxKeys {
		// Allow updating existing keys, but reject new ones if full
		if _, exists := w.data[key]; !exists {
			return fmt.Errorf("node full: max keys %d reached", w.maxKeys)
		}
	}

	w.data[key] = value
	log.Printf("[Worker-%s] Put(%s, %s)", w.port, key, value)
	return nil
}

// forwardToNext handles the logic of parsing the chain and calling the next worker.
func (w *KVWorker) forwardToNext(args *common.PutArgs) error {
	parts := strings.SplitN(args.ForwardTo, ",", 2)
	nextWorker := parts[0]
	remainingChain := ""
	if len(parts) > 1 {
		remainingChain = parts[1]
	}

	client, err := rpc.Dial("tcp", nextWorker)
	if err != nil {
		return fmt.Errorf("chain forwarding failed to %s: %v", nextWorker, err)
	}
	defer client.Close()

	forwardArgs := &common.PutArgs{
		Key:       args.Key,
		Value:     args.Value,
		ForwardTo: remainingChain,
	}
	return client.Call("KV.Put", forwardArgs, &common.PutReply{})
}

// Get RPC handler.
func (w *KVWorker) Get(args *common.GetArgs, reply *common.GetReply) error {
	w.mu.Lock() // Lock for counter update + read
	w.reqCounter++
	val, ok := w.data[args.Key]
	w.mu.Unlock()

	reply.Value = val
	reply.Found = ok
	log.Printf("[Worker-%s] Get(%s) -> %s (Found: %v)", w.port, args.Key, val, ok)
	return nil
}

// GetStats returns current metrics to the Master.
func (w *KVWorker) GetStats(args *common.StatsArgs, reply *common.StatsReply) error {
	w.mu.RLock()
	defer w.mu.RUnlock()
	reply.KeyCount = len(w.data)
	reply.RequestRate = w.currentRate
	reply.MaxKeys = w.maxKeys
	reply.MaxLoad = w.maxLoad
	return nil
}

func (w *KVWorker) monitorLoad() {
	ticker := time.NewTicker(1 * time.Second)
	for range ticker.C {
		w.mu.Lock()
		w.currentRate = w.reqCounter
		w.reqCounter = 0
		w.mu.Unlock()
	}
}

func main() {
	maxKeys := flag.Int("max-keys", 0, "Maximum number of keys per node (0 = unlimited)")
	maxLoad := flag.Int("max-load", 0, "Maximum requests per second (0 = unlimited)")
	flag.Parse()

	args := flag.Args()
	if len(args) < 1 {
		fmt.Println("Usage: worker [-max-keys=N] [-max-load=N] <port>")
		return
	}
	port := args[0]

	// Create the worker instance
	worker := &KVWorker{
		data:    make(map[string]string),
		port:    port,
		maxKeys: *maxKeys,
		maxLoad: *maxLoad,
	}

	// Start load monitor
	go worker.monitorLoad()

	// Register the worker as an RPC service
	rpc.RegisterName("KV", worker)
	rpc.HandleHTTP()

	// Listen on TCP
	l, e := net.Listen("tcp", ":"+port)
	if e != nil {
		log.Fatal("listen error:", e)
	}
	log.Printf("Worker started on port %s (MaxKeys: %d, MaxLoad: %d)", port, *maxKeys, *maxLoad)

	// Accept connections
	for {
		conn, err := l.Accept()
		if err != nil {
			log.Fatal(err)
		}
		go rpc.ServeConn(conn)
	}
}