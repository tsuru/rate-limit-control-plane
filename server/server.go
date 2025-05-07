package server

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/gofiber/contrib/websocket"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/logger"
	"github.com/gofiber/template/html/v2"
	"github.com/tsuru/rate-limit-control-plane/internal/repository"
	"github.com/tsuru/rate-limit-control-plane/internal/trace"
)

func Notification(repo *repository.ZoneDataRepository, listenAddr string) {
	// Initialize template engine
	engine := html.New("./views", ".html")

	app := fiber.New(fiber.Config{
		Views: engine,
	})

	app.Server().LogAllErrors = true
	app.Use(logger.New(logger.Config{
		Format:     "[${time}] ${status} - ${latency} ${method} ${path}\n",
		TimeFormat: time.RFC3339,
		TimeZone:   "Local",
	}))

	// Setup static files
	app.Static("/static", "./static")

	app.Get("/", func(c *fiber.Ctx) error {
		shutdown := trace.InitTrace("http://jaeger-collector.observability.svc.cluster.local:14268/api/traces")
		defer shutdown()
		tracer := trace.TraceProvider.Tracer("data_test")
		_, span := tracer.Start(context.Background(), "main")
		defer span.End()
		instances := repo.ListInstances()
		instancesJSON, err := json.Marshal(instances)
		if err != nil {
			log.Println("Error marshaling JSON:", err)
			return c.Status(fiber.StatusInternalServerError).SendString("Error marshaling JSON")
		}

		return c.Render("index", fiber.Map{
			"InstancesJSON": string(instancesJSON),
		})
	})

	app.Use("/ws/:rpaasName", func(c *fiber.Ctx) error {
		if websocket.IsWebSocketUpgrade(c) {
			log.Printf("WS upgrade requested for rpaasName: %s\n", c.Params("rpaasName"))
			c.Locals("allowed", true)
			return c.Next()
		}
		return fiber.ErrUpgradeRequired
	})

	app.Get("/ws/:rpaasName", websocket.New(func(c *websocket.Conn) {
		rpaasName := c.Params("rpaasName")
		fmt.Printf("Connected to rpaasName: %s\n", rpaasName)
		for {
			data, ok := repo.GetRpaasZoneData(rpaasName)
			fmt.Printf("Data for rpaasName: %s, ok: %v\n", rpaasName, ok)
			if !ok {
				if err := c.WriteMessage(websocket.TextMessage, []byte("Not Found")); err != nil {
					log.Println("Error sending message:", err)
					return
				}
			}
			if err := c.WriteMessage(websocket.TextMessage, data); err != nil {
				log.Println("Error sending message:", err)
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
