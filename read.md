# Bolt Real-time Vehicle Tracker UI

## Project Overview
สร้าง Web UI แบบ Real-time สำหรับติดตามตำแหน่งรถ Bolt ในพื้นที่เชียงใหม่ พร้อมฟีเจอร์ปรับเปลี่ยนพิกัดและดูข้อมูลรถแบบสด

## Features Required

### 1. **Real-time Map Display**
- แสดงแผนที่ Google Maps หรือ Leaflet
- แสดงตำแหน่งรถทั้งหมดจาก API response
- ใช้ไอคอนตามประเภทรถ (icon_url จาก API)
- แสดงทิศทางรถด้วย bearing
- Auto-refresh ทุก 2 วินาที (ตาม poll_interval_sec)

### 2. **Vehicle Categories Panel**
แสดงข้อมูลรถแต่ละประเภท:
- **Send Motorbike** (13591) - 15 คัน
- **XL** (13592) - 5 คัน
- **Bolt** (13593) - 14 คัน
- **Motorbike** (13595) - 3 คัน
- **City Ride** (13596) - 11 คัน

แต่ละ category แสดง:
- จำนวนรถที่มี
- สีหรือไอคอนประจำประเภท
- สามารถคลิกเพื่อ filter แสดงเฉพาะประเภทนั้น

### 3. **Location Control Panel**
ฟอร์มสำหรับปรับเปลี่ยง pickup location:
```
Pickup Location:
- Latitude: [18.756651] (input field)
- Longitude: [98.994667] (input field)
- Address: [135 ซอย หมู่บ้านในฝัน] (input field)

[Update Location Button]
```

### 4. **Statistics Dashboard**
แสดงข้อมูลสถิติ:
- จำนวนรถทั้งหมด
- จำนวนรถแต่ละประเภท
- รถที่ใกล้ที่สุด (คำนวณระยะทาง)
- Last update timestamp
- Connection status (Connected/Disconnected)

### 5. **Vehicle Details Popup**
เมื่อคลิกที่รถบนแผนที่แสดง:
- Vehicle ID
- ประเภทรถ
- พิกัด (lat, lng)
- ทิศทาง (bearing)
- ระยะทางจาก pickup point

## Technical Stack

### Frontend
- **HTML/CSS/JavaScript** (Vanilla หรือ React)
- **Leaflet.js** หรือ **Google Maps API** สำหรับแผนที่
- **Tailwind CSS** สำหรับ styling
- **Fetch API** สำหรับเรียก backend

### Backend (Python)
```python
from flask import Flask, request, jsonify
from flask_cors import CORS
import requests

app = Flask(__name__)
CORS(app)

@app.route('/api/vehicles', methods=['POST'])
def get_vehicles():
    data = request.json
    
    # Extract parameters
    lat = data.get('lat', 18.756651)
    lng = data.get('lng', 98.994667)
    address = data.get('address', '135 ซอย หมู่บ้านในฝัน')
    
    # Build Bolt API request
    base_url = "https://user.live.boltsvc.net/mobility/search/poll"
    params = {
        "version": "CA.180.0",
        "deviceId": "ffac2e78-84c8-403d-b34e-8394499d7c29",
        # ... (other params)
        "gps_lat": str(lat),
        "gps_lng": str(lng),
    }
    
    headers = {
        "Authorization": "Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJkYXRhIjp7InVzZXJfaWQiOjI4MzYxNzQ5NSwidXNlcl9sb2dpbl9pZCI6NjAzNzMzNTg3fSwiaWF0IjoxNzU5NTA3MDQ2LCJleHAiOjE3NTk1MTA2NDZ9.LmStjqFCy18BJlJh13jJGPUVjcENqYdmU5RnpF3Ijo8",
        "Content-Type": "application/json"
    }
    
    body = {
        "destination_stops": [],
        "payment_method": {"id": "cash", "type": "default"},
        "pickup_stop": {
            "lat": lat,
            "lng": lng,
            "address": address,
            "place_id": data.get('place_id', 'google|ChIJwSgfJj4w2jAR_72NE5V00bA')
        },
        "stage": "overview",
        "viewport": {
            "north_east": {"lat": lat + 0.02, "lng": lng + 0.02},
            "south_west": {"lat": lat - 0.02, "lng": lng - 0.02}
        }
    }
    
    response = requests.post(base_url, params=params, headers=headers, json=body)
    return jsonify(response.json())

if __name__ == '__main__':
    app.run(debug=True, port=5000)
```

## UI Design Specifications

### Layout
```
┌─────────────────────────────────────────────┐
│  🚗 Bolt Real-time Tracker                  │
├──────────┬──────────────────────────────────┤
│          │                                  │
│ Controls │         Map Area                 │
│  Panel   │    (with vehicle markers)        │
│          │                                  │
│  Stats   │                                  │
│          │                                  │
├──────────┴──────────────────────────────────┤
│  Status Bar: Last update: 16:45:50 | 48 🚗 │
└─────────────────────────────────────────────┘
```

### Color Scheme
- **Primary**: #34D399 (Bolt green)
- **Background**: #1F2937 (Dark)
- **Cards**: #374151 (Gray)
- **Text**: #F9FAFB (White)
- **Accent**: #60A5FA (Blue)

### Vehicle Icon Colors
- Send Motorbike: 🟢 Green
- XL: 🔵 Blue
- Bolt: 🟣 Purple
- Motorbike: 🟡 Yellow
- City Ride: 🟠 Orange

## Key Functions

### 1. Calculate Distance
```javascript
function calculateDistance(lat1, lon1, lat2, lon2) {
    const R = 6371; // km
    const dLat = (lat2 - lat1) * Math.PI / 180;
    const dLon = (lon2 - lon1) * Math.PI / 180;
    const a = Math.sin(dLat/2) * Math.sin(dLat/2) +
              Math.cos(lat1 * Math.PI / 180) * Math.cos(lat2 * Math.PI / 180) *
              Math.sin(dLon/2) * Math.sin(dLon/2);
    const c = 2 * Math.atan2(Math.sqrt(a), Math.sqrt(1-a));
    return R * c;
}
```

### 2. Rotate Vehicle Icon
```javascript
function rotateIcon(bearing) {
    return `transform: rotate(${bearing}deg)`;
}
```

### 3. Auto Refresh
```javascript
setInterval(() => {
    fetchVehicles();
}, 2000); // Every 2 seconds
```

## Sample API Response Structure
```json
{
  "vehicles": {
    "taxi": {
      "13591": [
        {
          "id": "1660285396",
          "lat": 18.772542,
          "lng": 98.979779,
          "bearing": 278.77,
          "icon_id": "13591"
        }
      ]
    }
  },
  "category_details": {
    "taxi": {
      "13591": {
        "name": "Send Motorbike",
        "group": "delivery_motorbike"
      }
    }
  }
}
```

## Implementation Steps

1. **Setup Flask backend** with CORS
2. **Create HTML structure** with map container
3. **Initialize Leaflet/Google Maps**
4. **Implement API fetch** with location parameters
5. **Parse and display vehicles** on map
6. **Add category filters** and statistics
7. **Implement auto-refresh** mechanism
8. **Add location update form**
9. **Style with Tailwind CSS**
10. **Add animations** for smooth updates

## Performance Optimization
- Cache vehicle icons
- Use requestAnimationFrame for smooth marker updates
- Debounce location input changes
- Implement virtual scrolling for large vehicle lists

## Responsive Design
- Desktop: Side panel + map
- Tablet: Collapsible panel
- Mobile: Bottom sheet + full map

## Additional Features (Optional)
- 🔔 Notifications when vehicle nearby
- 📊 Charts for vehicle availability trends
- 🗺️ Route drawing from pickup to destination
- 🎯 Nearest vehicle highlighting
- 📍 Multiple pickup locations
- 💾 Save favorite locations