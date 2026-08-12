package main

import (
	"context"
	"embed"
	"encoding/json"
	"log"
	"net/http"

	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"
)

//go:embed all:frontend/dist
var assets embed.FS

func main() {
	myAppBackend := NewApp()
	fsHandler := application.AssetFileServerFS(assets)

	apiHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/dashboard":
			myAppBackend.ExpandToDashboard()
			w.WriteHeader(http.StatusOK)
		case "/api/widget":
			myAppBackend.ExpandToWidget()
			w.WriteHeader(http.StatusOK)
		case "/api/dashboard_closed":
			myAppBackend.MarkDashboardClosed()
			w.WriteHeader(http.StatusOK)
		case "/api/metrics":
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(myAppBackend.GetHistory())
		case "/api/sysinfo":
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(myAppBackend.GetSystemInfo())
		default:
			fsHandler.ServeHTTP(w, r)
		}
	})

	app := application.New(application.Options{
		Name:        "Vitality",
		Description: "System Monitor",
		Services: []application.Service{
			application.NewService(myAppBackend),
		},
		Assets: application.AssetOptions{
			Handler: apiHandler,
		},
		Mac: application.MacOptions{
			ActivationPolicy: application.ActivationPolicyAccessory,
		},
	})

	myAppBackend.appInstance = app

	systray := app.SystemTray.New()
	systray.SetLabel("⚡")

	widgetWin := app.Window.NewWithOptions(application.WebviewWindowOptions{
		Title:       "Vitality Widget",
		Width:       250,
		Height:      320,
		MinWidth:    250,
		MinHeight:   320,
		MaxWidth:    250,
		MaxHeight:   320,
		Frameless:   true, 
		AlwaysOnTop: true,
		Hidden:      true,
		URL:         "/#widget",
		Mac: application.MacWindow{
			InvisibleTitleBarHeight: 0,
		},
	})

	widgetWin.OnWindowEvent(events.Common.WindowLostFocus, func(e *application.WindowEvent) {
		myAppBackend.HideWidget()
	})

	myAppBackend.widgetWin = widgetWin
	myAppBackend.systray = systray

	systray.OnClick(func() {
		myAppBackend.ToggleWidget()
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	myAppBackend.startup(ctx)

	err := app.Run()
	if err != nil {
		log.Fatal(err)
	}
}
