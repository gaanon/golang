# Go Radio API

A simple internet radio player with REST API for managing and playing radio stations.

## Installation

```bash
# Install dependencies
go get github.com/gorilla/mux
go get github.com/hajimehoshi/oto
go get github.com/hajimehoshi/go-mp3
go get github.com/google/uuid

# On macOS, install additional dependencies
brew install pkg-config
brew install portaudio
```

## API Documentation

### Base URL
```
http://localhost:8080/api
```

### Endpoints

#### Stations Management

##### List all stations
```
GET /stations
```
Response:
```json
{
    "stations": [
        {
            "id": "550e8400-e29b-41d4-a716-446655440000",
            "name": "Vermont Public Radio",
            "url": "https://vpr.streamguys1.com/vpr64.mp3",
            "icon": "https://example.com/vpr-icon.png"
        }
    ]
}
```

##### Add a new station
```
POST /stations
```
Request body:
```json
{
    "name": "Vermont Public Radio",
    "url": "https://vpr.streamguys1.com/vpr64.mp3",
    "icon": "https://example.com/vpr-icon.png"
}
```
Response: Returns the created station with generated ID.

##### Delete a station
```
DELETE /stations/{id}
```
Response: 204 No Content on success

##### Update a station
```
PUT /stations/{id}
```
Request body:
```json
{
    "name": "Updated Radio Name",
    "url": "https://example.com/stream.mp3",
    "icon": "https://example.com/new-icon.png"
}
```
Response: Returns the updated station

#### Playback Control

##### Start playing a station
```
POST /play
```
Request body:
```json
{
    "station_id": "550e8400-e29b-41d4-a716-446655440000"
}
```
Response:
```json
{
    "is_playing": true,
    "station": "Vermont Public Radio"
}
```

##### Stop playback
```
POST /stop
```
Response:
```json
{
    "is_playing": false,
    "station": ""
}
```

##### Get player status
```
GET /status
```
Response:
```json
{
    "is_playing": true,
    "station": "Vermont Public Radio"
}
```

### Example Usage with curl

1. Add a new station:
```bash
curl -X POST http://localhost:8080/api/stations \
  -H "Content-Type: application/json" \
  -d '{
    "name": "Vermont Public Radio",
    "url": "https://vpr.streamguys1.com/vpr64.mp3",
    "icon": "https://example.com/vpr-icon.png"
  }'
```

2. List all stations:
```bash
curl http://localhost:8080/api/stations
```

3. Play a station:
```bash
curl -X POST http://localhost:8080/api/play \
  -H "Content-Type: application/json" \
  -d '{"station_id": "550e8400-e29b-41d4-a716-446655440000"}'
```

4. Check player status:
```bash
curl http://localhost:8080/api/status
```

5. Stop playback:
```bash
curl -X POST http://localhost:8080/api/stop
```

6. Delete a station:
```bash
curl -X DELETE http://localhost:8080/api/stations/550e8400-e29b-41d4-a716-446655440000
```

7. Update a station:
```bash
curl -X PUT http://localhost:8080/api/stations/550e8400-e29b-41d4-a716-446655440000 \
  -H "Content-Type: application/json" \
  -d '{
    "name": "Updated Radio Name",
    "url": "https://example.com/stream.mp3",
    "icon": "https://example.com/new-icon.png"
  }'
```

### Error Responses
The API returns appropriate HTTP status codes:
- 200: Success
- 201: Created (for POST /stations)
- 204: No Content (for DELETE)
- 400: Bad Request
- 404: Not Found
- 500: Internal Server Error

Error responses include a message in the response body explaining the error.

## License

MIT License