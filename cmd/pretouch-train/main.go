// Command pretouch-train trains the ETH pretouch timing model (DT3 + RF)
// from the canonical events CSV and outputs data/pretouch_model.json.
package main

import (
	"fmt"
	"os"
	"time"

	"github.com/wuyaocheng/bktrader/internal/service"
)

func main() {
	config := service.DefaultPretouchTrainerConfig()
	fmt.Printf("=== Pretouch Model Training ===\n")
	fmt.Printf("Events CSV: %s\n", config.EventsCSVPath)
	fmt.Printf("Model output: %s\n", config.ModelOutPath)
	fmt.Printf("Forward start: %s\n", config.ForwardStart)
	fmt.Printf("DT max depth: %d\n", config.MaxDepthDT)
	fmt.Printf("RF estimators: %d\n", config.NEstimatorsRF)
	fmt.Println()

	start := time.Now()
	if err := service.TrainPretouchModel(config); err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: %v\n", err)
		os.Exit(1)
	}

	elapsed := time.Since(start)
	fmt.Printf("\nModel trained successfully in %s\n", elapsed)
	fmt.Printf("Output: %s\n", config.ModelOutPath)
}
