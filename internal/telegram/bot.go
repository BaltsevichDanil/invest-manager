package telegram

import (
	"fmt"
	"invest-manager/internal/analysis"
	"invest-manager/internal/config"
	"invest-manager/internal/invest"
	"log"
	"strings"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

// Bot handles Telegram communication
type Bot struct {
	api      *tgbotapi.BotAPI
	chatID   string
	logger   *log.Logger
}

// NewBot creates a new Telegram bot
func NewBot(cfg *config.Config, logger *log.Logger) (*Bot, error) {
	api, err := tgbotapi.NewBotAPI(cfg.TelegramToken)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize Telegram bot: %w", err)
	}
	
	return &Bot{
		api:    api,
		chatID: cfg.TelegramChatID,
		logger: logger,
	}, nil
}

// SendMessage sends a simple text message
func (b *Bot) SendMessage(text string) error {
	// Check if message is too long for Telegram
	const maxMessageLength = 4096
	
	if len(text) <= maxMessageLength {
		// Send as a single message
		msg := tgbotapi.NewMessage(parseChatID(b.chatID), text)
		_, err := b.api.Send(msg)
		if err != nil {
			return fmt.Errorf("failed to send Telegram message: %w", err)
		}
	} else {
		// Split into multiple messages
		chunks := splitMessage(text, maxMessageLength)
		for i, chunk := range chunks {
			b.logger.Printf("Sending message part %d/%d", i+1, len(chunks))
			
			msg := tgbotapi.NewMessage(parseChatID(b.chatID), chunk)
			_, err := b.api.Send(msg)
			if err != nil {
				return fmt.Errorf("failed to send Telegram message part %d: %w", i+1, err)
			}
		}
	}
	
	return nil
}

// SendPortfolioAnalysis sends a formatted portfolio analysis report
func (b *Bot) SendPortfolioAnalysis(portfolio *invest.Portfolio, analysis *analysis.PortfolioAnalysis) error {
	var sb strings.Builder
	
	// Create header
	sb.WriteString("📊 *PORTFOLIO ANALYSIS* 📊\n\n")
	
	// Add summary
	sb.WriteString("*SUMMARY:*\n")
	sb.WriteString(analysis.Summary)
	sb.WriteString("\n\n")
	
	// Add portfolio overview
	sb.WriteString("*PORTFOLIO OVERVIEW:*\n")
	sb.WriteString(fmt.Sprintf("Total Value: %.2f %s\n", portfolio.TotalAmount, portfolio.Currency))
	sb.WriteString(fmt.Sprintf("Expected Yield: %.2f %s\n\n", portfolio.ExpectedYield, portfolio.Currency))
	
	// Add recommendations
	sb.WriteString("*RECOMMENDATIONS:*\n\n")
	
	for _, rec := range analysis.Recommendations {
		// Format action with emoji
		actionEmoji := "🔄" // HOLD
		if rec.Action == "BUY" {
			actionEmoji = "🟢"
		} else if rec.Action == "SELL" {
			actionEmoji = "🔴"
		}
		
		sb.WriteString(fmt.Sprintf("*%s (%s)* - %s %s\n", rec.Ticker, rec.Name, actionEmoji, rec.Action))
		sb.WriteString(fmt.Sprintf("_%s_\n\n", rec.Reason))
	}
	
	// Add monthly reminder if needed
	if analysis.IsMonthlyReminder {
		sb.WriteString("\n⚠️ *REMINDER* ⚠️\n")
		sb.WriteString("Don't forget to add funds and redistribute your portfolio this month!\n")
	}
	
	// Send the message
	msg := tgbotapi.NewMessage(parseChatID(b.chatID), sb.String())
	msg.ParseMode = tgbotapi.ModeMarkdown
	
	_, err := b.api.Send(msg)
	if err != nil {
		// If markdown fails, try without formatting
		b.logger.Printf("Error sending formatted message: %v. Trying without markdown", err)
		plainMsg := tgbotapi.NewMessage(parseChatID(b.chatID), stripMarkdown(sb.String()))
		_, err = b.api.Send(plainMsg)
		if err != nil {
			return fmt.Errorf("failed to send portfolio analysis: %w", err)
		}
	}
	
	return nil
}

// Helper function to parse chat ID from string to int64
func parseChatID(chatID string) int64 {
	var id int64
	fmt.Sscanf(chatID, "%d", &id)
	return id
}

// Helper function to split a message into chunks
func splitMessage(message string, maxLength int) []string {
	if len(message) <= maxLength {
		return []string{message}
	}
	
	var chunks []string
	for len(message) > 0 {
		if len(message) <= maxLength {
			chunks = append(chunks, message)
			break
		}
		
		// Try to split at newline to preserve formatting
		cutIndex := strings.LastIndex(message[:maxLength], "\n")
		if cutIndex == -1 || cutIndex < maxLength/2 {
			// If no suitable newline found, split at maxLength
			cutIndex = maxLength
		}
		
		chunks = append(chunks, message[:cutIndex])
		message = message[cutIndex:]
	}
	
	return chunks
}

// Helper function to strip markdown for plain text fallback
func stripMarkdown(text string) string {
	text = strings.ReplaceAll(text, "*", "")
	text = strings.ReplaceAll(text, "_", "")
	return text
} 