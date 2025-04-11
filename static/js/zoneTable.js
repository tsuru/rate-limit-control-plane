// static/js/zoneTable.js
document.addEventListener('DOMContentLoaded', function () {
  console.log("ws://" + window.location.host + "/ws/", instanceName);
  const ws = new WebSocket("ws://" + window.location.host + "/ws/" + instanceName);
  const tableBody = document.getElementById("table-body");

  ws.onmessage = function (event) {
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
});
