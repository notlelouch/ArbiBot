package main

import (
	"context"
	"log"
	"notlelouch/ArbiBot/internal/arbitrage"
	"notlelouch/ArbiBot/internal/exchange"
	"notlelouch/ArbiBot/internal/exchange/hyperliquid"
	"notlelouch/ArbiBot/internal/exchange/kucoin"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"
)

func main() {
	coins := []string{
		"JUP",  // Jupiter
		"JTO",  // Jito
		"WIF",  // dogwifhat
		"BONK", // Bonk
		"MYRO", // Myro
		"ORCA", // Orca
		"RAY",  // Raydium
		"HNT",  // Helium
		"RNDR", // Render
		"GMT",  // Stepn
	}

	// Create a context that can be canceled
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Handle graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM) // Listen for SIGINT (Ctrl+C) and SIGTERM
	go func() {
		<-sigChan // Block until a signal is received
		log.Println("Shutting down gracefully...")
		cancel() // Cancel the context to signal all goroutines to stop
	}()

	// Get public token for KuCoin
	tokenResp, err := kucoin.GetToken("", "", "", false)
	if err != nil {
		log.Fatalf("Failed to get public token: %v", err)
	}

	// Initialize exchange clients
	hyperliquidClient := hyperliquid.NewHyperliquidWS(true)
	kucoinClient := kucoin.NewKuCoinWS(tokenResp)

	// Connect to exchanges
	if err := hyperliquidClient.Connect(ctx); err != nil {
		log.Fatalf("Failed to connect to Hyperliquid: %v", err)
	}
	if err := kucoinClient.Connect(ctx); err != nil {
		log.Fatalf("Failed to connect to KuCoin: %v", err)
	}

	var wg sync.WaitGroup
	for _, coin := range coins {
		wg.Add(1)
		go func() {
			// Subscribe to order books
			if err := hyperliquidClient.SubscribeToOrderBook(coin); err != nil {
				log.Fatalf("Failed to subscribe to Hyperliquid order book: %v", err)
			}
			if err := kucoinClient.SubscribeToOrderBook(coin); err != nil {
				log.Fatalf("Failed to subscribe to KuCoin order book: %v", err)
			}

			// Wait for order books to be populated
			log.Println("Waiting for order book updates...")
			time.Sleep(2 * time.Millisecond) // Initial delay to populate order books
		}()

		// Run arbitrage logic continuously in a goroutine
		go func() {
			exchanges := []exchange.Exchange{hyperliquidClient, kucoinClient}
			ticker := time.NewTicker(1 * time.Millisecond) // Check for arbitrage every 2 seconds
			defer ticker.Stop()

			for {
				select {
				case <-ctx.Done(): // Check if the context is canceled
					log.Println("Exiting arbitrage loop...")
					return
				case <-ticker.C:
					lowestAsk, highestBid, err := arbitrage.FindBestPrices(exchanges, coin)
					if err != nil {
						log.Printf("Error finding best prices: %v", err)
						continue
					}

					log.Printf("Lowest Ask: %+v", lowestAsk)
					log.Printf("Highest Bid: %+v", highestBid)

					// Check for arbitrage opportunity
					if highestBid.Price > lowestAsk.Price {
						log.Printf("Arbitrage Opportunity: Buy %s at %f (Exchange: %s), Sell %s at %f (Exchange: %s)",
							coin, lowestAsk.Price, lowestAsk.Exchange, coin, highestBid.Price, highestBid.Exchange)
					}
				}
			}
		}()

	}
	// Keep the program running until the context is canceled
	<-ctx.Done()
	log.Println("Program exited gracefully.")
}
