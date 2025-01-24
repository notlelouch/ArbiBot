package main

import (
	"context"
	"fmt"
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

	"github.com/mum4k/termdash"
	"github.com/mum4k/termdash/container"
	"github.com/mum4k/termdash/container/grid"
	"github.com/mum4k/termdash/linestyle"
	"github.com/mum4k/termdash/terminal/tcell"
	"github.com/mum4k/termdash/terminal/terminalapi"
	"github.com/mum4k/termdash/widgets/text"
)

const (
	redrawInterval = 250 * time.Millisecond
)

func main() {
	// Define the list of coins to monitor
	coins := []string{
		"SOL",
		"WIF",
		"DOGE",
		"JUP",
		"GMT",
	}

	// Create a context that can be canceled
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Handle graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigChan
		log.Println("Shutting down gracefully...")
		cancel()
	}()

	// Initialize terminal
	t, err := tcell.New(tcell.ColorMode(terminalapi.ColorMode256))
	if err != nil {
		log.Fatalf("Failed to initialize terminal: %v", err)
	}
	defer t.Close()

	// Create a map to hold text widgets for each coin
	coinWidgets := make(map[string]*text.Text)
	for _, coin := range coins {
		widget, err := text.New(text.RollContent(), text.WrapAtWords())
		if err != nil {
			log.Fatalf("Failed to create text widget for %s: %v", coin, err)
		}
		coinWidgets[coin] = widget
	}

	// Create a grid layout with one cell per coin
	builder := grid.New()
	for _, coin := range coins {
		builder.Add(
			grid.RowHeightPerc(20, // Each row takes 20% of the screen height
				grid.Widget(
					coinWidgets[coin],
					container.Border(linestyle.Light),
					container.BorderTitle(fmt.Sprintf(" %s Arbitrage Opportunities ", coin)),
				),
			),
		)
	}

	// Build the grid layout
	gridOpts, err := builder.Build()
	if err != nil {
		log.Fatalf("Failed to build grid layout: %v", err)
	}

	// Create the root container with the grid layout
	c, err := container.New(t, gridOpts...)
	if err != nil {
		log.Fatalf("Failed to create root container: %v", err)
	}

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

	// Wait for order books to be populated
	log.Println("Waiting for order book updates...")
	time.Sleep(2000 * time.Millisecond)

	// Start updating arbitrage data for each coin
	var wg sync.WaitGroup
	for _, coin := range coins {
		wg.Add(1)
		go func(symbol string) {
			defer wg.Done()

			// Subscribe to order books
			if err := hyperliquidClient.SubscribeToOrderBook(symbol); err != nil {
				log.Printf("Failed to subscribe to Hyperliquid order book for %s: %v\n", symbol, err)
				return
			}
			if err := kucoinClient.SubscribeToOrderBook(symbol); err != nil {
				log.Printf("Failed to subscribe to KuCoin order book for %s: %v\n", symbol, err)
				return
			}

			// Run arbitrage logic continuously in a goroutine
			ticker := time.NewTicker(1 * time.Second) // Update every second
			defer ticker.Stop()

			for {
				select {
				case <-ticker.C:
					exchanges := []exchange.Exchange{hyperliquidClient, kucoinClient}
					lowestAsk, highestBid, err := arbitrage.FindBestPrices(exchanges, symbol)
					if err != nil {
						log.Printf("Error finding best prices for %s: %v\n", symbol, err)
						continue
					}

					// Check for arbitrage opportunity
					if highestBid.Price > lowestAsk.Price {
						coinWidgets[symbol].Reset()
						coinWidgets[symbol].Write(fmt.Sprintf(`
Buy Exchange: %s
Buy Price:    $%.8f
Sell Exchange: %s
Sell Price:    $%.8f
Profit:        $%.4f
Time:          %s
`,
							lowestAsk.Exchange, lowestAsk.Price,
							highestBid.Exchange, highestBid.Price,
							highestBid.Price-lowestAsk.Price,
							time.Now().Format("15:04:05"),
						))
					}
				case <-ctx.Done():
					return
				}
			}
		}(coin)
	}

	// Run the terminal dashboard
	if err := termdash.Run(ctx, t, c, termdash.RedrawInterval(redrawInterval)); err != nil {
		log.Fatalf("Failed to run termdash: %v", err)
	}

	// Keep the program running until the context is canceled
	<-ctx.Done()
	log.Println("Program exited gracefully.")
}
