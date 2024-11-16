package commands

import (
	"fmt"
	"log"
)

var commandDescriptionMap map[string]string

func init() {
	commandDescriptionMap = map[string]string{
		"create": "Create a new container",
		"help":   "Print help message",
	}
}

func Help() {
	log.Println("Showing help message")
	for command, description := range commandDescriptionMap {
		fmt.Printf("\t%s: %s\n", command, description)
	}
}
