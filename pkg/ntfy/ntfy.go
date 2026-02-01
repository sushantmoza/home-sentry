package ntfy

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"
)

// Command types that can be received via ntfy
type Command string

const (
	CmdCancelAndPause Command = "cancel_pause" // Cancel shutdown and pause protection
	CmdCancelOnly     Command = "cancel_only"  // Cancel shutdown, keep monitoring
	CmdPause          Command = "pause"        // Pause protection
	CmdResume         Command = "resume"       // Resume protection
	CmdStatus         Command = "status"       // Request status update
)

// NotificationPayload represents an ntfy notification with actions
type NotificationPayload struct {
	Topic    string   `json:"topic"`
	Title    string   `json:"title"`
	Message  string   `json:"message"`
	Priority int      `json:"priority"`
	Tags     []string `json:"tags"`
	Actions  []Action `json:"actions"`
}

// Action represents a notification action button
type Action struct {
	Action string `json:"action"`
	Label  string `json:"label"`
	URL    string `json:"url,omitempty"`
	Method string `json:"method,omitempty"`
	Body   string `json:"body,omitempty"`
	Clear  bool   `json:"clear,omitempty"`
}

// Client handles ntfy.sh communication
type Client struct {
	server     string
	topic      string
	cancelFunc context.CancelFunc
	mu         sync.Mutex
	listening  bool
}

// NewClient creates a new ntfy client
func NewClient(server, topic string) *Client {
	// Ensure server has no trailing slash
	server = strings.TrimSuffix(server, "/")
	return &Client{
		server: server,
		topic:  topic,
	}
}

// SendShutdownNotification sends a notification with multiple action buttons
func (c *Client) SendShutdownNotification(delaySeconds int) error {
	payload := NotificationPayload{
		Topic:    c.topic,
		Title:    "🚨 Home Sentry Alert",
		Message:  fmt.Sprintf("Phone not detected! Laptop shutting down in %d seconds...", delaySeconds),
		Priority: 5, // Max priority
		Tags:     []string{"warning", "computer", "rotating_light"},
		Actions: []Action{
			{
				Action: "http",
				Label:  "⏸ Cancel & Pause",
				URL:    fmt.Sprintf("%s/%s", c.server, c.topic),
				Method: "POST",
				Body:   string(CmdCancelAndPause),
				Clear:  true,
			},
			{
				Action: "http",
				Label:  "❌ Cancel Only",
				URL:    fmt.Sprintf("%s/%s", c.server, c.topic),
				Method: "POST",
				Body:   string(CmdCancelOnly),
				Clear:  true,
			},
		},
	}

	return c.sendNotification(payload)
}

// SendStatusNotification sends the current status to the phone
func (c *Client) SendStatusNotification(status, wifiName, phoneMac string, isPaused bool) error {
	var emoji, stateText string
	if isPaused {
		emoji = "⏸"
		stateText = "PAUSED"
	} else {
		emoji = "🟢"
		stateText = status
	}

	message := fmt.Sprintf("Status: %s\nWiFi: %s\nPhone: %s", stateText, wifiName, phoneMac)

	payload := NotificationPayload{
		Topic:    c.topic,
		Title:    fmt.Sprintf("%s Home Sentry Status", emoji),
		Message:  message,
		Priority: 3,
		Tags:     []string{"house", "information_source"},
		Actions: []Action{
			{
				Action: "http",
				Label:  "⏸ Pause",
				URL:    fmt.Sprintf("%s/%s", c.server, c.topic),
				Method: "POST",
				Body:   string(CmdPause),
				Clear:  true,
			},
			{
				Action: "http",
				Label:  "▶ Resume",
				URL:    fmt.Sprintf("%s/%s", c.server, c.topic),
				Method: "POST",
				Body:   string(CmdResume),
				Clear:  true,
			},
		},
	}

	return c.sendNotification(payload)
}

// SendPausedNotification confirms protection was paused
func (c *Client) SendPausedNotification() error {
	payload := NotificationPayload{
		Topic:    c.topic,
		Title:    "⏸ Protection Paused",
		Message:  "Home Sentry protection is paused. Send 'resume' or tap the button to resume.",
		Priority: 3,
		Tags:     []string{"pause_button"},
		Actions: []Action{
			{
				Action: "http",
				Label:  "▶ Resume Protection",
				URL:    fmt.Sprintf("%s/%s", c.server, c.topic),
				Method: "POST",
				Body:   string(CmdResume),
				Clear:  true,
			},
		},
	}

	return c.sendNotification(payload)
}

// SendResumedNotification confirms protection was resumed
func (c *Client) SendResumedNotification() error {
	payload := NotificationPayload{
		Topic:    c.topic,
		Title:    "▶ Protection Resumed",
		Message:  "Home Sentry is now monitoring your laptop.",
		Priority: 3,
		Tags:     []string{"arrow_forward", "shield"},
		Actions:  []Action{},
	}

	return c.sendNotification(payload)
}

// SendTestNotification sends a test notification to verify configuration
func (c *Client) SendTestNotification() error {
	payload := NotificationPayload{
		Topic:    c.topic,
		Title:    "✅ Home Sentry Test",
		Message:  "Notifications working! Commands: 'pause', 'resume', 'status'",
		Priority: 3,
		Tags:     []string{"white_check_mark", "computer"},
		Actions: []Action{
			{
				Action: "http",
				Label:  "⏸ Pause",
				URL:    fmt.Sprintf("%s/%s", c.server, c.topic),
				Method: "POST",
				Body:   string(CmdPause),
				Clear:  true,
			},
			{
				Action: "http",
				Label:  "📊 Status",
				URL:    fmt.Sprintf("%s/%s", c.server, c.topic),
				Method: "POST",
				Body:   string(CmdStatus),
				Clear:  true,
			},
		},
	}

	return c.sendNotification(payload)
}

func (c *Client) sendNotification(payload NotificationPayload) error {
	jsonData, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal notification: %w", err)
	}

	resp, err := http.Post(c.server, "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("failed to send notification: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("ntfy returned status %d: %s", resp.StatusCode, string(body))
	}

	log.Printf("ntfy notification sent successfully to topic: %s", c.topic)
	return nil
}

// CommandCallback is called when a command is received
type CommandCallback func(cmd Command)

// StartCommandListener starts listening for all commands on the topic
// The callback is called whenever a command is received
func (c *Client) StartCommandListener(callback CommandCallback) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.listening {
		return fmt.Errorf("listener already running")
	}

	ctx, cancel := context.WithCancel(context.Background())
	c.cancelFunc = cancel
	c.listening = true

	go c.listenForCommands(ctx, callback)

	return nil
}

// StartShutdownCancelListener starts a temporary listener just for shutdown cancel commands
// Returns a channel that signals which type of cancel was received
func (c *Client) StartShutdownCancelListener() (<-chan Command, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.listening {
		return nil, fmt.Errorf("listener already running")
	}

	cmdCh := make(chan Command, 1)
	ctx, cancel := context.WithCancel(context.Background())
	c.cancelFunc = cancel
	c.listening = true

	go c.listenForShutdownCancel(ctx, cmdCh)

	return cmdCh, nil
}

// StopListener stops the command listener
func (c *Client) StopListener() {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.cancelFunc != nil {
		c.cancelFunc()
		c.cancelFunc = nil
	}
	c.listening = false
}

// IsListening returns true if the listener is active
func (c *Client) IsListening() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.listening
}

func (c *Client) listenForCommands(ctx context.Context, callback CommandCallback) {
	defer func() {
		c.mu.Lock()
		c.listening = false
		c.mu.Unlock()
	}()

	// Use polling approach
	url := fmt.Sprintf("%s/%s/json?poll=1&since=10s", c.server, c.topic)
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	log.Printf("ntfy: Started command listener on %s/%s", c.server, c.topic)

	// Track last seen message to avoid duplicates
	var lastMessageID string

	for {
		select {
		case <-ctx.Done():
			log.Println("ntfy: Command listener stopped")
			return
		case <-ticker.C:
			cmd, msgID := c.checkForCommand(url, lastMessageID)
			if cmd != "" && msgID != lastMessageID {
				lastMessageID = msgID
				log.Printf("ntfy: Received command: %s", cmd)
				callback(cmd)
			}
		}
	}
}

func (c *Client) listenForShutdownCancel(ctx context.Context, cmdCh chan<- Command) {
	defer func() {
		c.mu.Lock()
		c.listening = false
		c.mu.Unlock()
		close(cmdCh)
	}()

	url := fmt.Sprintf("%s/%s/json?poll=1&since=10s", c.server, c.topic)
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	log.Printf("ntfy: Started shutdown cancel listener on %s/%s", c.server, c.topic)

	for {
		select {
		case <-ctx.Done():
			log.Println("ntfy: Shutdown cancel listener stopped")
			return
		case <-ticker.C:
			cmd, _ := c.checkForCommand(url, "")
			if cmd == CmdCancelAndPause || cmd == CmdCancelOnly {
				log.Printf("ntfy: Received shutdown cancel: %s", cmd)
				select {
				case cmdCh <- cmd:
				default:
				}
				return
			}
		}
	}
}

// NtfyMessage represents a message from ntfy
type NtfyMessage struct {
	ID      string `json:"id"`
	Message string `json:"message"`
}

func (c *Client) checkForCommand(url string, lastID string) (Command, string) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return "", ""
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", ""
	}
	defer resp.Body.Close()

	scanner := bufio.NewScanner(resp.Body)
	var latestCmd Command
	var latestID string

	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}

		var msg NtfyMessage
		if err := json.Unmarshal([]byte(line), &msg); err != nil {
			continue
		}

		// Skip if we've already processed this message
		if msg.ID == lastID {
			continue
		}

		// Check for commands
		msgLower := strings.ToLower(strings.TrimSpace(msg.Message))
		switch msgLower {
		case string(CmdCancelAndPause):
			latestCmd = CmdCancelAndPause
			latestID = msg.ID
		case string(CmdCancelOnly):
			latestCmd = CmdCancelOnly
			latestID = msg.ID
		case string(CmdPause):
			latestCmd = CmdPause
			latestID = msg.ID
		case string(CmdResume):
			latestCmd = CmdResume
			latestID = msg.ID
		case string(CmdStatus):
			latestCmd = CmdStatus
			latestID = msg.ID
		}
	}

	return latestCmd, latestID
}
