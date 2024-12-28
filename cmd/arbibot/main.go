package main

import (
	// "notlelouch/ArbiBot/internal/arbitrage"
	"context"
	"log"
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
	kucoinClient := kucoin.NewKuCoinWS(tokenResp)

	// Connect to exchanges via websockets
	hyperliquidClient.Connect(ctx)
	kucoinClient.Connect(ctx)

	// Subscribe to order book
	hyperliquidClient.SubscribeToOrderBook("SOL")
	kucoinClient.Subscribe("/market/level2:SOL-USDT", false)

	// Run arbitrage logic
	// arbitrage.FindArbitrageOpportunities(exchanges, coin)

	// Keep the program running
	select {}
}
