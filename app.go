package main

import (
	"context"
	"fmt"
	"os"
	"runtime"
	"sync"
	"time"

	"github.com/shirou/gopsutil/v3/cpu"
	"github.com/shirou/gopsutil/v3/disk"
	"github.com/shirou/gopsutil/v3/host"
	"github.com/shirou/gopsutil/v3/mem"

	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"
)

type SliceMetrics struct {
	CheckTime time.Time `json:"check_time"`
	Metrics   []Metric  `json:"metrics"`
}

type Metric struct {
	Name  string  `json:"name"`
	Value float64 `json:"value"`
}

type SystemInfo struct {
	Hostname string `json:"hostname"`
	CPUCores int    `json:"cpu_cores"`
	RAMTotal string `json:"ram_total"`
	OS       string `json:"os"`
}

type App struct {
	appInstance   *application.App
	ctx           context.Context
	widgetWin     *application.WebviewWindow
	dashboardWin  *application.WebviewWindow
	systray       *application.SystemTray
	metrics       []SliceMetrics
	mu            sync.Mutex
	widgetVisible bool
}

func NewApp() *App {
	return &App{}
}

func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
	a.CollectMetrics()

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

func (a *App) GetSystemInfo() SystemInfo {
	hostname, _ := os.Hostname()
	if hostname == "" {
		hostname = "Local Mac"
	}
	cores, _ := cpu.Counts(true)
	v, _ := mem.VirtualMemory()
	var ramGb string
	if v != nil {
		ramGb = fmt.Sprintf("%.1f GB", float64(v.Total)/(1024*1024*1024))
	}
	return SystemInfo{
		Hostname: hostname,
		CPUCores: cores,
		RAMTotal: ramGb,
		OS:       runtime.GOOS,
	}
}

func (a *App) CollectMetrics() {
	var collected SliceMetrics
	collected.CheckTime = time.Now()

	vMem, err := mem.VirtualMemory()
	if err == nil {
		collected.Metrics = append(collected.Metrics, Metric{Name: "RAM", Value: vMem.UsedPercent})
	}

	cpuPercents, err := cpu.Percent(0, false)
	if err == nil && len(cpuPercents) > 0 {
		collected.Metrics = append(collected.Metrics, Metric{Name: "CPU", Value: cpuPercents[0]})
	}

	rootPath := "/"
	if runtime.GOOS == "windows" {
		rootPath = "C:\\"
	}
	usage, err := disk.Usage(rootPath)
	if err == nil {
		collected.Metrics = append(collected.Metrics, Metric{Name: "DISK", Value: usage.UsedPercent})
	}

	temp := getCPUTemp()
	collected.Metrics = append(collected.Metrics, Metric{Name: "TEMP", Value: temp})

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

func (a *App) ToggleWidget() {
	a.mu.Lock()
	widget := a.widgetWin
	dash := a.dashboardWin
	isVisible := a.widgetVisible
	
	a.widgetVisible = !isVisible
	a.mu.Unlock()

	if widget == nil {
		return
	}

	if dash != nil {
		dash.Hide()
	}

	if isVisible {
		widget.Hide()
	} else {
		screens := a.appInstance.Screen.GetAll()
		if len(screens) > 0 {
			primaryScreen := screens[0]
			x := primaryScreen.WorkArea.X + primaryScreen.WorkArea.Width - 250 - 12
			y := primaryScreen.WorkArea.Y + 4
			if runtime.GOOS == "windows" {
				y = primaryScreen.WorkArea.Y + primaryScreen.WorkArea.Height - 320 - 12
			}
			widget.SetPosition(x, y)
		}
		widget.Show()
		widget.Focus()
	}
}

func (a *App) HideWidget() {
	a.mu.Lock()
	widget := a.widgetWin
	a.widgetVisible = false
	a.mu.Unlock()

	if widget != nil {
		widget.Hide()
	}
}

func (a *App) ExpandToWidget() {
	a.mu.Lock()
	widget := a.widgetWin
	dash := a.dashboardWin
	a.dashboardWin = nil
	a.widgetVisible = true
	a.mu.Unlock()

	if dash != nil {
		dash.Close() 
	}

	if widget != nil {
		screens := a.appInstance.Screen.GetAll()
		if len(screens) > 0 {
			primaryScreen := screens[0]
			x := primaryScreen.WorkArea.X + primaryScreen.WorkArea.Width - 250 - 12
			y := primaryScreen.WorkArea.Y + 4
			if runtime.GOOS == "windows" {
				y = primaryScreen.WorkArea.Y + primaryScreen.WorkArea.Height - 320 - 12
			}
			widget.SetPosition(x, y)
		}
		widget.Show()
		widget.Focus()
	}
}

func (a *App) ExpandToDashboard() {
	a.mu.Lock()
	widget := a.widgetWin
	dash := a.dashboardWin
	a.widgetVisible = false
	a.mu.Unlock()

	if widget != nil {
		widget.Hide()
	}

	if dash != nil {
		dash.Show()
		dash.Focus()
		return
	}

	newDash := a.appInstance.Window.NewWithOptions(application.WebviewWindowOptions{
		Title:     "Vitality Dashboard",
		Width:     1024,
		Height:    768,
		MinWidth:  900,
		MinHeight: 600,
		URL:       "/#dashboard",
	})

	newDash.OnWindowEvent(events.Common.WindowClosing, func(e *application.WindowEvent) {
		a.MarkDashboardClosed()
	})

	a.mu.Lock()
	a.dashboardWin = newDash
	a.mu.Unlock()

	newDash.Show()
	newDash.Focus()
}

func (a *App) MarkDashboardClosed() {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.dashboardWin = nil
}

func (a *App) Quit() {
	if a.appInstance != nil {
		go func() {
			time.Sleep(100 * time.Millisecond)
			a.appInstance.Quit()
		}()
	}
}
