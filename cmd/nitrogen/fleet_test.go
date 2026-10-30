package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestNewFleetDefaultsToNATSAndValkey(t *testing.T) {
	dir := t.TempDir()
	root := filepath.Join(dir, "fl")
	cmdNewFleet(newOpts{name: root, module: "github.com/acme/fl", services: []string{"gateway", "mail"}})

	compose := readFile(t, filepath.Join(root, "docker-compose.yml"))
	for _, want := range []string{"nats:2.", "valkey/valkey:", "- nats\n"} {
		if !strings.Contains(compose, want) {
			t.Errorf("docker-compose.yml lacks %q", want)
		}
	}
	for _, unwanted := range []string{"rabbitmq", "redis:"} {
		if strings.Contains(compose, unwanted) {
			t.Errorf("docker-compose.yml still has %q", unwanted)
		}
	}
	env := readFile(t, filepath.Join(root, "services", "mail", ".env"))
	if !strings.Contains(env, "NATS_URL=") || strings.Contains(env, "RABBITMQ_URL") {
		t.Errorf("worker .env should configure NATS only:\n%s", env)
	}
	main := readFile(t, filepath.Join(root, "services", "mail", "cmd", "worker", "main.go"))
	if !strings.Contains(main, "nitro.QueueFromEnv()") {
		t.Error("worker main should pick the broker with nitro.QueueFromEnv")
	}
	gomod := readFile(t, filepath.Join(root, "services", "mail", "go.mod"))
	if !strings.Contains(gomod, "go-nitro/pkg/v2 "+frameworkVersion) {
		t.Errorf("go.mod should require %s:\n%s", frameworkVersion, gomod)
	}
}

func TestNewFleetRabbitMQBroker(t *testing.T) {
	dir := t.TempDir()
	root := filepath.Join(dir, "fl")
	cmdNewFleet(newOpts{name: root, module: "github.com/acme/fl", services: []string{"gateway", "mail"}, broker: "rabbitmq"})

	compose := readFile(t, filepath.Join(root, "docker-compose.yml"))
	if !strings.Contains(compose, "rabbitmq:3.") || strings.Contains(compose, "nats:2.") {
		t.Errorf("--broker rabbitmq should run RabbitMQ only:\n%s", compose)
	}
	if env := readFile(t, filepath.Join(root, "services", "mail", ".env")); !strings.Contains(env, "RABBITMQ_URL=") {
		t.Errorf("worker .env should configure RabbitMQ:\n%s", env)
	}
}

func TestParseNewArgsBroker(t *testing.T) {
	o, ok := parseNewArgs([]string{"backend", "--fleet", "--broker", "rabbitmq"})
	if !ok || o.broker != "rabbitmq" {
		t.Fatalf("got ok=%v broker=%q", ok, o.broker)
	}
	o, ok = parseNewArgs([]string{"backend", "--fleet", "--broker=nats"})
	if !ok || o.broker != "nats" {
		t.Fatalf("got ok=%v broker=%q", ok, o.broker)
	}
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}
