package main

import (
	"distributed-kv/common"
	"fmt"
	"log"
	"net"
	"net/rpc"
	"os"
	"strings"
	"sync"
)

// KVWorker holds the data and a mutex for thread safety.
type KVWorker struct {
	mu   sync.RWMutex
	data map[string]string
	port string
}

// Put RPC handler: Coordinates storage and replication.
func (w *KVWorker) Put(args *common.PutArgs, reply *common.PutReply) error {
	// 1. Storage Concern: Write to local memory
	w.writeLocal(args.Key, args.Value)

	// 2. Replication Concern: Forward if part of a chain
	if args.ForwardTo != "" {
		return w.forwardToNext(args)
	}
	return nil
}

// writeLocal handles the thread-safe writing to the map.
func (w *KVWorker) writeLocal(key, value string) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.data[key] = value
	log.Printf("[Worker-%s] Put(%s, %s)", w.port, key, value)
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
	w.mu.RLock()
	defer w.mu.RUnlock()
	val, ok := w.data[args.Key]
	reply.Value = val
	reply.Found = ok
	log.Printf("[Worker-%s] Get(%s) -> %s (Found: %v)", w.port, args.Key, val, ok)
	return nil
}

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage: worker <port>")
		return
	}
	port := os.Args[1]

	// Create the worker instance
	worker := &KVWorker{
		data: make(map[string]string),
		port: port,
	}

	// Register the worker as an RPC service
	rpc.RegisterName("KV", worker)
	rpc.HandleHTTP()

	// Listen on TCP
	l, e := net.Listen("tcp", ":"+port)
	if e != nil {
		log.Fatal("listen error:", e)
	}
	log.Printf("Worker started on port %s", port)

	// Accept connections (using default RPC server)
	for {
		conn, err := l.Accept()
		if err != nil {
			log.Fatal(err)
		}
		go rpc.ServeConn(conn)
	}
}