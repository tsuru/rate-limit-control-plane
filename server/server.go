package server

import (
	"encoding/json"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/gofiber/contrib/websocket"
	"github.com/gofiber/fiber/v2"
	fiberLogger "github.com/gofiber/fiber/v2/middleware/logger"
	"github.com/gofiber/template/html/v2"
	"github.com/tsuru/rate-limit-control-plane/internal/logger"
	"github.com/tsuru/rate-limit-control-plane/internal/repository"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var eventLatency = promauto.NewHistogram(prometheus.HistogramOpts{
	Name:    "event_latency_seconds",
	Help:    "Latency of events in seconds",
	Buckets: prometheus.ExponentialBuckets(0.001, 2, 15),
})

func processEvent(start time.Time) {
	duration := time.Since(start).Seconds()
	eventLatency.Observe(duration)
}

func Notification(repo *repository.ZoneDataRepository, listenAddr string) {
	go func() {
		http.Handle("/metrics", promhttp.Handler())
		http.ListenAndServe(":3001", nil)
	}()

	serverLogger := logger.NewLogger(map[string]string{"emitter": "rate-limit-control-plane-notification-server"}, os.Stdout)
	// Initialize template engine
	engine := html.New("./views", ".html")

	app := fiber.New(fiber.Config{
		Views: engine,
	})

	app.Server().LogAllErrors = true
	app.Use(fiberLogger.New(fiberLogger.Config{
		Format:     "[${time}] ${status} - ${latency} ${method} ${path}\n",
		TimeFormat: time.RFC3339,
		TimeZone:   "Local",
	}))

	// Setup static files
	app.Static("/static", "./static")

	app.Get("/", func(c *fiber.Ctx) error {
		start := time.Now()
		defer processEvent(start)
		instances := repo.ListInstances()
		instancesJSON, err := json.Marshal(instances)
		if err != nil {
			serverLogger.Error("Error marshaling JSON", "error", err)
			return c.Status(fiber.StatusInternalServerError).SendString("Error marshaling JSON")
		}

		return c.Render("index", fiber.Map{
			"InstancesJSON": string(instancesJSON),
		})
	})

	app.Use("/ws/:rpaasName", func(c *fiber.Ctx) error {
		if websocket.IsWebSocketUpgrade(c) {
			serverLogger.Info("WS upgrade requested for rpaasName", "rpaasName", c.Params("rpaasName"))
			c.Locals("allowed", true)
			return c.Next()
		}
		return fiber.ErrUpgradeRequired
	})

	app.Get("/ws/:rpaasName", websocket.New(func(c *websocket.Conn) {
		rpaasName := c.Params("rpaasName")
		for {
			data, ok := repo.GetRpaasZoneData(rpaasName)
			if !ok {
				if err := c.WriteMessage(websocket.TextMessage, []byte("Not Found")); err != nil {
					serverLogger.Error("Error sending message", "error", err)
					return
				}
			}
			if err := c.WriteMessage(websocket.TextMessage, data); err != nil {
				serverLogger.Error("Error sending message", "error", err)
				return
			}
			time.Sleep(2 * time.Second)
		}
	}))

	app.Get("/instances/:instance", func(c *fiber.Ctx) error {
		instance := c.Params("instance")
		return c.Render("instance", fiber.Map{
			"Instance": instance,
		})
	})

	// Create necessary directories and files
	setupStaticFiles()

	log.Fatal(app.Listen(listenAddr))
}

func setupStaticFiles() {
	// Create directories
	dirs := []string{
		"views",
		"static/js",
		"static/css",
	}

	for _, dir := range dirs {
		createDirIfNotExists(dir)
	}
}

func createDirIfNotExists(dir string) {
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		err := os.MkdirAll(dir, os.ModePerm)
		if err != nil {
			log.Printf("Error creating directory %s: %v\n", dir, err)
		}
	}
}
