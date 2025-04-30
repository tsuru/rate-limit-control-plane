package test

import (
	"net"

	"github.com/gofiber/fiber/v2"
	"github.com/vmihailenco/msgpack/v5"
)

func NewServerMock(listener net.Listener, repository *Repositories) {
	app := fiber.New(fiber.Config{DisableStartupMessage: true})
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

	app.Get("/rate-limit/:zone", func(c *fiber.Ctx) error {
		zones := repository.GetRateLimit()
		zone := c.Params("zone")
		data, ok := zones[zone]
		if !ok {
			return c.Status(fiber.StatusNotFound).SendString("Zone not found")
		}
		encodedData := make([]byte, 0)
		encondedHeader, err := msgpack.Marshal(data.Header)
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).SendString("Failed to encode header")
		}
		encodedData = append(encodedData, encondedHeader...)
		for _, body := range data.Body {
			encodedBody, err := msgpack.Marshal(body)
			if err != nil {
				return c.Status(fiber.StatusInternalServerError).SendString("Failed to encode body")
			}
			encodedData = append(encodedData, encodedBody...)
		}
		c.Set("Content-Type", "application/msgpack")
		return c.Send(encodedData)
	})

	app.Put("/rate-limit/:zone", func(c *fiber.Ctx) error {
		var body []*Body
		if err := c.BodyParser(&body); err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid request"})
		}
		zone := c.Params("zone")
		repository.SetRateLimit(zone, body)
		return c.SendStatus(fiber.StatusNoContent)
	})

	if err := app.Listener(listener); err != nil {
		panic(err)
	}
}
