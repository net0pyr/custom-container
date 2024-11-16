package main

import (
	"fmt"
	"log"
	"os"
)

func main() {
	file, err := os.OpenFile("app.log", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err != nil {
		log.Fatal(err)
	}
	defer file.Close()
	log.SetOutput(file)

	args := os.Args[1:]

	for _, arg := range args {
		if cmd, exists := commandMap[arg]; exists {
			cmd()
		} else {
			fmt.Printf("Unknown argument: %s\n", arg)
		}
	}
}
