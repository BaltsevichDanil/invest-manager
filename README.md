# Invest Manager Bot

A Telegram bot that provides daily analysis of your Tinkoff Invest portfolio using AI-powered recommendations based on market news.

## Features

- Fetches your portfolio data from Tinkoff Invest API
- Collects recent news about Russian stocks
- Analyzes portfolio positions using OpenAI (GPT-4)
- Sends actionable recommendations (BUY/SELL/HOLD) with explanations
- Runs automatically every day at 7:00 MSK
- Provides monthly reminders to add funds and rebalance your portfolio

## Requirements

- Go 1.18 or higher
- Tinkoff Invest API token
- Telegram Bot API token
- OpenAI API key
- NewsAPI.org API key

## Environment Variables

The application uses the following environment variables:

- `TINKOFF_TOKEN` - Your Tinkoff Invest API token
- `TINKOFF_ENDPOINT` - (Optional) Custom Tinkoff API endpoint
- `OPENAI_API_KEY` - Your OpenAI API key
- `TELEGRAM_TOKEN` - Your Telegram Bot token
- `TELEGRAM_CHAT_ID` - Your Telegram chat ID for receiving notifications
- `NEWSAPI_TOKEN` - Your NewsAPI.org API key
- `TIMEZONE` - Timezone for scheduling (default: Europe/Moscow)
- `LOG_LEVEL` - Logging level (default: info)

## Installation

### From Source

1. Clone the repository:
   ```bash
   git clone https://github.com/username/invest-manager.git
   cd invest-manager
   ```

2. Update Go dependencies:
   ```bash
   go mod tidy
   ```

3. Build the application:
   ```bash
   make build
   ```

4. Run:
   ```bash
   make run
   ```

### Using Systemd (Linux)

1. Build the application:
   ```bash
   make build
   ```

2. Edit the systemd service file:
   ```bash
   nano deploy/invest-manager.service
   ```

3. Update environment variables with your tokens and credentials

4. Install as a service:
   ```bash
   make install
   ```

5. Start the service:
   ```bash
   sudo systemctl start invest-manager
   ```

6. Enable autostart on boot:
   ```bash
   sudo systemctl enable invest-manager
   ```

## Usage

Once running, the bot will:

- Automatically analyze your portfolio daily at 7:00 MSK
- Send a detailed report with recommendations to your Telegram
- On the 5th of each month, include a reminder to add funds and rebalance

### Manual Triggers

You can manually trigger analysis with:

```bash
# Run once
make run

# Run with monthly reminder
make run-monthly
```

## Monitoring

View logs with:

```bash
# If installed as a service
make logs

# Or
sudo journalctl -u invest-manager -f
```

## License

MIT 