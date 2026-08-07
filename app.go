package main

import (
	"context"
	"fmt"
	//"runtime/metrics"
	"sync"
	"time"
	"log"

	"github.com/shirou/gopsutil/v3/cpu"
	"github.com/shirou/gopsutil/v3/mem"
	"github.com/shirou/gopsutil/v3/disk"
	"github.com/northwindlight/cputemp"
)

type SliceMetrics struct {
	CheckTime time.Time `json:"check_time"`
	Metrics []Metric `json:"metrics"`
}

type Metric struct {
	Name string `json:"name"`
	Value float64 `json:"value"`
}

// App struct
type App struct {
	ctx context.Context
	metrics []SliceMetrics
	mu sync.Mutex
}

// NewApp creates a new App application struct
func NewApp() *App {
	return &App{}
}

// startup is called when the app starts. The context is saved
// so we can call the runtime methods
func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
}

// Greet returns a greeting for the given name
func (a *App) Greet(name string) string {
	return fmt.Sprintf("Hello %s, It's show time!", name)
}

func (a *App) CollectMetrics() {
	var collected SliceMetrics

	vMem, err := mem.VirtualMemory()
	if err != nil {
		log.Printf("RAM scanning error: %v", err)
	} else {
		collected.Metrics = append(collected.Metrics, Metric{Name: "RAM", Value: vMem.UsedPercent})
	}
	cpuPercents, err := cpu.Percent(0, false)
	if err != nil || len(cpuPercents) == 0 {
		log.Printf("CPU scanning error: %v", err)
	} else {
		collected.Metrics = append(collected.Metrics, Metric{Name: "CPU", Value: cpuPercents[0]})
	}
	usage, err := disk.Usage("/")
	if err != nil {
		log.Printf("Disk scanning error: %v", err)
	} else {
		collected.Metrics = append(collected.Metrics, Metric{Name: "DISK", Value: usage.UsedPercent})
	}
	temp, err := cputemp.GetCPUTemperature()
	if err != nil {
		log.Printf("Temperature scanning error: %v", err)
	} else {
		collected.Metrics = append(collected.Metrics, Metric{Name: "TEMP", Value: temp})
	}

	a.mu.Lock()
	a.metrics = append(a.metrics, collected)
	if len(a.metrics) > 100 {
		a.metrics = a.metrics[1:]
	}
	a.mu.Unlock()
}


