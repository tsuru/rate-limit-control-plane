// static/js/instances.js
document.addEventListener('DOMContentLoaded', function () {
  const tableBody = document.getElementById("table-body");

  tableBody.innerHTML = "";
  console.log(instancesData);
  const instances = JSON.parse(instancesData);

  instances.forEach(item => {
    const row = document.createElement("tr");

    const idName = document.createElement("td");
    idName.className = "px-6 py-4";

    const link = document.createElement("a");
    link.href = `/instances/${item}`;
    link.textContent = item;
    link.className = "text-blue-600 hover:underline";

    idName.appendChild(link);
    row.appendChild(idName);
    tableBody.appendChild(row);
  });
});
