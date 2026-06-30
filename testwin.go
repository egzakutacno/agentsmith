package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
)

func main() {
	logFile := filepath.Join(os.TempDir(), "agentsmith-test.log")
	f, _ := os.OpenFile(logFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	fmt.Fprintf(f, "Test started\n")
	log.SetOutput(f)

	fmt.Println("=== AGENTSMITH TEST ===")
	fmt.Println("This is console output")
	fmt.Printf("Temp dir: %s\n", os.TempDir())
	fmt.Printf("Log file: %s\n", logFile)
	fmt.Println("If you see this, console output works!")

	log.Println("Log test entry")

	hostname, _ := os.Hostname()
	fmt.Printf("Hostname: %s\n", hostname)

	fmt.Println("=== TEST COMPLETE ===")
}
