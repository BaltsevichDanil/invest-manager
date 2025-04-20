package invest

import (
	"context"
	"fmt"
	"invest-manager/internal/config"
	"log"

	"github.com/russianinvestments/invest-api-go-sdk/investgo"
)

// Client wraps Tinkoff Invest API client
type Client struct {
	sdk       *investgo.Client
	accountID string
	logger    *log.Logger
	config    *config.Config
}

// Position represents a position in portfolio
type Position struct {
	FIGI           string
	Ticker         string
	Name           string
	InstrumentType string
	Quantity       float64
	AveragePrice   float64
	CurrentPrice   float64
	ExpectedYield  float64
	Currency       string
}

// Portfolio contains all positions and total values
type Portfolio struct {
	Positions     []Position
	TotalAmount   float64
	ExpectedYield float64
	Currency      string
}

// NewClient creates a new Tinkoff Invest API client
func NewClient(cfg *config.Config, logger *log.Logger) (*Client, error) {
	// Set up connection config
	sdkConfig := investgo.Config{
		Token:     cfg.TinkoffToken,
		AppName:   "invest-manager-bot",
	}

	// Set endpoint if provided
	if cfg.TinkoffEndpoint != "" {
		sdkConfig.EndPoint = cfg.TinkoffEndpoint
	}

	// Initialize SDK client
	client, err := investgo.NewClient(context.Background(), sdkConfig, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize Tinkoff Invest client: %w", err)
	}

	return &Client{
		sdk:       client,
		accountID: cfg.TinkoffAccountID,
		logger:    logger,
		config:    cfg,
	}, nil
}

// Close closes the client connection
func (c *Client) Close() {
	c.sdk.Stop()
}

// GetPortfolio retrieves the current portfolio
func (c *Client) GetPortfolio(ctx context.Context) (*Portfolio, error) {
	// For demo/testing - create a dummy portfolio
	dummyPositions := []Position{
		{
			FIGI:           "BBG004730N88",
			Ticker:         "SBER",
			Name:           "Сбербанк",
			InstrumentType: "Акция",
			Quantity:       10,
			AveragePrice:   270.5,
			CurrentPrice:   285.3,
			ExpectedYield:  148.0,
			Currency:       "RUB",
		},
		{
			FIGI:           "BBG004S68CP5",
			Ticker:         "LKOH",
			Name:           "Лукойл",
			InstrumentType: "Акция",
			Quantity:       5,
			AveragePrice:   6500.0,
			CurrentPrice:   6700.0,
			ExpectedYield:  1000.0,
			Currency:       "RUB",
		},
		{
			FIGI:           "BBG004S68879",
			Ticker:         "GAZP",
			Name:           "Газпром",
			InstrumentType: "Акция",
			Quantity:       20,
			AveragePrice:   165.0,
			CurrentPrice:   160.0,
			ExpectedYield:  -100.0,
			Currency:       "RUB",
		},
	}

	// Calculate totals
	var totalAmount, totalYield float64
	for _, pos := range dummyPositions {
		totalAmount += pos.Quantity * pos.CurrentPrice
		totalYield += pos.ExpectedYield
	}

	return &Portfolio{
		Positions:     dummyPositions,
		TotalAmount:   totalAmount,
		ExpectedYield: totalYield,
		Currency:      "RUB",
	}, nil
}

// getInstrumentType converts API instrument type to readable string
func getInstrumentType(instrumentType string) string {
	switch instrumentType {
	case "share":
		return "Акция"
	case "bond":
		return "Облигация"
	case "etf":
		return "ETF"
	case "currency":
		return "Валюта"
	default:
		return instrumentType
	}
} 