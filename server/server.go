package server

import (
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/gofiber/contrib/websocket"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/logger"
	"github.com/tsuru/rate-limit-control-plane/internal/repository"
)

func Notification(repo *repository.ZoneDataRepository) {
	app := fiber.New()

	app.Server().LogAllErrors = true
	app.Use(logger.New(logger.Config{
		Format:     "[${time}] ${status} - ${latency} ${method} ${path}\n",
		TimeFormat: time.RFC3339,
		TimeZone:   "Local",
	}))

	app.Get("/", func(c *fiber.Ctx) error {
		instances := repo.ListInstances()
		b, err := json.Marshal(instances)
		if err != nil {
			log.Println("Error marshaling JSON:", err)
			return c.Status(fiber.StatusInternalServerError).SendString("Error marshaling JSON")
		}
		return c.Type("html").SendString(fmt.Sprintf(`
		<!DOCTYPE html>
			<html lang="en">
			<head>
				<meta charset="UTF-8">
				<meta name="viewport" content="width=device-width, initial-scale=1.0">
				<title>Instances</title>
				<script src="https://cdn.tailwindcss.com"></script>
			</head>
			<body class="bg-gray-900 text-white min-h-screen p-6">
				<div class="max-w-5xl mx-auto">
					<h1 class="text-3xl font-bold mb-6">Instances</h1>
					<div class="overflow-y-auto max-h-[70vh] border border-gray-700 rounded-lg">
						<table class="min-w-full text-sm text-left">
							<thead class="bg-gray-800 text-gray-300 sticky top-0">
								<tr>
									<th class="px-6 py-3">Name</th>
								</tr>
							</thead>
							<tbody id="table-body" class="bg-gray-900 divide-y divide-gray-700">
								<!-- Rows go here -->
							</tbody>
						</table>
					</div>
				</div>

				<script>
					const tableBody = document.getElementById("table-body");

					const data = %s;
					tableBody.innerHTML = "";
					console.log(data);
					data.forEach(item => {
						const row = document.createElement("tr");
						
						const idName = document.createElement("td");
						idName.className = "px-6 py-4";

						const link = document.createElement("a");
						link.href = %s;
						link.textContent = item;
						link.className = "text-blue-600 hover:underline";

						idName.appendChild(link);
						row.appendChild(idName);
						tableBody.appendChild(row);
					});
				</script>
			</body>
			</html>
		`, b, "`/instances/${item}`"))
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
		return c.Type("html").SendString(
			fmt.Sprintf(`
			<!DOCTYPE html>
			<html lang="en">
			<head>
				<meta charset="UTF-8">
				<meta name="viewport" content="width=device-width, initial-scale=1.0">
				<title>Zone Table</title>
				<script src="https://cdn.tailwindcss.com"></script>
			</head>
			<body class="bg-gray-900 text-white min-h-screen p-6">
				<div class="max-w-5xl mx-auto">
					<h1 class="text-3xl font-bold mb-6">Zone Table</h1>
					<div class="overflow-y-auto max-h-[70vh] border border-gray-700 rounded-lg">
						<table class="min-w-full text-sm text-left">
							<thead class="bg-gray-800 text-gray-300 sticky top-0">
								<tr>
									<th class="px-6 py-3">ID</th>
									<th class="px-6 py-3">Last</th>
									<th class="px-6 py-3">Excess</th>
								</tr>
							</thead>
							<tbody id="table-body" class="bg-gray-900 divide-y divide-gray-700">
								<!-- Rows go here -->
							</tbody>
						</table>
					</div>
				</div>

				<script>
					console.log("ws://" + window.location.host + "/ws")
					const ws = new WebSocket("ws://" + window.location.host + "/ws/%s");
					const tableBody = document.getElementById("table-body");

					ws.onmessage = function(event) {
						const data = JSON.parse(event.data);
						tableBody.innerHTML = ""; // clear previous rows
						data.forEach(item => {
							const row = document.createElement("tr");

							const idCell = document.createElement("td");
							idCell.textContent = item.id;
							idCell.className = "px-6 py-4";

							const lastCell = document.createElement("td");
							lastCell.textContent = item.last;
							lastCell.className = "px-6 py-4";

							const excessCell = document.createElement("td");
							excessCell.textContent = item.excess;
							excessCell.className = "px-6 py-4";

							row.appendChild(idCell);
							row.appendChild(lastCell);
							row.appendChild(excessCell);

							tableBody.appendChild(row);
						});
					};
				</script>
			</body>
			</html>
		`, instance))
	})

	log.Println("Server started at http://localhost:3000")
	log.Fatal(app.Listen(":3000"))
}
