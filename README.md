# MinecraftLightServer
This is a Proof of Concept of a simple Minecraft server written in Go, that has a simple multiplayer world.

## Version
This server actually supports only Minecraft 1.16.5 clients.

## Purpose
This is a very simple server, which can help those who are making one to better understand how the basic things that compose it interact with each other.

## Thanks
This project was inspired by:
- [ESP32 Minecraft Server](https://github.com/nikisalli/esp32-minecraft-server), a very simple Minecraft server written in C for the ESP32 development board.
- [Go-mc](https://github.com/Tnze/go-mc), a Minecraft library written in Go.
- [wiki vg](https://wiki.vg/Protocol), a website that has the documentation for every Minecraft package.

Everything has been adapted and rewritten to make code easy to understand.

## Screenshots
![MinecraftLightServer chunk](screenshots/screenshot1.png?raw=true "Chunk")
![MinecraftLightServer chat](screenshots/screenshot2.png?raw=true "Chat")
![MinecraftLightServer player moved](screenshots/screenshot3.png?raw=true "Player moved")

## Changelog
- There can be only one username online at the same time
- Concurrent handling of clients
- Concurrent handling of client packets
- Support for player running
- Detection of a disconnected player is immediate

### Changes for the future
- Support for chunk generation
- Support for mobs
- Game changes saving
- Support for more client packets
- Plugins