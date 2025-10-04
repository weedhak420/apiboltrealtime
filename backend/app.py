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

    bolt_token = os.environ.get(
        "BOLT_TOKEN",
        os.environ.get(
            "BOLT_AUTH_TOKEN",
            "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJkYXRhIjp7InVzZXJfaWQiOjI4MzYxNzQ5NSwidXNlcl9sb2dpbl9pZCI6NjAzNzMzNTg3fSwiaWF0IjoxNzU5NTEwNTg3LCJleHAiOjE3NTk1MTQxODd9.TuM29RFnJGWxmrpwPQVKaO_gu1t9bS6mNBv_Q9zK0-U",
        ),
    )

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
    CATEGORY_TARGET_COUNTS = {
        "13591": 15,
        "13592": 5,
        "13593": 14,
        "13595": 3,
        "13596": 11,
    }

    def build_fallback_vehicles(
        origin_lat: float = 18.756651,
        origin_lng: float = 98.994667,
    ) -> List[Vehicle]:
        vehicles: List[Vehicle] = []
        radius_step_km = 0.6
        angle_increment = 360 / sum(CATEGORY_TARGET_COUNTS.values())
        angle = 0.0

        for category_id, total in CATEGORY_TARGET_COUNTS.items():
            config = CATEGORY_CONFIG.get(category_id, {})
            for idx in range(total):
                # Spread fallback vehicles radially around the pickup point.
                radius_km = radius_step_km * (1 + (idx % 3) * 0.35)
                angle_rad = math.radians(angle)
                d_lat = (radius_km / 111) * math.cos(angle_rad)
                d_lng = (radius_km / (111 * math.cos(math.radians(origin_lat)))) * math.sin(angle_rad)
                vehicles.append(
                    Vehicle(
                        id=f"{category_id}-{idx+1:03d}",
                        lat=origin_lat + d_lat,
                        lng=origin_lng + d_lng,
                        bearing=(angle % 360),
                        category_id=category_id,
                        category_name=config.get("name", "Unknown"),
                        icon_url=None,
                    )
                )
                angle += angle_increment
        return vehicles

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
            "device_name": "XiaomiMi 11 Lite 4G",
            "device_os_version": "12",
            "channel": "googleplay",
            "brand": "bolt",
            "deviceType": "android",
            "signup_session_id": "",
            "country": "th",
            "is_local_authentication_available": "false",
            "language": "th",
            "gps_lat": f"{lat:.6f}",
            "gps_lng": f"{lng:.6f}",
            "gps_accuracy_m": "10.0",
            "gps_age": "0",
            "user_id": "283617495",
            "session_id": "283617495u1759509459582",
            "distinct_id": "client-283617495",
            "rh_session_id": "283617495u1759509267",
        }
        headers = {
            "Host": "user.live.boltsvc.net",
            "Authorization": f"Bearer {bolt_token}",
            "Content-Type": "application/json; charset=UTF-8",
            "Accept-Encoding": "gzip, deflate, br",
            "User-Agent": "okhttp/4.12.0",
        }
        viewport_padding_lat = 0.02070900924817
        viewport_padding_lng = 0.02647531139420637
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
                "north_east": {
                    "lat": lat + viewport_padding_lat,
                    "lng": lng + viewport_padding_lng,
                },
                "south_west": {
                    "lat": lat - viewport_padding_lat,
                    "lng": lng - viewport_padding_lng,
                },
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

        used_live_data = True
        try:
            vehicles = fetch_from_bolt(lat, lng, address, place_id)
        except Exception:
            used_live_data = False
            vehicles = []

        if not vehicles:
            used_live_data = False
            vehicles = build_fallback_vehicles(lat, lng)

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
                "connection": "connected" if used_live_data else "offline",
            },
            "poll_interval_sec": 2,
        }
        return jsonify(response_payload)

    return app


app = create_app()


if __name__ == "__main__":
    app.run(host="0.0.0.0", port=5000, debug=True)
