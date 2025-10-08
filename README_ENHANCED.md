# 🚀 Bolt Tracker API - Enhanced Edition

A high-performance, scalable Go backend for real-time vehicle tracking with comprehensive monitoring, security, and observability features.

## ✨ Key Features

### 🔧 Core Functionality
- **Real-time Vehicle Tracking**: Track vehicles across 95+ locations in Chiang Mai
- **Concurrent Data Fetching**: Batch processing with dynamic worker scaling
- **Multi-layer Caching**: Redis + In-memory + Database caching
- **Analytics Engine**: Heatmaps, trends, and historical analysis

### 🛡️ Security & Authentication
- **Enhanced JWT Management**: Key rotation, token revocation, configurable expiration
- **Rate Limiting**: Per-user, per-IP, and per-endpoint limits with distributed support
- **CORS Protection**: Configurable cross-origin resource sharing
- **Input Validation**: Comprehensive request validation and sanitization

### 📊 Observability & Monitoring
- **Structured Logging**: JSON logs with correlation IDs and context
- **Prometheus Metrics**: Comprehensive metrics collection and export
- **OpenTelemetry Tracing**: Distributed tracing with Jaeger integration
- **Health Checks**: Liveness, readiness, and dependency health monitoring

### ⚡ Performance & Scalability
- **Dynamic Worker Pool**: Auto-scaling based on load with backpressure
- **Circuit Breakers**: Fault tolerance for external dependencies
- **Connection Pooling**: Optimized database and Redis connections
- **Batch Processing**: Efficient handling of large datasets

### 🔄 Reliability & Operations
- **Graceful Shutdown**: Clean shutdown with context cancellation
- **Hot Configuration**: Runtime configuration updates without restart
- **Environment Variables**: 12-factor app compliance
- **Docker Support**: Production-ready containerization

## 🏗️ Architecture

```
┌─────────────────┐    ┌─────────────────┐    ┌─────────────────┐
│   Load Balancer │    │   API Gateway   │    │   Bolt Tracker  │
│     (Nginx)     │───▶│   (Optional)    │───▶│     Service     │
└─────────────────┘    └─────────────────┘    └─────────────────┘
                                                        │
                       ┌─────────────────────────────────┼─────────────────────────────────┐
                       │                                 │                                 │
                       ▼                                 ▼                                 ▼
              ┌─────────────────┐              ┌─────────────────┐              ┌─────────────────┐
              │   MySQL DB      │              │   Redis Cache   │              │   Worker Pool   │
              │   (Primary)     │              │   (Sessions)    │              │   (Dynamic)     │
              └─────────────────┘              └─────────────────┘              └─────────────────┘
                       │                                 │                                 │
                       └─────────────────────────────────┼─────────────────────────────────┘
                                                         │
                                                         ▼
                                              ┌─────────────────┐
                                              │   Monitoring    │
                                              │   Stack         │
                                              │   (Prometheus,  │
                                              │    Grafana,     │
                                              │    Jaeger)      │
                                              └─────────────────┘
```

## 🚀 Quick Start

### Prerequisites
- Go 1.22+
- MySQL 8.0+
- Redis 7.0+
- Docker & Docker Compose (optional)

### Development Setup

1. **Clone and Setup**
   ```bash
   git clone <repository>
   cd bolt-tracker
   make deps
   ```

2. **Configure Environment**
   ```bash
   cp config.json.example config.json
   # Edit config.json with your settings
   ```

3. **Start Dependencies**
   ```bash
   # Using Docker Compose
   make compose-up
   
   # Or manually
   mysql -u root -p < init.sql
   redis-server
   ```

4. **Run Application**
   ```bash
   # Development mode with hot reload
   make dev
   
   # Or build and run
   make build && make run
   ```

### Production Deployment

1. **Docker Deployment**
   ```bash
   # Build and run
   make docker-build
   make docker-run
   
   # Or use Docker Compose
   docker-compose -f docker-compose.prod.yml up -d
   ```

2. **Kubernetes Deployment**
   ```bash
   kubectl apply -f k8s/
   ```

## 📋 Configuration

### Environment Variables

The application supports comprehensive environment variable configuration:

```bash
# Server Configuration
BOLT_SERVER_PORT=:8000
BOLT_SERVER_READ_TIMEOUT=30s
BOLT_SERVER_WRITE_TIMEOUT=30s

# Database Configuration
BOLT_DATABASE_HOST=localhost
BOLT_DATABASE_PORT=3306
BOLT_DATABASE_USER=root
BOLT_DATABASE_PASSWORD=password
BOLT_DATABASE_NAME=bolt_tracker

# Redis Configuration
BOLT_REDIS_HOST=localhost
BOLT_REDIS_PORT=6379
BOLT_REDIS_PASSWORD=
BOLT_REDIS_DB=0

# JWT Configuration
BOLT_JWT_ACCESS_TOKEN_EXPIRY=15m
BOLT_JWT_REFRESH_TOKEN_EXPIRY=7d
BOLT_JWT_KEY_ROTATION_ENABLED=true
BOLT_JWT_REVOCATION_STORE=redis

# Rate Limiting
BOLT_RATE_LIMITING_GLOBAL_LIMIT=1000
BOLT_RATE_LIMITING_PER_USER_LIMIT=100
BOLT_RATE_LIMITING_PER_IP_LIMIT=200
BOLT_RATE_LIMITING_DISTRIBUTED=true

# Observability
BOLT_OBSERVABILITY_STRUCTURED_LOGGING=true
BOLT_OBSERVABILITY_LOG_FORMAT=json
BOLT_OBSERVABILITY_CORRELATION_ID=true
BOLT_MONITORING_ENABLE_PROMETHEUS=true
BOLT_MONITORING_PROMETHEUS_PORT=:9090
```

### Configuration File

The `config.json` file supports hot reloading:

```json
{
  "server": {
    "port": ":8000",
    "read_timeout": "30s",
    "write_timeout": "30s",
    "idle_timeout": "120s",
    "graceful_shutdown_timeout": "30s"
  },
  "database": {
    "host": "localhost",
    "port": "3306",
    "user": "root",
    "password": "",
    "name": "bolt_tracker",
    "max_connections": 25,
    "max_idle_connections": 5,
    "connection_lifetime": "5m",
    "retry_attempts": 3,
    "retry_delay": "1s"
  },
  "redis": {
    "host": "localhost",
    "port": "6379",
    "password": "",
    "db": 0,
    "pool_size": 10,
    "min_idle_connections": 5,
    "dial_timeout": "5s",
    "read_timeout": "3s",
    "write_timeout": "3s",
    "max_retries": 3,
    "retry_backoff": "1s",
    "namespace": "bolt_tracker",
    "circuit_breaker": true
  },
  "jwt": {
    "access_token_expiry": "15m",
    "refresh_token_expiry": "7d",
    "key_rotation_enabled": true,
    "revocation_store": "redis",
    "issuer": "bolt-tracker",
    "audience": "bolt-api"
  },
  "rate_limiting": {
    "global_limit": 1000,
    "per_user_limit": 100,
    "per_ip_limit": 200,
    "window": "1m",
    "distributed": false,
    "storage_backend": "memory"
  },
  "observability": {
    "structured_logging": true,
    "log_format": "json",
    "correlation_id": true,
    "enable_metrics": true,
    "enable_tracing": false,
    "service_name": "bolt-tracker",
    "service_version": "1.0.0"
  }
}
```

## 🔌 API Endpoints

### Health & Monitoring
- `GET /healthz` - Health check
- `GET /readyz` - Readiness check
- `GET /livez` - Liveness check
- `GET /metrics` - Prometheus metrics

### JWT Management
- `GET /api/jwt/status` - JWT token status
- `POST /api/jwt/refresh` - Refresh token
- `POST /api/jwt/revoke` - Revoke token
- `GET /api/jwt/keys` - Public keys for verification

### Vehicle Tracking
- `GET /api/vehicles/latest` - Latest vehicle positions
- `GET /api/vehicles/history` - Historical vehicle data

### Analytics
- `GET /api/analytics/heatmap` - Vehicle density heatmap
- `GET /api/analytics/trend` - Usage trends
- `GET /api/analytics/history` - Historical analytics

### Worker Pool Management
- `GET /api/workers/status` - Worker pool status
- `GET /api/workers/metrics` - Worker pool metrics

## 📊 Monitoring & Observability

### Metrics (Prometheus)

The application exposes comprehensive metrics:

- **HTTP Metrics**: Request count, duration, status codes
- **Database Metrics**: Connection pool, query duration, errors
- **Redis Metrics**: Operations, cache hits/misses, connection pool
- **Worker Pool Metrics**: Active workers, queued jobs, processing time
- **Rate Limiting Metrics**: Hits, rejects, limits
- **Circuit Breaker Metrics**: State changes, failures
- **Business Metrics**: Vehicles fetched, locations processed

### Logging

Structured JSON logging with correlation IDs:

```json
{
  "timestamp": "2025-01-01T12:00:00Z",
  "level": "info",
  "message": "HTTP request completed",
  "service": "bolt-tracker",
  "version": "1.0.0",
  "correlation_id": "abc123",
  "method": "GET",
  "path": "/api/vehicles/latest",
  "status_code": 200,
  "duration_ms": 45
}
```

### Tracing

OpenTelemetry integration with Jaeger:

- **Distributed Tracing**: Track requests across services
- **Performance Analysis**: Identify bottlenecks
- **Error Tracking**: Trace error propagation

## 🛠️ Development

### Code Quality

```bash
# Run tests
make test

# Run linter
make lint

# Format code
make fmt

# Security scan
make security-scan
```

### Build & Deploy

```bash
# Build for development
make build

# Build for production
make build-all

# Docker build
make docker-build

# Deploy with Docker Compose
make compose-up
```

### Database Management

```bash
# Run migrations
make db-migrate

# Seed test data
make db-seed
```

## 🐳 Docker Support

### Development
```bash
# Start all services
docker-compose up -d

# View logs
docker-compose logs -f

# Stop services
docker-compose down
```

### Production
```bash
# Build optimized image
docker build -t bolt-tracker:latest .

# Run with production config
docker run -d \
  --name bolt-tracker \
  -p 8000:8000 \
  -p 9090:9090 \
  -e BOLT_DATABASE_HOST=mysql \
  -e BOLT_REDIS_HOST=redis \
  bolt-tracker:latest
```

## 📈 Performance Tuning

### Database Optimization
- Connection pooling
- Query optimization
- Index optimization
- Connection lifetime management

### Redis Optimization
- Memory optimization
- Connection pooling
- Circuit breaker
- Retry policies

### Worker Pool Tuning
- Dynamic scaling
- Backpressure handling
- Priority queues
- Metrics collection

## 🔒 Security Best Practices

### JWT Security
- Key rotation
- Token revocation
- Short-lived access tokens
- Secure refresh token flow

### Rate Limiting
- Per-user limits
- Per-IP limits
- Distributed rate limiting
- Graceful degradation

### Input Validation
- Request validation
- SQL injection prevention
- XSS protection
- CORS configuration

## 🚨 Troubleshooting

### Common Issues

1. **High Memory Usage**
   - Check worker pool scaling
   - Monitor cache size
   - Review connection pools

2. **Rate Limiting Issues**
   - Adjust rate limits
   - Check distributed configuration
   - Monitor Redis connectivity

3. **Database Connection Issues**
   - Check connection pool settings
   - Monitor connection lifetime
   - Review retry policies

### Debugging

```bash
# Enable debug logging
export BOLT_MONITORING_LOG_LEVEL=debug

# View detailed metrics
curl http://localhost:9090/metrics

# Check health status
curl http://localhost:8000/healthz
```

## 📚 Additional Resources

- [API Documentation](API_DOCS.md)
- [Configuration Reference](docs/configuration.md)
- [Deployment Guide](docs/deployment.md)
- [Monitoring Guide](docs/monitoring.md)
- [Security Guide](docs/security.md)

## 🤝 Contributing

1. Fork the repository
2. Create a feature branch
3. Make your changes
4. Run tests and linting
5. Submit a pull request

## 📄 License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.

## 🙏 Acknowledgments

- Go community for excellent libraries
- Prometheus for metrics collection
- OpenTelemetry for distributed tracing
- Docker for containerization
- All contributors and users
