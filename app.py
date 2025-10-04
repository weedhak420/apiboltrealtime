from flask import Flask, render_template, jsonify
from flask_socketio import SocketIO
import requests
import time
import threading
from concurrent.futures import ThreadPoolExecutor, as_completed
from datetime import datetime

from config import (
    DISTRICTS,
    LOCATION_TYPES,
    LOCATIONS,
    MAP_SETTINGS,
    VIEWPORT_PADDING,
)

app = Flask(__name__)
socketio = SocketIO(app, cors_allowed_origins="*", async_mode='threading')

# ==========================================
# ⚙️ CONFIGURATION
# ==========================================

# โหมดทดสอบ
TEST_MODE = False

# จำนวน workers สำหรับ concurrent requests
MAX_WORKERS = 10

# ความถี่ในการดึงข้อมูล (วินาที)
FETCH_INTERVAL = 1  # ลดจาก 5 เหลือ 3 วินาที

# ==========================================
# 🗺️ LOCATION HELPERS
# ==========================================


def get_location_name(location, language="en"):
    name = location.get("name", {})
    if isinstance(name, dict):
        return name.get(language) or name.get("en") or name.get("th") or location.get("id", "Unknown")
    return name or location.get("id", "Unknown")


def get_coordinates(location):
    coordinates = location.get("coordinates", {})
    return coordinates.get("lat"), coordinates.get("lng")


def summarize_location(location):
    summary_keys = ("id", "name", "district", "type", "priority", "coordinates")
    return {key: location[key] for key in summary_keys if key in location}

# ==========================================
# 🚀 CONCURRENT API FETCHING
# ==========================================

def fetch_single_location(location):
    try:
        lat, lng = get_coordinates(location)
        if lat is None or lng is None:
            raise ValueError("Missing coordinates for location")

        location_name_en = get_location_name(location, "en")
        location_summary = summarize_location(location)
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
            "gps_lat": str(lat),
            "gps_lng": str(lng),
            "gps_accuracy_m": "10.0",
            "gps_age": "0",
            "user_id": "283617495",
            "session_id": f"283617495u{int(time.time())}",
            "distinct_id": "client-283617495",
            "rh_session_id": "283617495u1759507023"
        }

        headers = {
            "Host": "user.live.boltsvc.net",
            "Authorization": "Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJkYXRhIjp7InVzZXJfaWQiOjI4MzYxNzQ5NSwidXNlcl9sb2dpbl9pZCI6NjAzNzMzNTg3fSwiaWF0IjoxNzU5NTU3NTcxLCJleHAiOjE3NTk1NjExNzF9.DEdZUUURblb1e4qMr_B_tytyDsA3N1sbPjzDZEXD8-A",
            "Content-Type": "application/json; charset=UTF-8",
            "Accept-Encoding": "gzip, deflate, br",
            "User-Agent": "okhttp/4.12.0"
        }

        viewport_offset = VIEWPORT_PADDING
        body = {
            "destination_stops": [],
            "payment_method": {"id": "cash", "type": "default"},
            "pickup_stop": {
                "lat": lat,
                "lng": lng,
                "address": location_name_en,
                "place_id": f"custom|{location_summary['id']}"
            },
            "stage": "overview",
            "viewport": {
                "north_east": {"lat": lat + viewport_offset, "lng": lng + viewport_offset},
                "south_west": {"lat": lat - viewport_offset, "lng": lng - viewport_offset}
            }
        }

        response = requests.post(base_url, params=params, headers=headers, json=body, timeout=5)

        if response.status_code == 200:
            return {"location": location_summary, "data": response.json(), "success": True}
        else:
            return {
                "location": location_summary,
                "data": None,
                "success": False,
                "error": f"Status {response.status_code}",
            }
    except Exception as e:
        return {
            "location": summarize_location(location),
            "data": None,
            "success": False,
            "error": str(e),
        }

def fetch_all_locations():
    all_responses = []
    with ThreadPoolExecutor(max_workers=MAX_WORKERS) as executor:
        future_to_location = {executor.submit(fetch_single_location, loc): loc for loc in LOCATIONS}
        for future in as_completed(future_to_location):
            result = future.result()
            all_responses.append(result)
    return all_responses

def process_all_responses(responses):
    all_vehicles = []
    vehicle_ids = set()
    success_count = 0
    fail_count = 0

    for response in responses:
        if not response["success"]:
            fail_count += 1
            continue

        success_count += 1
        data = response["data"]
        if not data:
            continue

        location_info = response.get("location", {})

        vehicles = data.get("data", {}).get("vehicles", {})
        taxi_vehicles = vehicles.get("taxi", {})
        icons = vehicles.get("icons", {}).get("taxi", {})
        categories = vehicles.get("category_details", {}).get("taxi", {})

        for category_id, cars in taxi_vehicles.items():
            if not isinstance(cars, list):
                continue
            for car in cars:
                vehicle_id = car.get("id")
                if vehicle_id in vehicle_ids:
                    continue
                vehicle_ids.add(vehicle_id)
                icon_url = icons.get(category_id, {}).get("icon_url", "").strip()
                vehicle_info = {
                    "id": vehicle_id,
                    "lat": car.get("lat"),
                    "lng": car.get("lng"),
                    "bearing": car.get("bearing", 0),
                    "icon_url": icon_url,
                    "category_name": categories.get(category_id, {}).get("name", "Unknown"),
                    "category_id": category_id,
                    "source_location": location_info,
                }
                if vehicle_info["lat"] and vehicle_info["lng"]:
                    all_vehicles.append(vehicle_info)

    print(f"✅ สำเร็จ: {success_count}/{len(responses)} locations")
    print(f"🚗 พบรถทั้งหมด: {len(all_vehicles)} คัน (ไม่ซ้ำ)")
    return all_vehicles

# ==========================================
# 🧪 FAKE DATA GENERATOR (TEST MODE)
# ==========================================

def generate_fake_data_multi_location():
    import random
    all_vehicles = []
    for idx, location in enumerate(LOCATIONS):
        lat, lng = get_coordinates(location)
        if lat is None or lng is None:
            continue
        location_summary = summarize_location(location)
        num_cars = random.randint(5, 15)
        for i in range(num_cars):
            lat_offset = random.uniform(-0.015, 0.015)
            lng_offset = random.uniform(-0.015, 0.015)
            vehicle = {
                "id": f"FAKE_{idx}_{i}",
                "lat": lat + lat_offset,
                "lng": lng + lng_offset,
                "bearing": random.randint(0, 359),
                "icon_url": "https://raw.githubusercontent.com/pointhi/leaflet-color-markers/master/img/marker-icon-2x-blue.png",
                "category_name": random.choice(["Economy", "Comfort", "XL"]),
                "category_id": "economy",
                "source_location": location_summary,
            }
            all_vehicles.append(vehicle)
    return all_vehicles

# ==========================================
# 🔄 MAIN FETCH LOOP
# ==========================================

def data_fetch_loop():
    print("🚀 เริ่ม multi-location concurrent fetch loop")
    while True:
        try:
            start_time = time.time()
            print("\n" + "="*60)
            print(f"🔄 [{datetime.now().strftime('%H:%M:%S')}] กำลังดึงข้อมูลจาก {len(LOCATIONS)} ตำแหน่ง...")

            if TEST_MODE:
                print("🧪 โหมดทดสอบ: ใช้ข้อมูลปลอม")
                vehicles = generate_fake_data_multi_location()
                responses = []
            else:
                responses = fetch_all_locations()
                vehicles = process_all_responses(responses)

            elapsed = time.time() - start_time

            socketio.emit("vehicles_update", {
                "vehicles": vehicles,
                "count": len(vehicles),
                "locations_count": len(LOCATIONS),
                "success_locations": [r["location"] for r in responses if r.get("success")],
                "failed_locations": [r["location"] for r in responses if not r.get("success")],
                "fetch_time": elapsed,
                "location_metadata": LOCATIONS,
                "config": {
                    "map": MAP_SETTINGS,
                    "districts": DISTRICTS,
                    "types": LOCATION_TYPES,
                }
            })

            print(f"⏳ รอ {FETCH_INTERVAL} วินาที...")
            time.sleep(FETCH_INTERVAL)
        except Exception as e:
            print(f"❌ Error in fetch loop: {e}")
            time.sleep(FETCH_INTERVAL)

# ==========================================
# 🌐 FLASK ROUTES
# ==========================================

@app.route("/")
def index():
    return render_template("index.html")

@app.route("/test-api")
def test_api():
    print("\n🧪 กำลังทดสอบ Multi-location Concurrent API...")
    start_time = time.time()

    if TEST_MODE:
        vehicles = generate_fake_data_multi_location()
        elapsed = time.time() - start_time
        return jsonify({
            "success": True,
            "mode": "TEST",
            "vehicle_count": len(vehicles),
            "locations_count": len(LOCATIONS),
            "fetch_time": f"{elapsed:.2f}s",
            "vehicles_sample": vehicles[:5],
            "location_metadata": LOCATIONS,
            "config": {
                "map": MAP_SETTINGS,
                "districts": DISTRICTS,
                "types": LOCATION_TYPES,
            }
        })

    responses = fetch_all_locations()
    vehicles = process_all_responses(responses)
    elapsed = time.time() - start_time
    success_locations = [r["location"] for r in responses if r.get("success")]
    failed_locations = [r["location"] for r in responses if not r.get("success")]

    return jsonify({
        "success": True,
        "mode": "REAL",
        "vehicle_count": len(vehicles),
        "locations_count": len(LOCATIONS),
        "fetch_time": f"{elapsed:.2f}s",
        "success_locations": success_locations,
        "failed_locations": failed_locations,
        "vehicles_sample": vehicles[:5],
        "location_metadata": LOCATIONS,
        "config": {
            "map": MAP_SETTINGS,
            "districts": DISTRICTS,
            "types": LOCATION_TYPES,
        }
    })

@app.route("/locations")
def get_locations():
    return jsonify({
        "locations": LOCATIONS,
        "count": len(LOCATIONS),
        "districts": DISTRICTS,
        "types": LOCATION_TYPES,
        "map_settings": MAP_SETTINGS,
    })

@socketio.on('connect')
def handle_connect():
    print("✅ Client เชื่อมต่อแล้ว")

@socketio.on('disconnect')
def handle_disconnect():
    print("⚠️ Client ตัดการเชื่อมต่อ")

# ==========================================
# 🚀 MAIN
# ==========================================

if __name__ == "__main__":
    print("\n" + "="*60)
    print("🚕 Bolt Taxi Tracker - Multi-location Concurrent Edition")
    print("="*60)
    print(f"📍 จำนวนตำแหน่ง: {len(LOCATIONS)}")
    print(f"⚡ Workers: {MAX_WORKERS}")
    print(f"🔄 Interval: {FETCH_INTERVAL} วินาที")
    print(f"🧪 Test Mode: {TEST_MODE}")
    print("="*60 + "\n")

    fetch_thread = threading.Thread(target=data_fetch_loop, daemon=True)
    fetch_thread.start()

    print("🌐 เริ่ม Flask-SocketIO server ที่ http://0.0.0.0:8000")
    print("💡 เปิดเบราว์เซอร์ไปที่: http://localhost:8000")
    print("🧪 ทดสอบ API: http://localhost:8000/test-api")
    print("📍 ดูตำแหน่งทั้งหมด: http://localhost:8000/locations")
    print("="*60 + "\n")

    socketio.run(app, debug=False, host="0.0.0.0", port=8000, allow_unsafe_werkzeug=True)
