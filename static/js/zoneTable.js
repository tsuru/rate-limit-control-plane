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

      const keyCell = document.createElement("td");
      keyCell.textContent = item.key;
      keyCell.className = "px-6 py-4";

      const zoneCell = document.createElement("td");
      zoneCell.textContent = item.zone;
      zoneCell.className = "px-6 py-4";

      const lastCell = document.createElement("td");

      const date = new Date(item.last);
      const formatted = `${String(date.getDate()).padStart(2, '0')}/${
        String(date.getMonth() + 1).padStart(2, '0')}/${
        String(date.getFullYear()).slice(-2)} ${
        String(date.getHours()).padStart(2, '0')}:${
        String(date.getMinutes()).padStart(2, '0')}:${
        String(date.getSeconds()).padStart(2, '0')}.${
        String(date.getMilliseconds()).padStart(3, '0')}`;

      lastCell.textContent = formatted;
      lastCell.className = "px-6 py-4";

      const excessCell = document.createElement("td");
      excessCell.textContent = item.excess;
      excessCell.className = "px-6 py-4";

      row.appendChild(keyCell);
      row.appendChild(zoneCell);
      row.appendChild(lastCell);
      row.appendChild(excessCell);

      tableBody.appendChild(row);
    });
  };
});
