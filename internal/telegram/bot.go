package telegram

import (
	"context"
	"fmt"
	"invest-manager/internal/analysis"
	"invest-manager/internal/config"
	"invest-manager/internal/invest"
	"invest-manager/internal/news"
	"log"
	"strings"
	"sync"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

// Bot handles Telegram communication
type Bot struct {
	api         *tgbotapi.BotAPI
	chatID      string
	logger      *log.Logger
	investor    *invest.Client
	analyzer    *analysis.Analyzer
	newsFetcher *news.Fetcher
	stopChan    chan struct{}
	wg          sync.WaitGroup
}

// NewBot creates a new Telegram bot
func NewBot(cfg *config.Config, logger *log.Logger, 
	investor *invest.Client, analyzer *analysis.Analyzer, 
	newsFetcher *news.Fetcher) (*Bot, error) {
	api, err := tgbotapi.NewBotAPI(cfg.TelegramToken)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize Telegram bot: %w", err)
	}
	
	return &Bot{
		api:         api,
		chatID:      cfg.TelegramChatID,
		logger:      logger,
		investor:    investor,
		analyzer:    analyzer,
		newsFetcher: newsFetcher,
		stopChan:    make(chan struct{}),
	}, nil
}

// Start begins listening for commands from the authorized user
func (b *Bot) Start() {
	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60

	updates := b.api.GetUpdatesChan(u)

	b.wg.Add(1)
	go b.handleUpdates(updates)
	
	b.logger.Println("Telegram bot started and listening for commands")
}

// Stop stops the bot
func (b *Bot) Stop() {
	close(b.stopChan)
	b.wg.Wait()
	b.api.StopReceivingUpdates()
	b.logger.Println("Telegram bot stopped")
}

// handleUpdates processes incoming messages and commands
func (b *Bot) handleUpdates(updates tgbotapi.UpdatesChannel) {
	defer b.wg.Done()
	
	for {
		select {
		case <-b.stopChan:
			return
		case update := <-updates:
			if update.Message == nil {
				continue
			}

			// Only process messages from authorized chat ID
			chatIDStr := fmt.Sprintf("%d", update.Message.Chat.ID)
			if chatIDStr != b.chatID {
				b.logger.Printf("Received message from unauthorized chat: %s", chatIDStr)
				continue
			}

			// Process commands
			if update.Message.IsCommand() {
				b.handleCommand(update.Message)
			}
		}
	}
}

// handleCommand processes bot commands
func (b *Bot) handleCommand(message *tgbotapi.Message) {
	switch message.Command() {
	case "analyze":
		b.logger.Printf("Received analyze command from chat ID: %d", message.Chat.ID)
		b.handleAnalyzeCommand(message)
	case "help":
		b.handleHelpCommand(message)
	case "status":
		b.handleStatusCommand(message)
	default:
		b.sendMessage("Неизвестная команда. Используйте /help для списка доступных команд.")
	}
}

// handleAnalyzeCommand performs immediate portfolio analysis
func (b *Bot) handleAnalyzeCommand(message *tgbotapi.Message) {
	replyMsg := tgbotapi.NewMessage(message.Chat.ID, "🔄 Запускаю анализ вашего портфеля...")
	b.api.Send(replyMsg)
	
	// Run analysis in a separate goroutine to not block message handling
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 60*1000*1000*1000) // 60 sec timeout
		defer cancel()
		
		// Get portfolio
		b.logger.Println("Getting portfolio...")
		portfolio, err := b.investor.GetPortfolio(ctx)
		if err != nil {
			errorMsg := fmt.Sprintf("Ошибка при получении портфеля: %v", err)
			b.logger.Println(errorMsg)
			b.sendMessage(errorMsg)
			return
		}
		
		// Get news about Russia
		b.logger.Println("Fetching news about Russia...")
		articles, err := b.newsFetcher.FetchNews("Russia", 5)
		if err != nil {
			b.logger.Printf("Warning: could not fetch news: %v, continuing without news", err)
			articles = []news.Article{} // Empty but continue
		}
		
		// Analyze portfolio
		b.logger.Println("Analyzing portfolio...")
		analysis, err := b.analyzer.AnalyzePortfolio(ctx, portfolio, articles, false)
		if err != nil {
			errorMsg := fmt.Sprintf("Ошибка при анализе портфеля: %v", err)
			b.logger.Println(errorMsg)
			b.sendMessage(errorMsg)
			return
		}
		
		// Send analysis results with fresh news
		err = b.SendPortfolioAnalysis(portfolio, analysis, articles)
		if err != nil {
			b.logger.Printf("Error sending portfolio analysis: %v", err)
			b.sendMessage(fmt.Sprintf("Ошибка при отправке анализа: %v", err))
		}
	}()
}

// handleHelpCommand shows available commands
func (b *Bot) handleHelpCommand(message *tgbotapi.Message) {
	helpText := `🤖 *Доступные команды*:

/analyze - запустить анализ портфеля прямо сейчас
/status - проверить статус бота
/help - показать это сообщение

Бот также автоматически анализирует ваш портфель каждый день в 7:00 (МСК).`

	msg := tgbotapi.NewMessage(message.Chat.ID, helpText)
	msg.ParseMode = tgbotapi.ModeMarkdown
	b.api.Send(msg)
}

// handleStatusCommand shows bot status
func (b *Bot) handleStatusCommand(message *tgbotapi.Message) {
	statusText := "✅ Бот работает нормально. Ежедневный анализ портфеля выполняется в 7:00 (МСК)."
	
	msg := tgbotapi.NewMessage(message.Chat.ID, statusText)
	b.api.Send(msg)
}

// SendMessage sends a simple text message
func (b *Bot) SendMessage(text string) error {
	return b.sendMessage(text)
}

// sendMessage is an internal method to send a simple text message
func (b *Bot) sendMessage(text string) error {
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

// SendPortfolioAnalysis sends a formatted portfolio analysis report along with fresh news articles
func (b *Bot) SendPortfolioAnalysis(portfolio *invest.Portfolio, analysis *analysis.PortfolioAnalysis, articles []news.Article) error {
	var sb strings.Builder
	
	// Add fresh news section
	sb.WriteString("📰 *NEWS:*")
	if len(articles) == 0 {
		sb.WriteString("\nНет доступных новостей.\n\n")
	} else {
		sb.WriteString("\n\n")
		for _, article := range articles {
			sb.WriteString(fmt.Sprintf("*%s*\n", article.Title))
			sb.WriteString(fmt.Sprintf("Source: %s\n", article.Source.Name))
			sb.WriteString(fmt.Sprintf("Date: %s\n", article.PublishedAt.Format("2006-01-02")))
			sb.WriteString(fmt.Sprintf("URL: %s\n\n", article.URL))
		}
	}
	
	// Portfolio analysis header
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
	
	// After sending recommendations, add opportunities if available
	if analysis.Opportunities != nil && len(analysis.Opportunities) > 0 {
		sb.WriteString("\n*OPPORTUNITIES:*\n")
		for _, opp := range analysis.Opportunities {
			// Determine emoji for LONG/SHORT
			actionEmoji := "📈" // default LONG
			if strings.ToUpper(opp.Action) == "SHORT" {
				actionEmoji = "📉"
			}
			sb.WriteString(fmt.Sprintf("*%s (%s)* - %s %s\n", opp.Ticker, opp.Name, actionEmoji, strings.ToUpper(opp.Action)))
			sb.WriteString(fmt.Sprintf("_%s_\n\n", opp.Reason))
		}
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