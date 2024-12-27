package main

import (
	// "notlelouch/ArbiBot/internal/arbitrage"
	"context"
	"log"
	"notlelouch/ArbiBot/internal/exchange"
	"notlelouch/ArbiBot/internal/exchange/hyperliquid"
	"notlelouch/ArbiBot/internal/exchange/kucoin"
	// Import other exchanges
)

func main() {
	ctx := context.Background()

	// Get public token
	tokenResp, err := kucoin.GetToken("", "", "", false)
	if err != nil {
		log.Fatalf("Failed to get public token: %v", err)
	}

	// Initialize exchanges
	hyperliquidClient := hyperliquid.NewHyperliquidWS(true)
	// Initialize other exchanges

	// Initialize KuCoin WebSocket client
	kucoinClient := kucoin.NewKuCoinWS(tokenResp)

	exchanges := []exchange.Exchange{hyperliquidClient /*, other exchanges */}

	// Connect to exchanges
	for _, ex := range exchanges {
		if err := ex.Connect(ctx); err != nil {
			log.Fatalf("Failed to connect to exchange: %v", err)
		}
	}

	// Subscribe to order books
	coin := "SOL"
	for _, ex := range exchanges {
		if err := ex.SubscribeToOrderBook(coin); err != nil {
			log.Fatalf("Failed to subscribe to order book: %v", err)
		}
	}

	// Connect to WebSocket
	if err := kucoinClient.Connect(ctx); err != nil {
		log.Fatalf("Failed to connect to KuCoin WebSocket: %v", err)
	}
	// Subscribe to public order book
	if err := kucoinClient.Subscribe("/market/level2:BTC-USDT", false); err != nil {
		log.Fatalf("Failed to subscribe to KuCoin order book: %v", err)
	}

	// Run arbitrage logic
	// arbitrage.FindArbitrageOpportunities(exchanges, coin)

	// Keep the program running
	select {}
}
