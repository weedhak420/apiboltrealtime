"""Centralized configuration for Bolt Taxi Tracker."""

from __future__ import annotations

from typing import Any, Dict, List

# Map defaults that the frontend can use to center the map.
MAP_SETTINGS: Dict[str, Any] = {
    "center": {"lat": 18.7883, "lng": 98.9853},
    "zoom": 12,
}

# Offset (in degrees) applied to create the Bolt viewport bounding box.
VIEWPORT_PADDING: float = 0.018

# Structured list of locations that will be polled.
LOCATIONS: List[Dict[str, Any]] = [
    {
        "id": "city_center",
        "name": {"th": "ใจกลางเมือง", "en": "City Center"},
        "district": "Mueang Chiang Mai",
        "type": "urban",
        "priority": 1,
        "coordinates": {"lat": 18.7883, "lng": 98.9853},
    },
    {
        "id": "old_city",
        "name": {"th": "เมืองเก่า", "en": "Old City"},
        "district": "Mueang Chiang Mai",
        "type": "historic",
        "priority": 1,
        "coordinates": {"lat": 18.7912, "lng": 98.9853},
    },
    {
        "id": "tha_phae_gate",
        "name": {"th": "ประตูท่าแพ", "en": "Tha Phae Gate"},
        "district": "Mueang Chiang Mai",
        "type": "historic",
        "priority": 1,
        "coordinates": {"lat": 18.7868, "lng": 98.9931},
    },
    {
        "id": "nimman",
        "name": {"th": "นิมมาน", "en": "Nimman"},
        "district": "Mueang Chiang Mai",
        "type": "lifestyle",
        "priority": 1,
        "coordinates": {"lat": 18.8002, "lng": 98.9679},
    },
    {
        "id": "cmu_area",
        "name": {"th": "มช.", "en": "CMU Area"},
        "district": "Mueang Chiang Mai",
        "type": "education",
        "priority": 2,
        "coordinates": {"lat": 18.8063, "lng": 98.9511},
    },
    {
        "id": "maya_mall",
        "name": {"th": "เมญ่า", "en": "Maya Mall"},
        "district": "Mueang Chiang Mai",
        "type": "shopping",
        "priority": 2,
        "coordinates": {"lat": 18.8025, "lng": 98.9667},
    },
    {
        "id": "airport",
        "name": {"th": "สนามบิน", "en": "Airport"},
        "district": "Mueang Chiang Mai",
        "type": "transport",
        "priority": 1,
        "coordinates": {"lat": 18.7667, "lng": 98.9625},
    },
    {
        "id": "san_kamphaeng",
        "name": {"th": "สันกำแพง", "en": "San Kamphaeng"},
        "district": "San Kamphaeng",
        "type": "suburban",
        "priority": 3,
        "coordinates": {"lat": 18.75, "lng": 99.1167},
    },
    {
        "id": "hang_dong",
        "name": {"th": "หางดง", "en": "Hang Dong"},
        "district": "Hang Dong",
        "type": "suburban",
        "priority": 3,
        "coordinates": {"lat": 18.6833, "lng": 98.9167},
    },
    {
        "id": "doi_saket",
        "name": {"th": "ดอยสะเก็ด", "en": "Doi Saket"},
        "district": "Doi Saket",
        "type": "suburban",
        "priority": 3,
        "coordinates": {"lat": 18.9167, "lng": 99.1667},
    },
    {
        "id": "mae_rim",
        "name": {"th": "แม่ริม", "en": "Mae Rim"},
        "district": "Mae Rim",
        "type": "suburban",
        "priority": 2,
        "coordinates": {"lat": 18.9167, "lng": 98.8833},
    },
    {
        "id": "doi_suthep",
        "name": {"th": "ดอยสุเทพ", "en": "Doi Suthep"},
        "district": "Mueang Chiang Mai",
        "type": "nature",
        "priority": 2,
        "coordinates": {"lat": 18.8047, "lng": 98.9217},
    },
    {
        "id": "san_sai",
        "name": {"th": "สันทราย", "en": "San Sai"},
        "district": "San Sai",
        "type": "suburban",
        "priority": 3,
        "coordinates": {"lat": 18.8667, "lng": 99.0333},
    },
    {
        "id": "saraphi",
        "name": {"th": "สารภี", "en": "Saraphi"},
        "district": "Saraphi",
        "type": "suburban",
        "priority": 3,
        "coordinates": {"lat": 18.7167, "lng": 99.0167},
    },
]

DISTRICTS: List[str] = sorted({location["district"] for location in LOCATIONS})
LOCATION_TYPES: List[str] = sorted({location["type"] for location in LOCATIONS})

# Quick lookup table when a location needs to be referenced by id.
LOCATION_LOOKUP: Dict[str, Dict[str, Any]] = {loc["id"]: loc for loc in LOCATIONS}

__all__ = [
    "LOCATIONS",
    "DISTRICTS",
    "LOCATION_TYPES",
    "LOCATION_LOOKUP",
    "MAP_SETTINGS",
    "VIEWPORT_PADDING",
]
