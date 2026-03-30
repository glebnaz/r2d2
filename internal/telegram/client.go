package telegram

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

// Client wraps the Telegram Bot API for sending messages.
type Client struct {
	bot    *tgbotapi.BotAPI
	chatID int64
}

// New creates a new Telegram client. It validates the token by calling GetMe.
func New(token string, chatID int64) (*Client, error) {
	bot, err := tgbotapi.NewBotAPI(token)
	if err != nil {
		return nil, fmt.Errorf("creating telegram bot: %w", err)
	}

	slog.Info("telegram bot authorized", "username", bot.Self.UserName)

	return &Client{
		bot:    bot,
		chatID: chatID,
	}, nil
}

// SendMessage sends a Markdown-formatted message to the configured chat.
func (c *Client) SendMessage(ctx context.Context, text string) error {
	msg := tgbotapi.NewMessage(c.chatID, text)
	msg.ParseMode = tgbotapi.ModeMarkdown

	var lastErr error
	for attempt := 0; attempt < 3; attempt++ {
		if ctx.Err() != nil {
			return ctx.Err()
		}

		_, err := c.bot.Send(msg)
		if err == nil {
			return nil
		}
		lastErr = err
		slog.Warn("telegram send failed, retrying", "attempt", attempt+1, "error", err)

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(time.Duration(attempt+1) * time.Second):
		}
	}

	return fmt.Errorf("sending telegram message after 3 attempts: %w", lastErr)
}

// StartPolling starts long polling for incoming messages. It blocks until
// the context is cancelled, providing graceful shutdown.
func (c *Client) StartPolling(ctx context.Context) {
	u := tgbotapi.NewUpdate(0)
	u.Timeout = 30

	updates := c.bot.GetUpdatesChan(u)

	for {
		select {
		case <-ctx.Done():
			c.bot.StopReceivingUpdates()
			slog.Info("telegram polling stopped")
			return
		case update := <-updates:
			if update.Message != nil {
				slog.Info("received message", "from", update.Message.From.UserName, "text", update.Message.Text)
			}
		}
	}
}

// Stop gracefully stops the bot by stopping the update channel.
func (c *Client) Stop() {
	c.bot.StopReceivingUpdates()
}
