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
	Opportunities   []Recommendation // Added opportunities field
	Summary         string
	IsMonthlyReminder bool
	RawText         string // store original AI response
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
Additionally, suggest a few trading opportunities: stocks not currently in the portfolio that present attractive long or short positions (LONG/SHORT), with a brief explanation.
Use clear language suitable for non-financial experts ("for beginners").
Format your response as:

SUMMARY:
[Overall portfolio assessment and 1-2 key insights]

RECOMMENDATIONS:
[ticker]: [NAME] - [BUY/SELL/HOLD]
Explanation: [1-2 sentences explaining the recommendation]

OPPORTUNITIES:
[ticker]: [NAME] - [LONG/SHORT]
Explanation: [1-2 sentences explaining the opportunity]

Отвечай на русском языке.
Пожалуйста, используйте заголовки строго на английском языке как "SUMMARY:", "RECOMMENDATIONS:", and "OPPORTUNITIES:".`

	userPrompt := fmt.Sprintf("Here is the current portfolio information:\n\n%s\n\nRecent news about Russia:\n\n%s\n\nPlease provide investment recommendations for each position in the portfolio, and suggest trading opportunities (LONG/SHORT) for other relevant stocks.\n\nОтвечай на русском языке.", portfolioInfo, newsInfo)
	
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
	
	// Store raw AI response for fallback display
	analysis.RawText = analysisText
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
		Opportunities:   []Recommendation{},
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
		
		// Parse the recommendations: lines starting with a ticker
		recsText := summaryParts[1]
		recsLines := strings.Split(recsText, "\n")
		var currentRec *Recommendation
		for i := 0; i < len(recsLines); i++ {
			line := strings.TrimSpace(recsLines[i])
			if line == "" {
				continue
			}
			// Detect new recommendation by ticker prefix
			for _, pos := range portfolio.Positions {
				if strings.HasPrefix(line, pos.Ticker) {
					// Append previous rec
					if currentRec != nil {
						analysis.Recommendations = append(analysis.Recommendations, *currentRec)
					}
					// Split into parts on first hyphen
					parts := strings.SplitN(line, "-", 2)
					action := ""
					if len(parts) > 1 {
						p := strings.ToUpper(parts[1])
						if strings.Contains(p, "BUY") {
							action = "BUY"
						} else if strings.Contains(p, "SELL") {
							action = "SELL"
						} else if strings.Contains(p, "HOLD") {
							action = "HOLD"
						}
					}
					// Start new recommendation
					currentRec = &Recommendation{
						Ticker: pos.Ticker,
						Name:   pos.Name,
						Action: action,
					}
					// Next non-empty line is the explanation
					if i+1 < len(recsLines) {
						next := strings.TrimSpace(recsLines[i+1])
						if next != "" && !strings.HasPrefix(next, pos.Ticker) {
							currentRec.Reason = next
							i++ // skip explanation line
						}
					}
					break
				}
			}
		}
		// Append last recommendation
		if currentRec != nil {
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
	
	// Parse opportunities section if present
	if strings.Contains(cleanedText, "OPPORTUNITIES:") {
		oppParts := strings.SplitN(cleanedText, "OPPORTUNITIES:", 2)
		oppText := oppParts[1]
		oppLines := strings.Split(oppText, "\n")
		for i := 0; i < len(oppLines); i++ {
			line := strings.TrimSpace(oppLines[i])
			if line == "" {
				continue
			}
			if strings.Contains(line, "-") {
				parts := strings.SplitN(line, "-", 2)
				left := strings.TrimSpace(parts[0])
				action := strings.TrimSpace(parts[1])
				tn := strings.SplitN(left, ":", 2)
				ticker := strings.TrimSpace(tn[0])
				name := ""
				if len(tn) > 1 {
					name = strings.TrimSpace(tn[1])
				}
				opp := Recommendation{
					Ticker: ticker,
					Name:   name,
					Action: action,
				}
				if i+1 < len(oppLines) {
					next := strings.TrimSpace(oppLines[i+1])
					if strings.HasPrefix(next, "Explanation:") {
						opp.Reason = strings.TrimSpace(strings.TrimPrefix(next, "Explanation:"))
						i++
					}
				}
				analysis.Opportunities = append(analysis.Opportunities, opp)
			}
		}
	}
	
	return analysis, nil
} 