package main

import "github.com/net0pyr/custom-container/commands"

var commandMap map[string]func()

func init() {
	commandMap = map[string]func(){
		"create": commands.Create,
		"help":   commands.Help,
	}
}
