package main

import (
	"customise-db/common"
	"fmt"
	"log"
	"net/rpc"
	"os"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage: client <masterAddress>")
		return
	}
	masterAddr := os.Args[1]

	client, err := rpc.Dial("tcp", masterAddr)
	if err != nil {
		log.Fatal("dialing:", err)
	}

	// Helper to put
	put := func(key, val string) {
		args := &common.PutArgs{Key: key, Value: val}
		reply := &common.PutReply{}
		err = client.Call("KV.Put", args, reply)
		if err != nil {
			log.Fatal("put error:", err)
		}
		fmt.Printf("Client: Put(%s, %s)\n", key, val)
	}

	// Helper to get
	get := func(key string) {
		args := &common.GetArgs{Key: key}
		reply := &common.GetReply{}
		err = client.Call("KV.Get", args, reply)
		if err != nil {
			log.Fatal("get error:", err)
		}
		fmt.Printf("Client: Get(%s) -> %s (Found: %v)\n", key, reply.Value, reply.Found)
	}

	// 1. Put some data
	put("user:1", "Alice")
	put("user:2", "Bob")
	put("user:3", "Charlie")
	put("user:4", "Dave")
	put("user:5", "Eve")

	fmt.Println("---")

	// 2. Read it back
	get("user:1")
	get("user:2")
	get("user:3")
	get("user:4")
	get("user:5")
	get("user:99") // Should be not found
}
