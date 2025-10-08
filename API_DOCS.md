# Bolt Taxi Tracker API Documentation

## Overview

This Go backend service provides real-time tracking and analytics for Bolt taxi vehicles in Chiang Mai, Thailand. The system features an advanced JWT token management system with automatic refresh capabilities, ensuring seamless API access to Bolt's services.

### Key Features
- **Real-time vehicle tracking** with WebSocket support
- **Automatic JWT token refresh** from Bolt API
- **Comprehensive analytics** with heatmaps, trends, and historical data
- **Performance monitoring** with circuit breakers and rate limiting
- **Database caching** with MySQL and Redis support

## JWT Token Management System

### Overview
The JWTManager handles the complete lifecycle of JWT tokens used for authenticating with Bolt's API. It automatically refreshes tokens before expiration, ensuring uninterrupted service.

### Token Refresh Process
The system automatically refreshes JWT tokens by calling the Bolt API:

**Endpoint:** `POST https://node.bolt.eu/user-auth/profile/auth/getAccessToken`

**Headers:**
```
Authorization: Bearer <current_token>
User-Agent: okhttp/4.12.0
```

**Query Parameters:**
| Parameter | Value | Description |
|-----------|-------|-------------|
| `version` | `CA.180.0` | API version |
| `deviceId` | `1363e778-8d4a-4fd3-9ebf-d35aea4fb533` | Device identifier |
| `device_name` | `samsungSM-X910N` | Device name |
| `device_os_version` | `9` | Android version |
| `channel` | `googleplay` | Distribution channel |
| `brand` | `bolt` | App brand |
| `deviceType` | `android` | Device type |
| `signup_session_id` | `` | Session ID |
| `country` | `th` | Country code |
| `is_local_authentication_available` | `false` | Local auth flag |
| `language` | `th` | Language code |
| `gps_lat` | `13.727949` | GPS latitude |
| `gps_lng` | `100.446442` | GPS longitude |
| `gps_accuracy_m` | `0.02` | GPS accuracy |
| `gps_age` | `1` | GPS age |
| `user_id` | `284215753` | User ID |
| `session_id` | `284215753u1759766688900` | Session ID |
| `distinct_id` | `client-284215753` | Distinct ID |
| `rh_session_id` | `284215753u1759767820` | RH Session ID |

### Auto-Renewal Process
1. **Background Process**: `StartAutoRenewal()` runs every 5 minutes
2. **Expiry Check**: If token expires in <10 minutes, triggers refresh
3. **API Call**: Calls Bolt API to get new token with rate limiting and retry logic
4. **Token Update**: Stores new token with proper expiry/issued times
5. **Fallback**: If refresh fails, uses existing token (may trigger 401 retry)

### Enhanced Error Handling & Rate Limiting

#### Rate Limiting
- **Concurrent Requests**: Maximum 5 concurrent API requests
- **Request Rate**: Limited to 5 requests per second
- **Queue Management**: FIFO queue for requests exceeding limits
- **Automatic Throttling**: Prevents API overuse and reduces error 1005 occurrences

#### Error Code Handling
- **Error 1005 (TOO_MANY_REQUESTS)**: Automatic retry with exponential backoff (500ms → 1s → 2s)
- **Error 210 (REFRESH_TOKEN_INVALID)**: Triggers token regeneration with fallback
- **HTTP 429**: Handled with retry logic and backoff
- **Network Errors**: Automatic retry with exponential backoff
- **Maximum Retries**: 3 attempts per request with increasing delays

#### Retry Logic
```go
// Exponential backoff for retries
backoff := []time.Duration{500 * time.Millisecond, 1 * time.Second, 2 * time.Second}

// Rate limiting middleware
jm.rateLimiter.Acquire()
defer jm.rateLimiter.Release()
```

### API Response Format
```json
{
  "code": 0,
  "message": "OK",
  "data": {
    "access_token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...",
    "expires_timestamp": 1759774202,
    "expires_in_seconds": 3600,
    "next_update_in_seconds": 3540,
    "next_update_give_up_timestamp": 1759774182
  }
}
```

## API Endpoints

### Authentication & Status

#### `GET /api/jwt/status`
Returns current JWT token information and status with enhanced error handling details.

**Response:**
```json
{
  "status": "active",
  "token_info": {
    "expires_at": "2025-01-01T12:00:00Z",
    "issued_at": "2025-01-01T11:00:00Z",
    "user_id": 284215753,
    "login_id": 605369720,
    "is_valid": true,
    "time_until_expiry": "45m30s"
  },
  "rate_limiting": {
    "max_concurrent": 5,
    "requests_per_second": 5,
    "current_requests": 2
  },
  "error_handling": {
    "retry_attempts": 0,
    "last_error": null,
    "error_1005_count": 0,
    "error_210_count": 0,
    "http_429_count": 0
  }
}
```

**Error Response:**
```json
{
  "status": "no_token",
  "message": "No JWT token available",
  "rate_limiting": {
    "max_concurrent": 5,
    "requests_per_second": 5,
    "current_requests": 0
  },
  "error_handling": {
    "retry_attempts": 3,
    "last_error": "TOO_MANY_REQUESTS",
    "error_1005_count": 2,
    "error_210_count": 0,
    "http_429_count": 1
  }
}
```

#### `GET /api/status`
Returns general API status and configuration.

**Response:**
```json
{
  "status": "running",
  "timestamp": "2025-01-01T12:00:00Z",
  "interval": "5s",
  "workers": 3
}
```

### Vehicle Data

#### `GET /api/vehicles/latest`
Returns the latest cached vehicle data with fallback logic.

**Response:**
```json
{
  "vehicles": [
    {
      "id": "4289043010",
      "lat": 18.7883,
      "lng": 98.9853,
      "bearing": 45.5,
      "icon_url": "https://images.bolt.eu/store/...",
      "category_name": "Bolt_Taxi",
      "category_id": "taxi",
      "source_location": "city_center",
      "timestamp": "2025-01-01T12:00:00Z",
      "distance": 150.5
    }
  ],
  "count": 1,
  "timestamp": "2025-01-01T12:00:00Z",
  "source": "Redis"
}
```

#### `GET /api/vehicles`
Alias for `/api/vehicles/latest`.

#### `GET /api/vehicles/history`
Returns historical vehicle records with optional filtering.

**Query Parameters:**
| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| `limit` | int | 100 | Maximum records to return (max 1000) |
| `vehicle_id` | string | - | Filter by specific vehicle ID |

**Response:**
```json
{
  "records": [
    {
      "history_id": 12345,
      "vehicle_id": "4289043010",
      "lat": 18.7883,
      "lng": 98.9853,
      "bearing": 45,
      "category_name": "Bolt_Taxi",
      "timestamp": "2025-01-01 12:00:00",
      "created_at": "2025-01-01 12:00:05"
    }
  ],
  "count": 1,
  "limit": 100,
  "vehicle_id": "",
  "timestamp": "2025-01-01T12:00:00Z"
}
```

### Analytics

#### `GET /api/analytics/heatmap`
Returns heatmap data showing vehicle concentration hotspots.

**Query Parameters:**
| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| `limit` | int | 50 | Maximum hotspots to return (max 500) |

**Response:**
```json
{
  "hotspots": [
    {
      "grid_lat": 18.790,
      "grid_lng": 98.985,
      "vehicles": 42
    }
  ],
  "count": 1,
  "limit": 50,
  "timestamp": "2025-01-01T12:00:00Z"
}
```

#### `GET /api/analytics/trend`
Returns trend data showing vehicle activity over time.

**Query Parameters:**
| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| `interval` | string | hour | Time interval: `hour` or `day` |

**Response:**
```json
{
  "trends": [
    {
      "time": "2025-01-01 12:00:00",
      "vehicles": 25,
      "smoothed": 24.5
    }
  ],
  "count": 1,
  "interval": "hour",
  "smoothing": "ema_0.3",
  "timestamp": "2025-01-01T12:00:00Z"
}
```

#### `GET /api/analytics/history`
Returns comprehensive analytics from vehicle history data.

**Query Parameters:**
| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| `start` | string | 24h ago | Start time (RFC3339) |
| `end` | string | now | End time (RFC3339) |
| `vehicle_id` | string | - | Filter by vehicle ID |
| `bbox` | string | - | Bounding box "minLat,minLng,maxLat,maxLng" |
| `limit` | int | 1000 | Max records (max 5000) |
| `offset` | int | 0 | Pagination offset |
| `grid` | float | 0.001 | Grid size for hotspots |
| `stop_min_sec` | int | 120 | Min dwell time for stops |
| `stop_max_move_m` | float | 10.0 | Max movement for stops |

**Response:**
```json
{
  "summary": {
    "start": "2025-01-01T00:00:00Z",
    "end": "2025-01-01T23:59:59Z",
    "record_count": 1234,
    "vehicle_count": 106,
    "total_distance_km": 87.52,
    "avg_speed_kmh": 26.4,
    "stop_count": 312
  },
  "bearing_histogram": {
    "N": 120, "NE": 98, "E": 140, "SE": 90,
    "S": 110, "SW": 80, "W": 100, "NW": 75
  },
  "hotspots": [
    {
      "grid_lat": 18.790,
      "grid_lng": 98.985,
      "vehicles": 42
    }
  ],
  "vehicles": [
    {
      "vehicle_id": "4289043010",
      "points": 124,
      "distance_km": 6.31,
      "avg_speed_kmh": 19.2,
      "stops": 5
    }
  ],
  "timestamp": "2025-01-01T12:00:00Z"
}
```

### System Monitoring

#### `GET /api/health`
Returns health check status.

**Response:**
```json
{
  "status": "healthy",
  "database": "connected",
  "redis": "connected",
  "api": "operational",
  "timestamp": "2025-01-01T12:00:00Z"
}
```

#### `GET /api/performance`
Returns performance statistics.

**Response:**
```json
{
  "goroutine_count": 15,
  "cache_size": 25,
  "rate_limit_size": 0,
  "enhanced_rate_limiter": {
    "requests_per_minute": 60,
    "current_requests": 5
  },
  "worker_pool": {
    "active_workers": 3,
    "queued_tasks": 0
  },
  "timestamp": "2025-01-01T12:00:00Z"
}
```

#### `GET /api/analytics`
Returns basic analytics data.

**Response:**
```json
{
  "total_vehicles": 25,
  "locations": 7,
  "timestamp": "2025-01-01T12:00:00Z"
}
```

## Error Handling

### Enhanced JWT Token Error Handling

#### Error 1005 (TOO_MANY_REQUESTS)
- **Automatic Retry**: System retries with exponential backoff (500ms → 1s → 2s)
- **Rate Limiting**: Prevents future 1005 errors by limiting concurrent requests
- **Maximum Attempts**: 3 retry attempts before giving up
- **Logging**: Detailed logs for monitoring and debugging

#### Error 210 (REFRESH_TOKEN_INVALID)
- **Token Regeneration**: Automatically triggers refresh token regeneration
- **Fallback Token**: Uses backup token if regeneration fails
- **Seamless Recovery**: System continues operating with new token
- **Logging**: Clear indication of token regeneration process

#### HTTP 429 (Too Many Requests)
- **Automatic Retry**: Handled with same retry logic as error 1005
- **Rate Limiting**: Prevents future 429 errors through request throttling
- **Backoff Strategy**: Exponential backoff with maximum 3 attempts

#### Network Errors
- **Connection Timeouts**: Automatic retry with backoff
- **DNS Resolution**: Retry with exponential backoff
- **Request Failures**: Comprehensive error handling and logging

### JWT Token Errors (Legacy)
- **401 Unauthorized**: Token expired or invalid
  - System automatically attempts refresh with retry logic
  - Falls back to cached token if refresh fails
- **Token Refresh Failure**: Network or API errors
  - Enhanced retry logic with exponential backoff
  - Comprehensive error logging and monitoring

### API Error Responses
All endpoints return consistent error format:
```json
{
  "error": "Error description",
  "code": 500
}
```

### Common HTTP Status Codes
- `200`: Success
- `400`: Bad Request (invalid parameters)
- `401`: Unauthorized (JWT token issues)
- `500`: Internal Server Error

## WebSocket Support

### Real-time Updates
The system provides WebSocket endpoints for real-time vehicle updates:

**Connection:** `ws://localhost:8000/socket.io/`

**Events:**
- `vehicles_update`: Broadcasts latest vehicle data
- `connect`: Client connection established
- `disconnect`: Client disconnected

**WebSocket Data Format:**
```json
{
  "vehicles": [...],
  "count": 25,
  "timestamp": "2025-01-01T12:00:00Z",
  "fetch_time": 1735732800,
  "api_status": "success"
}
```

## Configuration

### Environment Variables
- `DB_HOST`: Database host (default: localhost)
- `DB_PORT`: Database port (default: 3306)
- `DB_USER`: Database username (default: root)
- `DB_PASSWORD`: Database password
- `DB_NAME`: Database name (default: bolt_tracker)

### Server Configuration
- **Port**: 8000
- **Workers**: 3 concurrent API workers
- **Fetch Interval**: 5 seconds
- **Rate Limiting**: 60 requests/minute per location
- **Cache TTL**: 5 minutes for vehicle data

## Development & Testing

### Running the Server
```bash
go run .
```

### Testing JWT Status
```bash
curl http://localhost:8000/api/jwt/status
```

### Testing Vehicle Data
```bash
curl http://localhost:8000/api/vehicles/latest
```

### Enhanced Monitoring & Auto-Refresh
Watch server logs for JWT refresh activity with enhanced error handling:
```
🔄 JWT Auto-renewal started (checking every 5 minutes)
⏰ JWT token still valid for 45m (no renewal needed)
🔄 Auto-renewing JWT token...
✅ Successfully refreshed JWT token (expires: 2025-01-01T12:00:00Z)
```

#### Enhanced Error Monitoring
```
⚠️ TOO_MANY_REQUESTS (code 1005) → retry with backoff (attempt 1)
⚠️ HTTP 429 TOO_MANY_REQUESTS → retry with backoff (attempt 2)
⚠️ REFRESH_TOKEN_INVALID (code 210) → regenerate token
🔄 Regenerating refresh token due to error 210...
✅ Using regenerated fallback JWT token (expires: 2025-01-01T12:00:00Z)
```

#### Rate Limiting Monitoring
```
🔒 Rate limiter acquired (current: 3/5 concurrent requests)
🔒 Rate limiter released (current: 2/5 concurrent requests)
⏱️ Request throttled: 200ms delay applied
```

## Security Considerations

- JWT tokens are automatically refreshed to prevent expiration
- Rate limiting prevents API abuse
- Circuit breakers protect against API failures
- Database connections use connection pooling
- Redis caching reduces API load
- All sensitive data is logged with appropriate levels

## Performance Features

### Enhanced Rate Limiting & Throttling
- **Concurrent Request Limiting**: Maximum 5 concurrent API requests
- **Request Rate Throttling**: Limited to 5 requests per second
- **Queue Management**: FIFO queue for requests exceeding limits
- **Automatic Backoff**: Prevents API overuse and reduces error rates
- **Token Bucket Algorithm**: Efficient rate limiting implementation

### JWT Token Management
- **Automatic Renewal**: Background token refresh every 5 minutes
- **Error Recovery**: Comprehensive retry logic for all error types
- **Fallback Tokens**: Multiple fallback strategies for token failures
- **Rate-Limited Requests**: All API calls respect rate limits
- **Exponential Backoff**: Smart retry strategy for failed requests

### System Performance
- **Connection Pooling**: Optimized database connections
- **Memory Pools**: Reusable object pools for better performance
- **Circuit Breakers**: Automatic failure detection and recovery
- **Caching**: Multi-layer caching (Redis + Database + Memory)
- **Background Processing**: Non-blocking operations
- **Graceful Shutdown**: Clean service termination

### Monitoring & Logging
- **Detailed Error Logging**: Comprehensive logs for all error types
- **Performance Metrics**: Request timing and success rates
- **Rate Limit Monitoring**: Track API usage and throttling
- **Token Status Tracking**: Monitor JWT token lifecycle
- **Retry Attempt Logging**: Track retry attempts and outcomes
