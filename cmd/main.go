package main

import (
	"github.com/ErikPelli/MinecraftLightServer"
	"runtime"
)

func main() {
	server := MinecraftLightServer.NewServer()
	if err := server.Start(); err != nil {
		panic(err)
	}

	// Terminate goroutine so all the others continue executing
	runtime.Goexit()
}
