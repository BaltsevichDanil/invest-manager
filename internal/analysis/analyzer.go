package analysis

import (
	"context"
	"fmt"
	"invest-manager/internal/config"
	"invest-manager/internal/invest"
	"invest-manager/internal/news"
	"strings"

	"github.com/sashabaranov/go-openai"
)

// Recommendation represents an investment recommendation
type Recommendation struct {
	Ticker  string
	Name    string
	Action  string // BUY, SELL, HOLD
	Reason  string
}

// PortfolioAnalysis contains the complete analysis results
type PortfolioAnalysis struct {
	Recommendations []Recommendation
	Summary         string
	IsMonthlyReminder bool
}

// Analyzer handles OpenAI interactions
type Analyzer struct {
	client *openai.Client
	model  string
}

// NewAnalyzer creates a new OpenAI analyzer
func NewAnalyzer(cfg *config.Config) *Analyzer {
	openaiConfig := openai.DefaultConfig(cfg.OpenAIApiKey)
	openaiConfig.BaseURL = cfg.OpenAIBaseURL
	client := openai.NewClientWithConfig(openaiConfig)
	
	return &Analyzer{
		client: client,
		model:  "gpt-4o", // Using GPT-4 as specified
	}
}

// AnalyzePortfolio analyzes portfolio data with news context
func (a *Analyzer) AnalyzePortfolio(ctx context.Context, portfolio *invest.Portfolio, newsArticles []news.Article, isMonthlyReminder bool) (*PortfolioAnalysis, error) {
	// Format the portfolio information
	portfolioInfo := formatPortfolioInfo(portfolio)
	
	// Format the news information
	newsInfo := formatNewsInfo(newsArticles)
	
	// Create the system and user prompts
	systemPrompt := `You are an investment advisor specializing in Russian stocks. 
You will analyze a portfolio and relevant news to provide actionable advice for each position.
For each position, provide a recommendation (BUY/SELL/HOLD) and a brief, easy-to-understand explanation.
Use clear language suitable for non-financial experts ("for beginners").
Format your response as:

SUMMARY:
[Overall portfolio assessment and 1-2 key insights]

RECOMMENDATIONS:
[ticker]: [NAME] - [BUY/SELL/HOLD]
Explanation: [1-2 sentences explaining the recommendation]

[Next position...]

Отвечай на русском языке.
Пожалуйста, используйте заголовки строго на английском языке как "SUMMARY:" и "RECOMMENDATIONS:".`

	userPrompt := fmt.Sprintf("Here is the current portfolio information:\n\n%s\n\nRecent news about Russian stocks:\n\n%s\n\nPlease provide investment recommendations for each position in the portfolio.\n\nОтвечай на русском языке.", portfolioInfo, newsInfo)
	
	// Add monthly reminder if needed
	if isMonthlyReminder {
		userPrompt += "\n\nThis is a monthly review. Please also include a reminder to add funds and redistribute the portfolio."
	}

	// Create the OpenAI API request
	request := openai.ChatCompletionRequest{
		Model: a.model,
		Messages: []openai.ChatCompletionMessage{
			{
				Role:    openai.ChatMessageRoleSystem,
				Content: systemPrompt,
			},
			{
				Role:    openai.ChatMessageRoleUser,
				Content: userPrompt,
			},
		},
		Temperature: 0.3, // Lower temperature for more focused responses
	}
	
	// Make the API call
	response, err := a.client.CreateChatCompletion(ctx, request)
	if err != nil {
		return nil, fmt.Errorf("error calling OpenAI API: %w", err)
	}
	
	if len(response.Choices) == 0 {
		return nil, fmt.Errorf("no response from OpenAI API")
	}
	
	// Parse the response
	analysisText := response.Choices[0].Message.Content
	analysis, err := parseAnalysisResponse(analysisText, portfolio)
	if err != nil {
		return nil, fmt.Errorf("error parsing analysis response: %w", err)
	}
	
	analysis.IsMonthlyReminder = isMonthlyReminder
	
	return analysis, nil
}

// formatPortfolioInfo formats the portfolio into a readable string
func formatPortfolioInfo(portfolio *invest.Portfolio) string {
	var sb strings.Builder
	
	sb.WriteString(fmt.Sprintf("Total portfolio value: %.2f %s\n", portfolio.TotalAmount, portfolio.Currency))
	sb.WriteString(fmt.Sprintf("Expected yield: %.2f %s\n\n", portfolio.ExpectedYield, portfolio.Currency))
	sb.WriteString("Positions:\n")
	
	for _, pos := range portfolio.Positions {
		sb.WriteString(fmt.Sprintf("- %s (%s): %s\n", pos.Ticker, pos.Name, pos.InstrumentType))
		sb.WriteString(fmt.Sprintf("  Quantity: %.2f\n", pos.Quantity))
		sb.WriteString(fmt.Sprintf("  Average Price: %.2f %s\n", pos.AveragePrice, pos.Currency))
		sb.WriteString(fmt.Sprintf("  Current Price: %.2f %s\n", pos.CurrentPrice, pos.Currency))
		sb.WriteString(fmt.Sprintf("  Expected Yield: %.2f %s\n", pos.ExpectedYield, pos.Currency))
		sb.WriteString("\n")
	}
	
	return sb.String()
}

// formatNewsInfo formats news articles into a readable string
func formatNewsInfo(articles []news.Article) string {
	var sb strings.Builder
	
	for i, article := range articles {
		sb.WriteString(fmt.Sprintf("%d. %s\n", i+1, article.Title))
		sb.WriteString(fmt.Sprintf("   Source: %s\n", article.Source.Name))
		sb.WriteString(fmt.Sprintf("   Date: %s\n", article.PublishedAt.Format("2006-01-02")))
		if article.Description != "" {
			sb.WriteString(fmt.Sprintf("   Description: %s\n", article.Description))
		}
		sb.WriteString(fmt.Sprintf("   URL: %s\n\n", article.URL))
	}
	
	return sb.String()
}

// parseAnalysisResponse parses the OpenAI response into structured data
func parseAnalysisResponse(analysisText string, portfolio *invest.Portfolio) (*PortfolioAnalysis, error) {
	// Clean markdown formatting to detect headings reliably
	cleanedText := strings.ReplaceAll(analysisText, "*", "")
	cleanedText = strings.ReplaceAll(cleanedText, "_", "")

	// Initialize the analysis
	analysis := &PortfolioAnalysis{
		Recommendations: []Recommendation{},
	}
	
	// Split into summary and recommendations sections (English then Russian)
	summaryParts := strings.Split(cleanedText, "RECOMMENDATIONS:")
	if len(summaryParts) < 2 {
		summaryParts = strings.Split(cleanedText, "РЕКОМЕНДАЦИИ:")
	}
	if len(summaryParts) < 2 {
		// If no "RECOMMENDATIONS:" section, try to extract summary anyway
		summaryLines := strings.Split(cleanedText, "\n")
		for i, line := range summaryLines {
			if strings.HasPrefix(line, "SUMMARY:") {
				analysis.Summary = strings.TrimSpace(strings.TrimPrefix(line, "SUMMARY:"))
				if i+1 < len(summaryLines) && analysis.Summary == "" {
					analysis.Summary = strings.TrimSpace(summaryLines[i+1])
				}
				break
			}
		}
	} else {
		// Extract summary from the first part
		summaryLines := strings.Split(summaryParts[0], "\n")
		for i, line := range summaryLines {
			if strings.HasPrefix(line, "SUMMARY:") {
				analysis.Summary = strings.TrimSpace(strings.TrimPrefix(line, "SUMMARY:"))
				if i+1 < len(summaryLines) && analysis.Summary == "" {
					analysis.Summary = strings.TrimSpace(summaryLines[i+1])
				}
				break
			}
		}
		
		// Parse the recommendations
		recsText := summaryParts[1]
		recsLines := strings.Split(recsText, "\n")
		
		var currentRec *Recommendation
		
		for _, line := range recsLines {
			line = strings.TrimSpace(line)
			if line == "" {
				continue
			}
			
			// Check if this is a new recommendation
			for _, pos := range portfolio.Positions {
				if strings.HasPrefix(line, pos.Ticker+":") || strings.HasPrefix(line, pos.Ticker+" -") || strings.HasPrefix(line, pos.Ticker+" –") {
					// Complete previous recommendation if exists
					if currentRec != nil && currentRec.Ticker != "" {
						analysis.Recommendations = append(analysis.Recommendations, *currentRec)
					}
					
					// Start new recommendation
					action := ""
					if strings.Contains(line, "BUY") {
						action = "BUY"
					} else if strings.Contains(line, "SELL") {
						action = "SELL"
					} else if strings.Contains(line, "HOLD") {
						action = "HOLD"
					}
					
					currentRec = &Recommendation{
						Ticker: pos.Ticker,
						Name:   pos.Name,
						Action: action,
					}
					break
				}
			}
			
			// If we have a current recommendation and this line is the explanation
			if currentRec != nil && strings.HasPrefix(line, "Explanation:") {
				currentRec.Reason = strings.TrimSpace(strings.TrimPrefix(line, "Explanation:"))
			} else if currentRec != nil && currentRec.Reason == "" && !strings.Contains(line, currentRec.Ticker) {
				// This might be the explanation without the "Explanation:" prefix
				currentRec.Reason = line
			}
		}
		
		// Add the last recommendation
		if currentRec != nil && currentRec.Ticker != "" {
			analysis.Recommendations = append(analysis.Recommendations, *currentRec)
		}
	}
	
	// If no recommendations were found but there is a response, create a fallback
	if len(analysis.Recommendations) == 0 && strings.TrimSpace(cleanedText) != "" {
		analysis.Summary = "Analysis completed, but could not parse specific recommendations."
		
		// Create generic recommendations based on portfolio
		for _, pos := range portfolio.Positions {
			action := "HOLD" // Default to HOLD
			
			// Simple heuristic: positive yield suggests BUY, negative suggests SELL
			if pos.ExpectedYield > 0 {
				action = "BUY"
			} else if pos.ExpectedYield < 0 {
				action = "SELL"
			}
			
			analysis.Recommendations = append(analysis.Recommendations, Recommendation{
				Ticker: pos.Ticker,
				Name:   pos.Name,
				Action: action,
				Reason: "Based on current position yield.",
			})
		}
	}
	
	return analysis, nil
} 