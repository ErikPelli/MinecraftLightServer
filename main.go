package main

import "github.com/ErikPelli/MinecraftLightServer/minecraft"

func main() {
	server := minecraft.NewServer()
	if err := server.Start(); err != nil {
		panic(err)
	}
}