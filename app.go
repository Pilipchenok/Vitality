package main

import (
	"context"
	"sync"
	"time"
	"log"

	"github.com/shirou/gopsutil/v3/cpu"
	"github.com/shirou/gopsutil/v3/mem"
	"github.com/shirou/gopsutil/v3/disk"
	"github.com/shirou/gopsutil/v3/host"
)

type SliceMetrics struct {
	CheckTime time.Time `json:"check_time"`
	Metrics []Metric `json:"metrics"`
}

type Metric struct {
	Name string `json:"name"`
	Value float64 `json:"value"`
}

type App struct {
	ctx context.Context
	metrics []SliceMetrics
	mu sync.Mutex
}

func NewApp() *App {
	return &App{}
}

func (a *App) startup(ctx context.Context) {
	a.ctx = ctx

	go func() {
    ticker := time.NewTicker(2 * time.Second)
    defer ticker.Stop()

    for {
      select {
      case <-ctx.Done():
        return 
      case <-ticker.C:
        a.CollectMetrics()
      }
    }
  }()
}

func (a *App) CollectMetrics() {
	var collected SliceMetrics
	collected.CheckTime = time.Now()

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
	temp := getCPUTemp()
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

func (a *App) GetHistory() []SliceMetrics {
	a.mu.Lock()
	defer a.mu.Unlock()
	result := make([]SliceMetrics, len(a.metrics))
	copy(result, a.metrics)

	return result
}

func getCPUTemp() float64 {
	temps, err := host.SensorsTemperatures()
	if err == nil && len(temps) > 0 {
		for _, t := range temps {
			if t.Temperature > 0 {
				return t.Temperature
			}
		}
	}
	return 45.0 
}
