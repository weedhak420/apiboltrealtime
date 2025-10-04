# Bolt Taxi Tracker

This project tracks Bolt taxi availability around Chiang Mai using a Flask + Socket.IO backend and a Leaflet frontend.

## Centralized configuration

All location metadata now lives in [`config.py`](config.py). Each entry in `LOCATIONS` contains:

```python
{
    "id": "nimman",
    "name": {"th": "นิมมาน", "en": "Nimman"},
    "district": "Mueang Chiang Mai",
    "type": "lifestyle",
    "priority": 1,
    "coordinates": {"lat": 18.8002, "lng": 98.9679},
}
```

Related constants exported from `config.py` include:

- `MAP_SETTINGS`: default center/zoom for the map.
- `VIEWPORT_PADDING`: viewport buffer used when calling the Bolt API.
- `DISTRICTS` and `LOCATION_TYPES`: derived lists that power frontend filters.

### Adding or updating a location

1. Edit `config.py` and add a new dictionary to `LOCATIONS` with Thai/English names, district, type, priority, and coordinates.
2. (Optional) Update `MAP_SETTINGS` if the default map center/zoom should change.
3. Restart the backend if it is already running.

The backend automatically exposes the latest data via the `/locations` endpoint and includes metadata in Socket.IO events. The frontend consumes this dataset to:

- Build district and location-type filters.
- Render bilingual names and district labels in vehicle popups.
- Apply map defaults supplied by the backend.

No additional frontend changes are required when adding new locations—refreshing the page is enough for the new entry to appear in filters and tooltips.

## Development

Run the application with:

```bash
python app.py
```

Open [http://localhost:8000](http://localhost:8000) to view the live map. Use the district and type selectors in the header to focus on specific areas of Chiang Mai.
