package main

import (
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"agentsmith/mqtt"
	"agentsmith/server"
)

const version = "1.0.0"

func initLog() {
	logFile := filepath.Join(os.TempDir(), "agentsmith.log")
	f, err := os.OpenFile(logFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return
	}
	log.SetOutput(io.MultiWriter(f, os.Stdout))
	log.SetFlags(log.Ldate | log.Ltime | log.Lshortfile)
}

func main() {
	install := flag.Bool("install", false, "Install as scheduled task ONLOGON")
	remove := flag.Bool("remove", false, "Remove scheduled task and kill process")
	flag.Parse()

	if *install {
		doInstall()
		return
	}
	if *remove {
		doRemove()
		return
	}

	initLog()
	log.Printf("AgentSmith %s starting...", version)

	hostname, _ := os.Hostname()

	mqttClient, err := mqtt.New("tcp://test.mosquitto.org:1883")
	if err != nil {
		log.Fatalf("Failed to start MQTT client: %v", err)
	}
	defer mqttClient.Disconnect(250)

	log.Printf("Connected to MQTT broker, agent/%s/", hostname)

	srv := server.New(hostname, version, time.Now())
	go func() {
		log.Println("HTTP server on 127.0.0.1:8080")
		if err := srv.Start("127.0.0.1:8080"); err != nil {
			log.Printf("HTTP server error: %v", err)
		}
	}()
	defer srv.Stop()

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	<-sig
	log.Println("Shutting down...")
}

func doInstall() {
	exe, _ := os.Executable()
	name := "AgentSmith"

	dest := filepath.Join(os.Getenv("ProgramFiles"), "AgentSmith", "agentsmith.exe")
	os.MkdirAll(filepath.Dir(dest), 0755)
	copyFile(exe, dest)

	psCmd := fmt.Sprintf(
		`powershell -WindowStyle Hidden -Command Start-Process -FilePath '%s' -WindowStyle Hidden`,
		dest,
	)

	exec.Command("schtasks", "/create",
		"/tn", name, "/tr", psCmd,
		"/sc", "ONLOGON", "/ru", os.Getenv("USERNAME"), "/f",
	).Run()

	exec.Command("schtasks", "/run", "/tn", name).Run()
	fmt.Println("AgentSmith installed and started.")
}

func doRemove() {
	exec.Command("taskkill", "/f", "/im", "agentsmith.exe").Run()
	exec.Command("schtasks", "/delete", "/tn", "AgentSmith", "/f").Run()
	os.RemoveAll(filepath.Join(os.Getenv("ProgramFiles"), "AgentSmith"))
	fmt.Println("AgentSmith removed.")
}

func copyFile(src, dst string) {
	data, _ := os.ReadFile(src)
	os.MkdirAll(filepath.Dir(dst), 0755)
	os.WriteFile(dst, data, 0644)
}
