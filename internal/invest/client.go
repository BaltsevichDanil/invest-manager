package invest

import (
	"context"
	"fmt"
	"invest-manager/internal/config"
	"log"

	"github.com/russianinvestments/invest-api-go-sdk/investgo"
	proto "github.com/russianinvestments/invest-api-go-sdk/proto"
)

// Client wraps Tinkoff Invest API client
type Client struct {
	sdk       *investgo.Client
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
		logger:    logger,
		config:    cfg,
	}, nil
}

// Close closes the client connection
func (c *Client) Close() {
	c.sdk.Stop()
}

// moneyValueToFloat64 converts MoneyValue to float64
func moneyValueToFloat64(mv *proto.MoneyValue) float64 {
	if mv == nil {
		return 0
	}
	return float64(mv.Units) + float64(mv.Nano)/1e9
}

// quotationToFloat64 converts Quotation to float64
func quotationToFloat64(q *proto.Quotation) float64 {
	if q == nil {
		return 0
	}
	return float64(q.Units) + float64(q.Nano)/1e9
}

// GetPortfolio retrieves the current portfolio
func (c *Client) GetPortfolio(ctx context.Context) (*Portfolio, error) {
	accountsClient := c.sdk.NewUsersServiceClient()
	accountsResp, err := accountsClient.GetAccounts(proto.AccountStatus_ACCOUNT_STATUS_UNSPECIFIED.Enum())
	if err != nil {
		return nil, fmt.Errorf("failed to get accounts: %w", err)
	}
	if len(accountsResp.Accounts) == 0 {
		return nil, fmt.Errorf("no accounts found")
	}
	accountId := accountsResp.Accounts[0].Id

	opsClient := c.sdk.NewOperationsServiceClient()
	portfolioResp, err := opsClient.GetPortfolio(accountId, 0) // 0 = RUB
	if err != nil {
		return nil, fmt.Errorf("failed to get portfolio: %w", err)
	}

	positions := make([]Position, 0, len(portfolioResp.Positions))
	var totalAmount, totalYield float64
	currency := "RUB"

	for _, pos := range portfolioResp.Positions {
		qty := quotationToFloat64(pos.Quantity)
		avgPrice := moneyValueToFloat64(pos.AveragePositionPrice)
		curPrice := moneyValueToFloat64(pos.CurrentPrice)
		yield := quotationToFloat64(pos.ExpectedYield)

		positions = append(positions, Position{
			FIGI:           pos.Figi,
			Ticker:         pos.Figi,
			Name:           pos.InstrumentType,
			InstrumentType: pos.InstrumentType,
			Quantity:       qty,
			AveragePrice:   avgPrice,
			CurrentPrice:   curPrice,
			ExpectedYield:  yield,
			Currency:       currency,
		})
		totalAmount += qty * curPrice
		totalYield += yield
	}

	return &Portfolio{
		Positions:     positions,
		TotalAmount:   totalAmount,
		ExpectedYield: totalYield,
		Currency:      currency,
	}, nil
}