package main

import "github.com/adamluo159/gameAgent/agentClient"

func main() {
	a := agentClient.New()
	a.Connect()
}
