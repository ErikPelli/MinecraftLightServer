package main

import (
	"github.com/ErikPelli/MinecraftLightServer"
)

func main() {
	server := MinecraftLightServer.NewServer()
	if err := server.Start(); err != nil {
		panic(err)
	}
}