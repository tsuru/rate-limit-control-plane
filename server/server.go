package server

import (
	"encoding/json"
	"log"

	"github.com/gofiber/contrib/websocket"
	"github.com/gofiber/fiber/v2"
)

func Notification(ch chan Data) {
	app := fiber.New()

	app.Use("/ws", func(c *fiber.Ctx) error {
		if websocket.IsWebSocketUpgrade(c) {
			c.Locals("allowed", true)
			return c.Next()
		}
		return fiber.ErrUpgradeRequired
	})

	app.Get("/ws", websocket.New(func(c *websocket.Conn) {
		for value := range ch {
			data := []Data{
				{
					ID:     value.ID,
					Last:   value.Last,
					Excess: value.Excess,
				},
			}
			msg, err := json.Marshal(data)
			if err != nil {
				log.Println("Error marshaling JSON:", err)
				return
			}
			if err := c.WriteMessage(websocket.TextMessage, msg); err != nil {
				log.Println("Error sending message:", err)
				return
			}
		}
	}))

	app.Get("/", func(c *fiber.Ctx) error {
		return c.Type("html").SendString(`
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
					const ws = new WebSocket("ws://" + window.location.host + "/ws");
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
		`)
	})

	log.Println("Server started at http://localhost:3000")
	log.Fatal(app.Listen(":3000"))
}
