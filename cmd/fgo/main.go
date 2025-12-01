package main

import (
	"fmt"
	"os"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Println("fgo: missing command (init, add, commit, push, pull, status)")
		os.Exit(1)
	}
	cmd := os.Args[1]
	switch cmd {
	case "config":
		fmt.Println("[stub] fgo config <key> <value>")
	case "init":
		fmt.Println("[stub] fgo init <box>")
	case "add":
		fmt.Println("[stub] fgo add <path>")
	case "commit":
		fmt.Println("[stub] fgo commit -m 'msg'")
	case "push":
		fmt.Println("[stub] fgo push")
	case "pull":
		fmt.Println("[stub] fgo pull")
	case "status":
		fmt.Println("[stub] fgo status")
	default:
		fmt.Printf("fgo: unknown command '%s'\n", cmd)
		os.Exit(2)
	}
}
