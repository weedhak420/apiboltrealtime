# Bolt Realtime Map

This project provides a simple Flask and Socket.IO application that queries Bolt's realtime mobility API for multiple Chiang Mai locations and streams the results to the frontend.

## Environment configuration

The application requires several secrets to authenticate with Bolt's API. Copy `.env.example` to `.env` and populate it with the correct values:

```bash
cp .env.example .env
# edit .env with your credentials
```

At runtime the app reads the following environment variables:

| Variable | Description |
| --- | --- |
| `BOLT_BEARER_TOKEN` | OAuth bearer token for Bolt's API. |
| `BOLT_DEVICE_ID` | Device identifier reported to the Bolt API. |
| `BOLT_DEVICE_NAME` | Human readable device name sent with API requests. |
| `BOLT_DEVICE_OS_VERSION` | Operating system version reported to Bolt. |
| `BOLT_USER_ID` | Bolt user identifier associated with the session. |
| `BOLT_DISTINCT_ID` | Distinct identifier used in Bolt telemetry headers. |
| `BOLT_RH_SESSION_ID` | Session identifier supplied to Bolt's API. |
| `BOLT_CHANNEL` | (Optional) Distribution channel, defaults to `googleplay`. |
| `BOLT_BRAND` | (Optional) Brand sent to the API, defaults to `bolt`. |
| `BOLT_DEVICE_TYPE` | (Optional) Device type (e.g. `android`). |
| `BOLT_COUNTRY` | (Optional) Country code, defaults to `th`. |
| `BOLT_LANGUAGE` | (Optional) Language code, defaults to `th`. |

The application automatically loads values from a `.env` file if it exists, without overwriting variables that are already defined in the environment. This makes it convenient for local development while still supporting production environments where secrets are injected directly into the process environment (e.g., via deployment configuration, container secrets, or platform settings).

## Running locally

1. Create and populate the `.env` file as described above.
2. Install the Python dependencies (Flask, Flask-SocketIO, Requests, etc.) if you haven't already.
3. Start the server:
   ```bash
   python app.py
   ```

The app will fail fast with a descriptive error message if any required environment variables are missing.

## Deployment notes

* Never commit real secrets to the repository. `.env` is ignored via `.gitignore` and sensitive values must be supplied via environment variables in each deployment environment.
* Rotate any credentials that may have been exposed before this change and ensure the git history no longer contains the old secrets.
