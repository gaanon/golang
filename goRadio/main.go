package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/mux"
	"github.com/hajimehoshi/go-mp3"
	"github.com/hajimehoshi/oto"
)

// RadioStation represents a streaming radio station
type RadioStation struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	URL  string `json:"url"`
	Icon string `json:"icon,omitempty"`
}

// StationRequest for the play endpoint
type StationRequest struct {
	StationID string `json:"station_id"`
}

// StationList represents a list of radio stations
type StationList struct {
	Stations []RadioStation `json:"stations"`
}

// RadioPlayer manages the playback state
type RadioPlayer struct {
	context    *oto.Context
	player     *oto.Player
	isPlaying  bool
	streamURL  string
	mutex      sync.Mutex
	stopSignal chan struct{}
}

// PlayerStatus represents the current state of the player
type PlayerStatus struct {
	IsPlaying bool   `json:"is_playing"`
	Station   string `json:"station"`
}

var radioPlayer *RadioPlayer

func NewRadioPlayer() *RadioPlayer {
	return &RadioPlayer{
		stopSignal: make(chan struct{}),
	}
}

func (rp *RadioPlayer) Start(url string) error {
	rp.mutex.Lock()
	defer rp.mutex.Unlock()

	if rp.isPlaying {
		return fmt.Errorf("already playing")
	}

	rp.streamURL = url
	rp.isPlaying = true
	rp.stopSignal = make(chan struct{})

	go rp.playStream(url, 5, 5*time.Second)
	return nil
}

func (rp *RadioPlayer) Stop() error {
	rp.mutex.Lock()
	defer rp.mutex.Unlock()

	if !rp.isPlaying {
		return fmt.Errorf("not playing")
	}

	close(rp.stopSignal)
	rp.isPlaying = false
	return nil
}

func (rp *RadioPlayer) playStream(streamURL string, maxRetries int, retryDelay time.Duration) {
	var context *oto.Context
	var player *oto.Player

	for attempts := 0; attempts <= maxRetries; attempts++ {
		if attempts > 0 {
			fmt.Printf("Attempting to reconnect... (attempt %d/%d)\n", attempts, maxRetries)
			time.Sleep(retryDelay)
		}

		// Create stream
		stream, err := createStream(streamURL)
		if err != nil {
			fmt.Printf("Connection error: %v\n", err)
			continue
		}
		defer stream.Close()

		// Initialize decoder
		decoder, err := mp3.NewDecoder(stream)
		if err != nil {
			fmt.Printf("Decoder error: %v\n", err)
			continue
		}

		// Initialize audio context and player if not already done
		if context == nil {
			context, err = oto.NewContext(decoder.SampleRate(), 2, 2, 4096)
			if err != nil {
				fmt.Printf("Context error: %v\n", err)
				continue
			}
			defer context.Close()

			player = context.NewPlayer()
			defer player.Close()
		}

		fmt.Println("Starting playback...")
		buf := make([]byte, 4096)

		// Play stream with error recovery and shutdown handling
		for {
			select {
			case <-rp.stopSignal:
				fmt.Println("Stopping playback...")
				return
			default:
				n, err := decoder.Read(buf)
				if err == io.EOF {
					break
				}
				if err != nil {
					fmt.Printf("Stream error: %v\n", err)
					break
				}

				if n > 0 {
					_, err := player.Write(buf[:n])
					if err != nil {
						fmt.Printf("Playback error: %v\n", err)
						break
					}
				}
			}
		}
	}

	// Reset player state if we've exhausted all retries
	rp.mutex.Lock()
	rp.isPlaying = false
	rp.mutex.Unlock()
	fmt.Println("Playback stopped after exhausting all retries")
}

// Creates an HTTP stream from the given URL
func createStream(url string) (io.ReadCloser, error) {
	resp, err := http.Get(url)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to stream: %v", err)
	}

	if resp.StatusCode != http.StatusOK {
		resp.Body.Close()
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	return resp.Body, nil
}

func loadStations() (*StationList, error) {
	data, err := os.ReadFile("stations.json")
	if err != nil {
		if os.IsNotExist(err) {
			return &StationList{Stations: []RadioStation{}}, nil
		}
		return nil, err
	}

	var stations StationList
	if err := json.Unmarshal(data, &stations); err != nil {
		return nil, err
	}
	return &stations, nil
}

func saveStations(stations *StationList) error {
	data, err := json.MarshalIndent(stations, "", "    ")
	if err != nil {
		return err
	}
	return os.WriteFile("stations.json", data, 0644)
}

// Find a station by its ID
func findStationByID(id string) (*RadioStation, error) {
	stations, err := loadStations()
	if err != nil {
		return nil, err
	}

	for _, station := range stations.Stations {
		if station.ID == id {
			return &station, nil
		}
	}
	return nil, fmt.Errorf("station not found")
}

// HTTP Handlers
func handleStart(w http.ResponseWriter, r *http.Request) {
	var req StationRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	station, err := findStationByID(req.StationID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	if err := radioPlayer.Start(station.URL); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	json.NewEncoder(w).Encode(PlayerStatus{
		IsPlaying: true,
		Station:   station.Name,
	})
}

func handleStop(w http.ResponseWriter, r *http.Request) {
	if err := radioPlayer.Stop(); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	json.NewEncoder(w).Encode(PlayerStatus{
		IsPlaying: false,
		Station:   "",
	})
}

func handleStatus(w http.ResponseWriter, r *http.Request) {
	json.NewEncoder(w).Encode(PlayerStatus{
		IsPlaying: radioPlayer.isPlaying,
		Station:   radioPlayer.streamURL,
	})
}

func handleAddStation(w http.ResponseWriter, r *http.Request) {
	var station RadioStation
	if err := json.NewDecoder(r.Body).Decode(&station); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Generate ID if not provided
	if station.ID == "" {
		station.ID = uuid.New().String()
	}

	stations, err := loadStations()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	stations.Stations = append(stations.Stations, station)
	if err := saveStations(stations); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(station)
}

func handleGetStations(w http.ResponseWriter, r *http.Request) {
	stations, err := loadStations()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	json.NewEncoder(w).Encode(stations)
}

func handleDeleteStation(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id := vars["id"]

	stations, err := loadStations()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	for i, station := range stations.Stations {
		if station.ID == id {
			stations.Stations = append(stations.Stations[:i], stations.Stations[i+1:]...)
			if err := saveStations(stations); err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			w.WriteHeader(http.StatusNoContent)
			return
		}
	}

	http.Error(w, "station not found", http.StatusNotFound)
}

func handleUpdateStation(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id := vars["id"]

	var updatedStation RadioStation
	if err := json.NewDecoder(r.Body).Decode(&updatedStation); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	stations, err := loadStations()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	found := false
	for i, station := range stations.Stations {
		if station.ID == id {
			// Preserve the original ID
			updatedStation.ID = id
			stations.Stations[i] = updatedStation
			found = true
			break
		}
	}

	if !found {
		http.Error(w, "station not found", http.StatusNotFound)
		return
	}

	if err := saveStations(stations); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	json.NewEncoder(w).Encode(updatedStation)
}

func main() {
	radioPlayer = NewRadioPlayer()
	router := mux.NewRouter()

	// API endpoints
	router.HandleFunc("/api/play", handleStart).Methods("POST")
	router.HandleFunc("/api/stop", handleStop).Methods("POST")
	router.HandleFunc("/api/status", handleStatus).Methods("GET")
	router.HandleFunc("/api/stations", handleGetStations).Methods("GET")
	router.HandleFunc("/api/stations", handleAddStation).Methods("POST")
	router.HandleFunc("/api/stations/{id}", handleDeleteStation).Methods("DELETE")
	router.HandleFunc("/api/stations/{id}", handleUpdateStation).Methods("PUT")

	fmt.Println("Starting server on :8080...")
	http.ListenAndServe(":8080", router)
}
