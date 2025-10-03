from __future__ import annotations

import math
import os
import time
from dataclasses import dataclass, asdict
from typing import Any, Dict, List, Optional

import requests
from flask import Flask, jsonify, request, send_from_directory
from flask_cors import CORS


def create_app() -> Flask:
    app = Flask(__name__, static_folder="../static", static_url_path="")
    CORS(app)

    bolt_token = os.environ.get("BOLT_AUTH_TOKEN")

    @dataclass
    class Vehicle:
        id: str
        lat: float
        lng: float
        bearing: float
        category_id: str
        category_name: str
        icon_url: Optional[str]

    CATEGORY_CONFIG: Dict[str, Dict[str, Any]] = {
        "13591": {"name": "Send Motorbike", "color": "#34D399", "emoji": "🟢"},
        "13592": {"name": "XL", "color": "#60A5FA", "emoji": "🔵"},
        "13593": {"name": "Bolt", "color": "#A855F7", "emoji": "🟣"},
        "13595": {"name": "Motorbike", "color": "#FBBF24", "emoji": "🟡"},
        "13596": {"name": "City Ride", "color": "#F97316", "emoji": "🟠"},
    }

    FALLBACK_VEHICLES: List[Vehicle] = [
        Vehicle("1660285396", 18.772542, 98.979779, 278.77, "13591", "Send Motorbike", None),
        Vehicle("1660285397", 18.768400, 98.982120, 102.10, "13593", "Bolt", None),
        Vehicle("1660285398", 18.761230, 98.995430, 45.00, "13596", "City Ride", None),
        Vehicle("1660285399", 18.749310, 98.986540, 310.20, "13592", "XL", None),
        Vehicle("1660285400", 18.755120, 98.999900, 210.40, "13595", "Motorbike", None),
    ]

    def haversine_distance(lat1: float, lng1: float, lat2: float, lng2: float) -> float:
        radius_km = 6371
        d_lat = math.radians(lat2 - lat1)
        d_lng = math.radians(lng2 - lng1)
        a = math.sin(d_lat / 2) ** 2 + math.cos(math.radians(lat1)) * math.cos(
            math.radians(lat2)
        ) * math.sin(d_lng / 2) ** 2
        c = 2 * math.atan2(math.sqrt(a), math.sqrt(1 - a))
        return radius_km * c

    def fetch_from_bolt(lat: float, lng: float, address: str, place_id: str) -> List[Vehicle]:
        if not bolt_token:
            return []

        base_url = "https://user.live.boltsvc.net/mobility/search/poll"
        params = {
            "version": "CA.180.0",
            "deviceId": "ffac2e78-84c8-403d-b34e-8394499d7c29",
            "locale": "en-TH",
            "gps_lat": str(lat),
            "gps_lng": str(lng),
        }
        headers = {
            "Authorization": f"Bearer {bolt_token}",
            "Content-Type": "application/json",
        }
        body = {
            "destination_stops": [],
            "payment_method": {"id": "cash", "type": "default"},
            "pickup_stop": {
                "lat": lat,
                "lng": lng,
                "address": address,
                "place_id": place_id,
            },
            "stage": "overview",
            "viewport": {
                "north_east": {"lat": lat + 0.02, "lng": lng + 0.02},
                "south_west": {"lat": lat - 0.02, "lng": lng - 0.02},
            },
        }
        response = requests.post(base_url, params=params, headers=headers, json=body, timeout=6)
        response.raise_for_status()
        payload = response.json()

        vehicles: List[Vehicle] = []
        vehicles_data: Dict[str, Any] = payload.get("vehicles", {})
        category_details: Dict[str, Any] = payload.get("category_details", {})

        for service_name, service_categories in vehicles_data.items():
            details_by_category = category_details.get(service_name, {})
            for category_id, items in service_categories.items():
                for item in items:
                    details = details_by_category.get(category_id, {})
                    vehicles.append(
                        Vehicle(
                            id=str(item.get("id")),
                            lat=float(item.get("lat", lat)),
                            lng=float(item.get("lng", lng)),
                            bearing=float(item.get("bearing", 0.0)),
                            category_id=str(category_id),
                            category_name=details.get(
                                "name",
                                CATEGORY_CONFIG.get(category_id, {}).get("name", "Unknown"),
                            ),
                            icon_url=item.get("icon_url"),
                        )
                    )
        return vehicles

    @app.route("/")
    def index() -> Any:
        return send_from_directory(app.static_folder, "index.html")

    @app.route("/api/vehicles", methods=["POST"])
    def get_vehicles() -> Any:
        body = request.get_json(force=True) if request.data else {}
        lat = float(body.get("lat", 18.756651))
        lng = float(body.get("lng", 98.994667))
        address = body.get("address", "135 ซอย หมู่บ้านในฝัน")
        place_id = body.get("place_id", "google|ChIJwSgfJj4w2jAR_72NE5V00bA")

        try:
            vehicles = fetch_from_bolt(lat, lng, address, place_id)
        except Exception:
            vehicles = []

        if not vehicles:
            vehicles = FALLBACK_VEHICLES

        enriched: List[Dict[str, Any]] = []
        category_counts: Dict[str, int] = {}
        nearest_vehicle: Optional[Dict[str, Any]] = None
        min_distance = float("inf")

        for vehicle in vehicles:
            distance_km = haversine_distance(lat, lng, vehicle.lat, vehicle.lng)
            record = {
                **asdict(vehicle),
                "distance_km": distance_km,
                "color": CATEGORY_CONFIG.get(vehicle.category_id, {}).get("color", "#FFFFFF"),
                "emoji": CATEGORY_CONFIG.get(vehicle.category_id, {}).get("emoji", "🚗"),
            }
            enriched.append(record)
            category_counts[vehicle.category_id] = category_counts.get(vehicle.category_id, 0) + 1

            if distance_km < min_distance:
                min_distance = distance_km
                nearest_vehicle = record

        categories: List[Dict[str, Any]] = []
        for category_id, config in CATEGORY_CONFIG.items():
            categories.append(
                {
                    "id": category_id,
                    "name": config["name"],
                    "color": config["color"],
                    "emoji": config["emoji"],
                    "count": category_counts.get(category_id, 0),
                }
            )

        response_payload = {
            "vehicles": enriched,
            "categories": categories,
            "stats": {
                "total": len(enriched),
                "nearest": nearest_vehicle,
                "last_update": time.strftime("%Y-%m-%d %H:%M:%S"),
            },
        }
        return jsonify(response_payload)

    return app


app = create_app()


if __name__ == "__main__":
    app.run(host="0.0.0.0", port=5000, debug=True)
