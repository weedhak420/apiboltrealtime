import os
import json
import threading
import time
from concurrent.futures import ThreadPoolExecutor, as_completed
from datetime import datetime

import requests
from flask import Flask, render_template, jsonify
from flask_socketio import SocketIO


def load_env_file(env_path: str = ".env") -> None:
    """Load environment variables from a .env file if it exists."""

    if not os.path.exists(env_path):
        return

    with open(env_path, "r", encoding="utf-8") as env_file:
        for line in env_file:
            stripped = line.strip()
            if not stripped or stripped.startswith("#"):
                continue
            if "=" not in stripped:
                continue
            key, value = stripped.split("=", 1)
            os.environ.setdefault(key.strip(), value.strip())


def load_config():
    """Load required configuration from the environment."""

    required_vars = [
        "BOLT_BEARER_TOKEN",
        "BOLT_DEVICE_ID",
        "BOLT_DEVICE_NAME",
        "BOLT_DEVICE_OS_VERSION",
        "BOLT_USER_ID",
        "BOLT_DISTINCT_ID",
        "BOLT_RH_SESSION_ID",
    ]

    config = {}
    missing = []

    for var in required_vars:
        value = os.getenv(var)
        if value:
            config[var] = value
        else:
            missing.append(var)

    if missing:
        missing_list = ", ".join(missing)
        raise RuntimeError(
            "Missing required environment variables: "
            f"{missing_list}. Please define them before starting the app."
        )

    optional_defaults = {
        "BOLT_CHANNEL": "googleplay",
        "BOLT_BRAND": "bolt",
        "BOLT_DEVICE_TYPE": "android",
        "BOLT_COUNTRY": "th",
        "BOLT_LANGUAGE": "th",
    }

    for var, default in optional_defaults.items():
        config[var] = os.getenv(var, default)

    return config


load_env_file()
CONFIG = load_config()

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
# 🗺️ MULTIPLE LOCATIONS - ครอบคลุม Chiang Mai
# ==========================================

# กำหนดจุดต่างๆ รอบเชียงใหม่ เพื่อดึงข้อมูลจากหลายพื้นที่
LOCATIONS = [
    {"name": "City Center", "lat": 18.7883, "lng": 98.9853},
    {"name": "Old City", "lat": 18.7912, "lng": 98.9853},
    {"name": "Tha Phae Gate", "lat": 18.7868, "lng": 98.9931},
    {"name": "Nimman", "lat": 18.8002, "lng": 98.9679},
    {"name": "CMU Area", "lat": 18.8063, "lng": 98.9511},
    {"name": "Maya Mall", "lat": 18.8025, "lng": 98.9667},
    {"name": "Airport", "lat": 18.7667, "lng": 98.9625},
    {"name": "San Kamphaeng", "lat": 18.7500, "lng": 99.1167},
    {"name": "Hang Dong", "lat": 18.6833, "lng": 98.9167},
    {"name": "Doi Saket", "lat": 18.9167, "lng": 99.1667},
    {"name": "Mae Rim", "lat": 18.9167, "lng": 98.8833},
    {"name": "Doi Suthep", "lat": 18.8047, "lng": 98.9217},
    {"name": "San Sai", "lat": 18.8667, "lng": 99.0333},
    {"name": "Saraphi", "lat": 18.7167, "lng": 99.0167},
]

# ==========================================
# 🚀 CONCURRENT API FETCHING
# ==========================================

def fetch_single_location(location):
    try:
        lat = location["lat"]
        lng = location["lng"]
        base_url = "https://user.live.boltsvc.net/mobility/search/poll"

        params = {
            "version": "CA.180.0",
            "deviceId": CONFIG["BOLT_DEVICE_ID"],
            "device_name": CONFIG["BOLT_DEVICE_NAME"],
            "device_os_version": CONFIG["BOLT_DEVICE_OS_VERSION"],
            "channel": CONFIG["BOLT_CHANNEL"],
            "brand": CONFIG["BOLT_BRAND"],
            "deviceType": CONFIG["BOLT_DEVICE_TYPE"],
            "signup_session_id": "",
            "country": CONFIG["BOLT_COUNTRY"],
            "is_local_authentication_available": "false",
            "language": CONFIG["BOLT_LANGUAGE"],
            "gps_lat": str(lat),
            "gps_lng": str(lng),
            "gps_accuracy_m": "10.0",
            "gps_age": "0",
            "user_id": CONFIG["BOLT_USER_ID"],
            "session_id": f"{CONFIG['BOLT_USER_ID']}u{int(time.time())}",
            "distinct_id": CONFIG["BOLT_DISTINCT_ID"],
            "rh_session_id": CONFIG["BOLT_RH_SESSION_ID"]
        }

        headers = {
            "Host": "user.live.boltsvc.net",
            "Authorization": f"Bearer {CONFIG['BOLT_BEARER_TOKEN']}",
            "Content-Type": "application/json; charset=UTF-8",
            "Accept-Encoding": "gzip, deflate, br",
            "User-Agent": "okhttp/4.12.0"
        }

        viewport_offset = 0.018
        body = {
            "destination_stops": [],
            "payment_method": {"id": "cash", "type": "default"},
            "pickup_stop": {
                "lat": lat,
                "lng": lng,
                "address": location["name"],
                "place_id": f"custom|{location['name']}"
            },
            "stage": "overview",
            "viewport": {
                "north_east": {"lat": lat + viewport_offset, "lng": lng + viewport_offset},
                "south_west": {"lat": lat - viewport_offset, "lng": lng - viewport_offset}
            }
        }

        response = requests.post(base_url, params=params, headers=headers, json=body, timeout=5)

        if response.status_code == 200:
            return {"location": location["name"], "data": response.json(), "success": True}
        else:
            return {"location": location["name"], "data": None, "success": False, "error": f"Status {response.status_code}"}
    except Exception as e:
        return {"location": location["name"], "data": None, "success": False, "error": str(e)}

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
                    "source_location": response["location"]
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
        num_cars = random.randint(5, 15)
        for i in range(num_cars):
            lat_offset = random.uniform(-0.015, 0.015)
            lng_offset = random.uniform(-0.015, 0.015)
            vehicle = {
                "id": f"FAKE_{idx}_{i}",
                "lat": location["lat"] + lat_offset,
                "lng": location["lng"] + lng_offset,
                "bearing": random.randint(0, 359),
                "icon_url": "https://raw.githubusercontent.com/pointhi/leaflet-color-markers/master/img/marker-icon-2x-blue.png",
                "category_name": random.choice(["Economy", "Comfort", "XL"]),
                "category_id": "economy",
                "source_location": location["name"]
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
                "fetch_time": elapsed
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
            "vehicles_sample": vehicles[:5]
        })

    responses = fetch_all_locations()
    vehicles = process_all_responses(responses)
    elapsed = time.time() - start_time
    success_locations = [r["location"] for r in responses if r["success"]]
    failed_locations = [r["location"] for r in responses if not r["success"]]

    return jsonify({
        "success": True,
        "mode": "REAL",
        "vehicle_count": len(vehicles),
        "locations_count": len(LOCATIONS),
        "fetch_time": f"{elapsed:.2f}s",
        "success_locations": success_locations,
        "failed_locations": failed_locations,
        "vehicles_sample": vehicles[:5]
    })

@app.route("/locations")
def get_locations():
    return jsonify({
        "locations": LOCATIONS,
        "count": len(LOCATIONS)
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
