package test

import (
	"fmt"
	"net"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/vmihailenco/msgpack/v5"
)

func scanPort() (int, error) {
	for port := 30000; port < 65535; port++ {
		address := fmt.Sprintf("localhost:%d", port)
		conn, err := net.DialTimeout("tcp", address, time.Duration(1)*time.Second)
		if err != nil {
			continue
		}
		conn.Close()
		return port, nil
	}
	return 0, fmt.Errorf("no available port found")
}

func NewServerMock() {
	repository := NewRepository()
	app := fiber.New()
	app.Get("/rate-limit", func(c *fiber.Ctx) error {
		var zones []string
		for key := range repository.GetRateLimit() {
			zones = append(zones, key)
		}
		encodedData, err := msgpack.Marshal(zones)
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).SendString("Failed to encode data")
		}
		c.Set("Content-Type", "application/msgpack")
		return c.Send(encodedData)
	})

	app.Get("/:zone", func(c *fiber.Ctx) error {
		zones := repository.GetRateLimit()
		zone := c.Params("zone")
		data, ok := zones[zone]
		if !ok {
			return c.Status(fiber.StatusNotFound).SendString("Zone not found")
		}
		encodedData, err := msgpack.Marshal(data)
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).SendString("Failed to encode data")
		}
		c.Set("Content-Type", "application/msgpack")
		return c.Send(encodedData)
	})

	app.Put("/:zone", func(c *fiber.Ctx) error {
		var body []Repository
		if err := c.BodyParser(&body); err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid request"})
		}
		zone := c.Params("zone")
		repository.SetRateLimit(zone, body)
		return c.SendStatus(fiber.StatusNoContent)
	})
	port, err := scanPort()
	if err != nil {
		panic(err)
	}
	if err := app.Listen(fmt.Sprintf(":%d", port)); err != nil {
		panic(err)
	}
}
