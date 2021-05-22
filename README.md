# MinecraftLightServer
This is a Proof of Concept of a simple Minecraft server written in Go, that has a simple multiplayer world.

## Version
This server actually supports only Minecraft 1.16.5 client.

## Purpose
This is a very simple server, which can help those who are making one to better understand how the basic things that compose it interact with each other.

## Thanks
This project was inspired by:
- [ESP32 Minecraft Server](https://github.com/nikisalli/esp32-minecraft-server), a very simple Minecraft server written in C for the ESP32 development board.
- [Go-mc](https://github.com/Tnze/go-mc), a Minecraft library written in Go.

Everything has been adapted and rewritten to make code easy to understand

## Known problems
- A user can only re-enter after the KeepAlive has removed him from the players' map, and the client must have closed the connection

## Changes for the future
- Support for chunk generation
- Support for mobs
- Game changes saving
- Support for more client packets
- Plugins