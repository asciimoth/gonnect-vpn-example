package main

import (
	"log"
	"os"

	"github.com/asciimoth/gonnect-vpn-example/logger"
)

func main() {
	var logger logger.Logger = log.New(os.Stdout, "", log.Ltime|log.Lmicroseconds)
	logger.Println("Test")
}
