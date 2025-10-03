const CATEGORY_CONFIG = {
  "13591": { name: "Send Motorbike", color: "#34D399", emoji: "🟢" },
  "13592": { name: "XL", color: "#60A5FA", emoji: "🔵" },
  "13593": { name: "Bolt", color: "#A855F7", emoji: "🟣" },
  "13595": { name: "Motorbike", color: "#FBBF24", emoji: "🟡" },
  "13596": { name: "City Ride", color: "#F97316", emoji: "🟠" },
};
const CATEGORY_TARGET_COUNTS = {
  "13591": 15,
  "13592": 5,
  "13593": 14,
  "13595": 3,
  "13596": 11,
};

const map = L.map("map", {
  zoomControl: true,
  scrollWheelZoom: true,
}).setView([18.756651, 98.994667], 13);

L.tileLayer("https://{s}.tile.openstreetmap.org/{z}/{x}/{y}.png", {
  maxZoom: 19,
  attribution: "© OpenStreetMap contributors",
}).addTo(map);

let pickupMarker = null;
let markersLayer = L.layerGroup().addTo(map);
let activeCategory = null;
let pollingTimer = null;
let pollIntervalMs = 2000;
let latestData = null;
let latestPickup = { lat: 18.756651, lng: 98.994667, address: "135 ซอย หมู่บ้านในฝัน" };

const locationForm = document.getElementById("location-form");
const categoryList = document.getElementById("category-list");
const statsGrid = document.getElementById("stats-grid");
const clearFilterBtn = document.getElementById("clear-filter");
const connectionIndicator = document.getElementById("connection-indicator");
const connectionStatus = document.getElementById("connection-status");
const lastUpdate = document.getElementById("last-update");
const totalVehicles = document.getElementById("total-vehicles");
const pollIntervalDisplay = document.getElementById("poll-interval");

function rotateIcon(bearing) {
  return `transform: rotate(${bearing}deg);`;
}

function calculateDistance(lat1, lon1, lat2, lon2) {
  const R = 6371;
  const dLat = ((lat2 - lat1) * Math.PI) / 180;
  const dLon = ((lon2 - lon1) * Math.PI) / 180;
  const a =
    Math.sin(dLat / 2) * Math.sin(dLat / 2) +
    Math.cos((lat1 * Math.PI) / 180) *
      Math.cos((lat2 * Math.PI) / 180) *
      Math.sin(dLon / 2) * Math.sin(dLon / 2);
  const c = 2 * Math.atan2(Math.sqrt(a), Math.sqrt(1 - a));
  return R * c;
}

function buildOfflineData(pickup) {
  const vehicles = [];
  const totalVehiclesCount = Object.values(CATEGORY_TARGET_COUNTS).reduce(
    (sum, count) => sum + count,
    0,
  );
  const angleIncrement = 360 / totalVehiclesCount;
  const baseRadiusKm = 0.6;
  let angle = 0;

  Object.entries(CATEGORY_TARGET_COUNTS).forEach(([categoryId, count]) => {
    const config = CATEGORY_CONFIG[categoryId] || {};
    for (let idx = 0; idx < count; idx += 1) {
      const radiusKm = baseRadiusKm * (1 + (idx % 3) * 0.35);
      const angleRad = (angle * Math.PI) / 180;
      const dLat = (radiusKm / 111) * Math.cos(angleRad);
      const dLng = (radiusKm / (111 * Math.cos((pickup.lat * Math.PI) / 180))) * Math.sin(angleRad);
      const lat = pickup.lat + dLat;
      const lng = pickup.lng + dLng;
      const distance_km = calculateDistance(pickup.lat, pickup.lng, lat, lng);

      vehicles.push({
        id: `${categoryId}-${idx + 1}`,
        lat,
        lng,
        bearing: angle % 360,
        category_id: categoryId,
        category_name: config.name || "Unknown",
        color: config.color || "#F1F5F9",
        emoji: config.emoji || "🚗",
        icon_url: null,
        distance_km,
      });

      angle += angleIncrement;
    }
  });

  const categories = Object.entries(CATEGORY_CONFIG).map(([id, config]) => ({
    id,
    name: config.name,
    color: config.color,
    emoji: config.emoji,
    count: CATEGORY_TARGET_COUNTS[id] || 0,
  }));

  const nearest = vehicles.reduce((closest, vehicle) => {
    if (!closest || vehicle.distance_km < closest.distance_km) {
      return vehicle;
    }
    return closest;
  }, null);

  return {
    vehicles,
    categories,
    stats: {
      total: vehicles.length,
      nearest,
      last_update: new Date().toISOString().replace("T", " ").slice(0, 19),
      connection: "offline",
    },
    poll_interval_sec: pollIntervalMs / 1000,
  };
}

function setPickupMarker(lat, lng) {
  if (pickupMarker) {
    pickupMarker.setLatLng([lat, lng]);
  } else {
    pickupMarker = L.marker([lat, lng], {
      icon: L.divIcon({
        className: "",
        html: `<div class="marker-icon" style="--marker-color:#FACC15">📍</div>`,
        iconSize: [36, 36],
        iconAnchor: [18, 36],
      }),
    }).addTo(map);
  }
}

function createVehicleIcon(vehicle) {
  const { color, emoji } = vehicle;
  const rotationStyle = rotateIcon(vehicle.bearing || 0);
  const inverseRotation = -(vehicle.bearing || 0);
  const iconContent = vehicle.icon_url
    ? `<img src="${vehicle.icon_url}" alt="${vehicle.category_name}" style="transform: rotate(${inverseRotation}deg);" />`
    : `<span style="display:inline-block; transform: rotate(${inverseRotation}deg);">${emoji}</span>`;

  return L.divIcon({
    className: "",
    html: `
      <div class="relative flex flex-col items-center">
        <div class="marker-icon" style="--marker-color:${color}; ${rotationStyle}">
          ${iconContent}
        </div>
      </div>
    `,
    iconSize: [36, 36],
    iconAnchor: [18, 36],
  });
}

function renderMarkers(vehicles, pickup) {
  markersLayer.clearLayers();
  const bounds = [];

  vehicles
    .filter((vehicle) => !activeCategory || vehicle.category_id === activeCategory)
    .forEach((vehicle) => {
      const marker = L.marker([vehicle.lat, vehicle.lng], {
        icon: createVehicleIcon(vehicle),
      });

      const distance =
        typeof vehicle.distance_km === "number"
          ? vehicle.distance_km
          : calculateDistance(pickup.lat, pickup.lng, vehicle.lat, vehicle.lng);
      const popupHtml = `
        <div class="space-y-2">
          <div class="flex items-center gap-2 text-sm font-semibold">
            <span style="color:${vehicle.color}">${vehicle.emoji}</span>
            <span>${vehicle.category_name}</span>
          </div>
          <table class="popup-table">
            <tr><td>ID</td><td>${vehicle.id}</td></tr>
            <tr><td>Coords</td><td>${vehicle.lat.toFixed(5)}, ${vehicle.lng.toFixed(5)}</td></tr>
            <tr><td>Bearing</td><td>${Math.round(vehicle.bearing)}°</td></tr>
            <tr><td>Distance</td><td>${distance.toFixed(2)} km</td></tr>
          </table>
        </div>
      `;
      marker.bindPopup(popupHtml);
      marker.addTo(markersLayer);
      bounds.push([vehicle.lat, vehicle.lng]);
    });

  if (bounds.length > 0) {
    const leafletBounds = L.latLngBounds(bounds);
    map.fitBounds(leafletBounds.pad(0.25));
  } else {
    map.setView([pickup.lat, pickup.lng], map.getZoom());
  }
}

function renderCategories(categories) {
  categoryList.innerHTML = "";

  categories.forEach((category) => {
    const card = document.createElement("button");
    card.className = `category-card w-full rounded-xl border border-slate-800 bg-slate-800/60 px-4 py-3 text-left transition focus:outline-none focus:ring-2 focus:ring-emerald-400 ${
      activeCategory === category.id ? "ring-2 ring-emerald-400" : ""
    }`;
    card.dataset.categoryId = category.id;
    card.innerHTML = `
      <div class="flex items-center justify-between">
        <div class="flex items-center gap-3">
          <span class="text-2xl" style="color:${category.color}">${category.emoji}</span>
          <div>
            <p class="font-semibold text-slate-100">${category.name}</p>
            <p class="text-xs text-slate-400">Category ID: ${category.id}</p>
          </div>
        </div>
        <span class="text-lg font-semibold text-emerald-300">${category.count}</span>
      </div>
    `;
    card.addEventListener("click", () => {
      activeCategory = activeCategory === category.id ? null : category.id;
      renderCategories(categories);
      if (latestData) {
        renderMarkers(latestData.vehicles, latestPickup);
      }
    });
    categoryList.appendChild(card);
  });
}

function renderStats(data) {
  const { total, nearest, connection, last_update } = data.stats;
  totalVehicles.textContent = `${total} Vehicles`;
  if (last_update) {
    lastUpdate.textContent = `Last update: ${last_update}`;
  }
  const statsItems = [
    {
      label: "Total Vehicles",
      value: total,
    },
    {
      label: "Connection Status",
      value: connection ? connection.charAt(0).toUpperCase() + connection.slice(1) : "--",
    },
    {
      label: "Last Update",
      value: last_update || "--",
    },
    ...data.categories.map((category) => ({
      label: category.name,
      value: category.count,
      color: category.color,
    })),
  ];

  if (nearest) {
    statsItems.push({
      label: "Nearest Vehicle",
      value: `${nearest.id} (${nearest.category_name})<br>${nearest.distance_km.toFixed(2)} km`,
      color: nearest.color,
    });
  }

  statsGrid.innerHTML = statsItems
    .map((item) => {
      const displayValue =
        typeof item.value === "number" ? item.value.toLocaleString() : item.value;
      return `
        <div class="rounded-xl border border-slate-800 bg-slate-800/60 p-4">
          <p class="text-xs uppercase tracking-wide text-slate-400">${item.label}</p>
          <p class="mt-2 text-2xl font-semibold" style="color:${item.color || "#F8FAFC"}">${displayValue}</p>
        </div>
      `;
    })
    .join("");
}

async function fetchVehicles(payload) {
  connectionIndicator.classList.replace("bg-red-400", "bg-amber-400");
  connectionStatus.textContent = "Refreshing...";

  try {
    const response = await fetch("/api/vehicles", {
      method: "POST",
      headers: {
        "Content-Type": "application/json",
      },
      body: JSON.stringify(payload),
    });

    if (!response.ok) {
      throw new Error("Failed to fetch vehicles");
    }

    const data = await response.json();
    const isConnected = data.stats.connection === "connected";
    connectionIndicator.classList.remove("bg-red-400", "bg-amber-400", "bg-emerald-400");
    connectionIndicator.classList.add(isConnected ? "bg-emerald-400" : "bg-red-400");
    connectionStatus.textContent = isConnected ? "Connected" : "Offline data";
    lastUpdate.textContent = `Last update: ${data.stats.last_update}`;
    if (typeof data.poll_interval_sec === "number") {
      pollIntervalMs = data.poll_interval_sec * 1000;
      pollIntervalDisplay.textContent = data.poll_interval_sec;
      startPolling();
    }
    return data;
  } catch (error) {
    console.error(error);
    connectionIndicator.classList.remove("bg-emerald-400", "bg-amber-400");
    connectionIndicator.classList.add("bg-red-400");
    connectionStatus.textContent = "Disconnected";
    throw error;
  }
}

async function refreshVehicles() {
  const pickup = {
    lat: parseFloat(locationForm.lat.value),
    lng: parseFloat(locationForm.lng.value),
    address: locationForm.address.value,
  };

  setPickupMarker(pickup.lat, pickup.lng);

  try {
    const data = await fetchVehicles(pickup);
    latestData = data;
    latestPickup = pickup;
  } catch (error) {
    latestData = buildOfflineData(pickup);
    latestPickup = pickup;
    connectionIndicator.classList.remove("bg-emerald-400", "bg-amber-400");
    connectionIndicator.classList.add("bg-red-400");
    connectionStatus.textContent = "Offline data";
    pollIntervalDisplay.textContent = pollIntervalMs / 1000;
  }

  renderCategories(latestData.categories);
  renderStats(latestData);
  renderMarkers(latestData.vehicles, latestPickup);
}

function startPolling() {
  if (pollingTimer) {
    clearInterval(pollingTimer);
  }
  pollingTimer = setInterval(() => {
    refreshVehicles();
  }, pollIntervalMs);
}

locationForm.addEventListener("submit", (event) => {
  event.preventDefault();
  activeCategory = null;
  refreshVehicles();
});

clearFilterBtn.addEventListener("click", () => {
  activeCategory = null;
  if (latestData) {
    renderCategories(latestData.categories);
    renderMarkers(latestData.vehicles, latestPickup);
  } else {
    refreshVehicles();
  }
});

refreshVehicles();
startPolling();
