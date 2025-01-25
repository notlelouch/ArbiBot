package ui

import (
	"context"
	"fmt"
	"math"
	"sync"
	"time"

	"github.com/mum4k/termdash"
	"github.com/mum4k/termdash/cell"
	"github.com/mum4k/termdash/container"
	"github.com/mum4k/termdash/container/grid"
	"github.com/mum4k/termdash/linestyle"
	"github.com/mum4k/termdash/terminal/tcell"
	"github.com/mum4k/termdash/terminal/terminalapi"
	"github.com/mum4k/termdash/widgets/barchart"
	"github.com/mum4k/termdash/widgets/button"
	"github.com/mum4k/termdash/widgets/linechart"
	"github.com/mum4k/termdash/widgets/text"
)

const (
	redrawInterval = 250 * time.Millisecond
	maxHistorySize = 50
)

type CoinData struct {
	Timestamp    time.Time
	Symbol       string
	BuyExchange  string
	SellExchange string
	BuyPrice     float64
	SellPrice    float64
	Profit       float64
	Spread       float64
}

// chartMode represents the current view mode of the line chart
type chartMode int

const (
	modeAll chartMode = iota
	modeSingle
)

type ArbitrageDashboard struct {
	coinWidgets    map[string]*text.Text
	barChart       *barchart.BarChart
	lineChart      *linechart.LineChart
	lineChartBtn   map[string]*button.Button
	updateChan     chan CoinData
	closeChan      chan struct{}
	spreadsHistory map[string][]float64
	profits        map[string]float64
	coins          []string
	chartColors    []cell.Color
	selectedCoin   string
	mode           chartMode
	mu             sync.RWMutex
}

func NewArbitrageDashboard(coins []string) *ArbitrageDashboard {
	return &ArbitrageDashboard{
		coins: coins,
		chartColors: []cell.Color{
			cell.ColorGreen,
			cell.ColorBlue,
			cell.ColorCyan,
			cell.ColorMagenta,
			cell.ColorYellow,
		},
		coinWidgets:    make(map[string]*text.Text),
		lineChartBtn:   make(map[string]*button.Button),
		updateChan:     make(chan CoinData, 100),
		closeChan:      make(chan struct{}),
		spreadsHistory: make(map[string][]float64),
		profits:        make(map[string]float64),
		mode:           modeAll,
	}
}

func (ad *ArbitrageDashboard) InitWidgets() error {
	// Initialize coin widgets
	for _, coin := range ad.coins {
		widget, err := text.New(text.RollContent(), text.WrapAtWords())
		if err != nil {
			return fmt.Errorf("failed to create text widget for %s: %v", coin, err)
		}
		ad.coinWidgets[coin] = widget
	}

	// Initialize bar chart
	barChart, err := barchart.New(
		barchart.BarColors(ad.chartColors),
		barchart.ShowValues(),
		barchart.Labels(ad.coins),
	)
	if err != nil {
		return fmt.Errorf("failed to create bar chart: %v", err)
	}
	ad.barChart = barChart

	// Initialize line chart with proper options
	lineChart, err := linechart.New(
		linechart.AxesCellOpts(cell.FgColor(cell.ColorRed)),
		linechart.YLabelCellOpts(cell.FgColor(cell.ColorGreen)),
		linechart.XLabelCellOpts(cell.FgColor(cell.ColorGreen)),
	)
	if err != nil {
		return fmt.Errorf("failed to create line chart: %v", err)
	}
	ad.lineChart = lineChart

	// Initialize chart control buttons
	if err := ad.initLineChartButtons(); err != nil {
		return fmt.Errorf("failed to create line chart buttons: %v", err)
	}

	return nil
}

func (ad *ArbitrageDashboard) initLineChartButtons() error {
	// "All Coins" button
	allButton, err := button.New("All Coins", func() error {
		ad.mu.Lock()
		ad.mode = modeAll
		ad.mu.Unlock()
		return nil
	},
		button.WidthFor("All Coins"),
		button.Height(1),
		button.FillColor(cell.ColorNumber(220)),
	)
	if err != nil {
		return fmt.Errorf("failed to create All button: %v", err)
	}
	ad.lineChartBtn["All"] = allButton

	// Individual coin buttons
	for _, coin := range ad.coins {
		coinCopy := coin
		btn, err := button.New(coinCopy, func() error {
			ad.mu.Lock()
			ad.mode = modeSingle
			ad.selectedCoin = coinCopy
			ad.mu.Unlock()
			return nil
		},
			button.WidthFor(coinCopy),
			button.Height(1),
			button.FillColor(cell.ColorNumber(196)),
		)
		if err != nil {
			return fmt.Errorf("failed to create button for %s: %v", coin, err)
		}
		ad.lineChartBtn[coinCopy] = btn
	}

	return nil
}

func (ad *ArbitrageDashboard) processCoinUpdate(coinData CoinData) {
	ad.mu.Lock()
	defer ad.mu.Unlock()

	// Update coin widget
	widget, exists := ad.coinWidgets[coinData.Symbol]
	if exists {
		widget.Reset()
		widget.Write(fmt.Sprintf(`
Buy Exchange: %s
Buy Price:    $%.7f
Sell Exchange: %s
Sell Price:    $%.7f
Profit:        %.7f%%
Spread:        %.7f%%
Time:          %s
`,
			coinData.BuyExchange, coinData.BuyPrice,
			coinData.SellExchange, coinData.SellPrice,
			coinData.Profit, coinData.Spread,
			coinData.Timestamp.Format("15:04:05"),
		))
	}

	// Update profits
	ad.profits[coinData.Symbol] = coinData.Profit

	// Update spread history
	if len(ad.spreadsHistory[coinData.Symbol]) >= maxHistorySize {
		ad.spreadsHistory[coinData.Symbol] = ad.spreadsHistory[coinData.Symbol][1:]
	}
	ad.spreadsHistory[coinData.Symbol] = append(ad.spreadsHistory[coinData.Symbol], coinData.Spread)

	// Update bar chart
	barData := make([]int, len(ad.coins))
	for i, coin := range ad.coins {
		barData[i] = int(math.Abs(ad.profits[coin]) * 20000)
	}
	ad.barChart.Values(barData, 1000)

	// Update line chart
	ad.updateLineChart()
}

func (ad *ArbitrageDashboard) updateLineChart() {
	// Clear existing series
	ad.lineChart.Series("", []float64{}, linechart.SeriesCellOpts(cell.FgColor(cell.ColorDefault)))

	// Optionally clear again with empty series for each coin to ensure full cleanup
	for _, coin := range ad.coins {
		ad.lineChart.Series(coin, []float64{}, linechart.SeriesCellOpts(cell.FgColor(cell.ColorDefault)))
	}

	switch ad.mode {
	case modeAll:
		for i, coin := range ad.coins {
			if spreads, ok := ad.spreadsHistory[coin]; ok && len(spreads) > 0 {
				ad.lineChart.Series(coin,
					spreads,
					linechart.SeriesCellOpts(cell.FgColor(ad.chartColors[i%len(ad.chartColors)])),
				)
			}
		}
	case modeSingle:
		if spreads, ok := ad.spreadsHistory[ad.selectedCoin]; ok && len(spreads) > 0 {
			colorIdx := 0
			for i, coin := range ad.coins {
				if coin == ad.selectedCoin {
					colorIdx = i
					break
				}
			}
			ad.lineChart.Series(
				ad.selectedCoin,
				spreads,
				linechart.SeriesCellOpts(cell.FgColor(ad.chartColors[colorIdx%len(ad.chartColors)])),
			)
		}
	}
}

func CreateGridLayout(ad *ArbitrageDashboard) ([]container.Option, error) {
	builder := grid.New()

	// Create button elements for line chart
	var buttonElements []grid.Element
	buttonElements = append(buttonElements,
		grid.ColWidthPerc(20,
			grid.Widget(ad.lineChartBtn["All"],
				container.Border(linestyle.Light),
			),
		),
	)
	for _, coin := range ad.coins {
		buttonElements = append(buttonElements,
			grid.ColWidthPerc(16,
				grid.Widget(ad.lineChartBtn[coin],
					container.Border(linestyle.Light),
				),
			),
		)
	}

	builder.Add(
		grid.RowHeightPerc(25,
			createCoinWidgetsRow(ad.coinWidgets, ad.coins)...,
		),
		grid.RowHeightPerc(65,
			grid.ColWidthPerc(50,
				grid.Widget(ad.barChart,
					container.Border(linestyle.Light),
					container.BorderTitle(" Arbitrage Profits "),
				),
			),
			grid.ColWidthPerc(50,
				grid.RowHeightPerc(15,
					buttonElements...,
				),
				grid.RowHeightPerc(85,
					grid.Widget(ad.lineChart,
						container.Border(linestyle.Light),
						container.BorderTitle(" Arbitrage Spread History "),
					),
				),
			),
		),
	)

	gridOpts, err := builder.Build()
	if err != nil {
		return nil, err
	}
	return gridOpts, nil
}

func createCoinWidgetsRow(coinWidgets map[string]*text.Text, coins []string) []grid.Element {
	var elements []grid.Element
	for _, coin := range coins {
		elements = append(elements,
			grid.ColWidthPerc(20,
				grid.Widget(coinWidgets[coin],
					container.Border(linestyle.Light),
					container.BorderTitle(fmt.Sprintf(" %s Arbitrage ", coin)),
				),
			),
		)
	}
	return elements
}

func (ad *ArbitrageDashboard) StartUpdateListener(ctx context.Context) {
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case coinData := <-ad.updateChan:
				ad.processCoinUpdate(coinData)
			}
		}
	}()
}

func RunDashboard(ctx context.Context, ad *ArbitrageDashboard) error {
	t, err := tcell.New(tcell.ColorMode(terminalapi.ColorMode256))
	if err != nil {
		return fmt.Errorf("failed to initialize terminal: %v", err)
	}
	defer t.Close()

	gridOpts, err := CreateGridLayout(ad)
	if err != nil {
		return fmt.Errorf("failed to build grid layout: %v", err)
	}

	c, err := container.New(t, gridOpts...)
	if err != nil {
		return fmt.Errorf("failed to create root container: %v", err)
	}

	return termdash.Run(ctx, t, c, termdash.RedrawInterval(redrawInterval))
}

func (ad *ArbitrageDashboard) SendCoinData(data CoinData) {
	ad.updateChan <- data
}
