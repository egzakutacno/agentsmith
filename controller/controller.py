#!/usr/bin/env python3
"""
AgentSmith Controller — RPi MQTT client za slanje komandi Windows agentima.

Usage:
  python controller.py list                    # Lista aktivnih agenata
  python controller.py exec <hostname> <cmd>   # Izvrši cmd komandu
  python controller.py ps <hostname> <cmd>     # Izvrši PowerShell komandu
  python controller.py info <hostname>         # Informacije o agentu
  python controller.py shell <hostname>        # Interaktivni shell
"""

import json
import sys
import time
import threading

import paho.mqtt.client as mqtt

BROKER = "test.mosquitto.org"
PORT = 1883
TIMEOUT = 10


class AgentSmithController:
    def __init__(self):
        self.client = mqtt.Client(client_id=f"controller-{int(time.time())}")
        self.client.on_connect = self._on_connect
        self.client.on_message = self._on_message
        self.agents = {}
        self.pending_resp = None
        self.resp_event = threading.Event()

    def _on_connect(self, client, userdata, flags, rc):
        print(f"[Connected to {BROKER}:{PORT}]" if rc == 0 else f"[Connection failed: {rc}]")
        if rc == 0:
            client.subscribe("agent/+/status", qos=0)
            print("[Listening for agent heartbeats...]")

    def _on_message(self, client, userdata, msg):
        topic = msg.topic
        try:
            payload = json.loads(msg.payload)
        except json.JSONDecodeError:
            return

        if topic.endswith("/status") and "hostname" in payload:
            self.agents[payload["hostname"]] = payload

        if topic.endswith("/resp"):
            self.pending_resp = payload
            self.resp_event.set()

    def start(self):
        self.client.connect(BROKER, PORT, 60)
        self.client.loop_start()

    def stop(self):
        self.client.loop_stop()
        self.client.disconnect()

    def list_agents(self):
        print(f"\n{'Hostname':<20} {'Uptime':<10} {'Version':<10} {'Last Seen'}")
        print("-" * 60)
        self.client.publish("agent/+/status", "").wait_for_publish()
        time.sleep(2)
        if not self.agents:
            print("  No active agents found.")
        for hostname, info in sorted(self.agents.items()):
            uptime = f"{info.get('uptime', 0):.0f}s"
            version = info.get("version", "?")
            ts = info.get("timestamp", "?")
            print(f"{hostname:<20} {uptime:<10} {version:<10} {ts}")

    def send_command(self, hostname, cmd_type, command=""):
        topic = f"agent/{hostname}/cmd"
        resp_topic = f"agent/{hostname}/resp"
        self.client.subscribe(resp_topic)

        payload = {"cmd": cmd_type}
        if command:
            payload["command"] = command

        self.resp_event.clear()
        self.pending_resp = None

        print(f"  Sending {cmd_type} to {hostname}...")
        self.client.publish(topic, json.dumps(payload))

        if self.resp_event.wait(timeout=TIMEOUT):
            resp = self.pending_resp
            if resp.get("error"):
                print(f"  Error: {resp['error']}")
            else:
                if cmd_type == "info":
                    print(f"  Hostname: {resp.get('hostname')}")
                    print(f"  Uptime:   {resp.get('uptime', 0)}s")
                    print(f"  Version:  {resp.get('version')}")
                else:
                    if resp.get("stdout"):
                        print(f"  Output:\n{resp['stdout']}")
                    if resp.get("stderr"):
                        print(f"  Stderr:\n{resp['stderr']}")
                    print(f"  Exit code: {resp.get('exit_code')}")
        else:
            print(f"  Timeout: no response from {hostname}")

    def interactive_shell(self, hostname):
        print(f"  AgentSmith interactive shell — {hostname}")
        print(f"  Commands starting with 'ps:' run PowerShell, otherwise cmd.exe.")
        print(f"  Type 'exit' or Ctrl+C to quit.\n")
        try:
            while True:
                line = input(f"  {hostname}> ").strip()
                if not line:
                    continue
                if line.lower() in ("exit", "quit"):
                    break
                if line.startswith("ps:"):
                    cmd_type = "ps"
                    command = line[3:].strip()
                else:
                    cmd_type = "execute"
                    command = line
                self.send_command(hostname, cmd_type, command)
        except (EOFError, KeyboardInterrupt):
            print()


def main():
    if len(sys.argv) < 2:
        print(__doc__)
        return

    ctrl = AgentSmithController()
    ctrl.start()

    try:
        cmd = sys.argv[1]

        if cmd == "list":
            ctrl.list_agents()

        elif cmd in ("exec", "ps", "info"):
            if len(sys.argv) < 3:
                print(f"Usage: python controller.py {cmd} <hostname> [command]")
                return
            hostname = sys.argv[2]
            command = sys.argv[3] if len(sys.argv) > 3 else ""
            ctrl.send_command(hostname, cmd, command)

        elif cmd == "shell":
            if len(sys.argv) < 3:
                print("Usage: python controller.py shell <hostname>")
                return
            ctrl.interactive_shell(sys.argv[2])

        else:
            print(f"Unknown command: {cmd}")
            print(__doc__)

    finally:
        ctrl.stop()


if __name__ == "__main__":
    main()
