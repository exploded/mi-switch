package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/mux"
)

const (
	DeviceType    = "switch"
	DeviceNumber  = 0
	DiscoveryPort  = 32227 // UDP discovery port (ASCOM standard)
	APIPort        = 5555  // HTTP API port (different from discovery for flexibility)
	MaxSwitch     = 6
	DriverName    = "ASCOM Alpaca Switch Driver"
	DriverVersion = "1.0.0"
	Description   = "ASCOM Alpaca 6-channel Switch Driver"
)

// AlpacaResponse represents the common response structure for all Alpaca API calls
type AlpacaResponse struct {
	ClientTransactionID uint32      `json:"ClientTransactionID"`
	ServerTransactionID uint32      `json:"ServerTransactionID"`
	ErrorNumber         int         `json:"ErrorNumber"`
	ErrorMessage        string      `json:"ErrorMessage"`
	Value               interface{} `json:"Value,omitempty"`
}

// SwitchDevice represents a single switch with its properties
type SwitchDevice struct {
	Name        string
	Description string
	State       bool
	Value       float64
	MinValue    float64
	MaxValue    float64
	CanWrite    bool
	CanAsync    bool
}

// SwitchDriver manages all switches
type SwitchDriver struct {
	mu                  sync.RWMutex
	switches            [MaxSwitch]SwitchDevice
	connected           bool
	serverTransactionID uint32
}

var driver *SwitchDriver

func init() {
	driver = &SwitchDriver{
		connected: false,
	}

	// Initialize switches
	for i := 0; i < MaxSwitch; i++ {
		driver.switches[i] = SwitchDevice{
			Name:        fmt.Sprintf("Switch %d", i),
			Description: fmt.Sprintf("Switch device number %d", i),
			State:       false,
			Value:       0.0,
			MinValue:    0.0,
			MaxValue:    1.0,
			CanWrite:    true,
			CanAsync:    false,
		}
	}
}

func main() {
	// Start UDP discovery service
	go startDiscoveryService()

	r := mux.NewRouter()

	// Common device methods
	r.HandleFunc("/api/v1/switch/{device_number}/connected", handleConnected).Methods("GET", "PUT")
	r.HandleFunc("/api/v1/switch/{device_number}/description", handleDescription).Methods("GET")
	r.HandleFunc("/api/v1/switch/{device_number}/driverinfo", handleDriverInfo).Methods("GET")
	r.HandleFunc("/api/v1/switch/{device_number}/driverversion", handleDriverVersion).Methods("GET")
	r.HandleFunc("/api/v1/switch/{device_number}/interfaceversion", handleInterfaceVersion).Methods("GET")
	r.HandleFunc("/api/v1/switch/{device_number}/name", handleName).Methods("GET")
	r.HandleFunc("/api/v1/switch/{device_number}/supportedactions", handleSupportedActions).Methods("GET")

	// Switch-specific methods
	r.HandleFunc("/api/v1/switch/{device_number}/maxswitch", handleMaxSwitch).Methods("GET")
	r.HandleFunc("/api/v1/switch/{device_number}/canwrite", handleCanWrite).Methods("GET")
	r.HandleFunc("/api/v1/switch/{device_number}/canasync", handleCanAsync).Methods("GET")
	r.HandleFunc("/api/v1/switch/{device_number}/getswitch", handleGetSwitch).Methods("GET")
	r.HandleFunc("/api/v1/switch/{device_number}/getswitchname", handleGetSwitchName).Methods("GET")
	r.HandleFunc("/api/v1/switch/{device_number}/getswitchdescription", handleGetSwitchDescription).Methods("GET")
	r.HandleFunc("/api/v1/switch/{device_number}/getswitchvalue", handleGetSwitchValue).Methods("GET")
	r.HandleFunc("/api/v1/switch/{device_number}/minswitchvalue", handleMinSwitchValue).Methods("GET")
	r.HandleFunc("/api/v1/switch/{device_number}/maxswitchvalue", handleMaxSwitchValue).Methods("GET")
	r.HandleFunc("/api/v1/switch/{device_number}/setswitch", handleSetSwitch).Methods("PUT")
	r.HandleFunc("/api/v1/switch/{device_number}/setswitchname", handleSetSwitchName).Methods("PUT")
	r.HandleFunc("/api/v1/switch/{device_number}/setswitchvalue", handleSetSwitchValue).Methods("PUT")

	// Management API - basic discovery
	r.HandleFunc("/management/apiversions", handleAPIVersions).Methods("GET")
	r.HandleFunc("/management/v1/description", handleManagementDescription).Methods("GET")
	r.HandleFunc("/management/v1/configureddevices", handleConfiguredDevices).Methods("GET")

	// Status page
	r.HandleFunc("/", handleStatusPage).Methods("GET")

	// Enable CORS for all routes
	r.Use(corsMiddleware)

	addr := fmt.Sprintf(":%d", APIPort)
	log.Printf("Starting ASCOM Alpaca Switch Driver")
	log.Printf("UDP Discovery: port %d", DiscoveryPort)
	log.Printf("HTTP API: port %d", APIPort)
	log.Printf("Status page: http://localhost:%d/", APIPort)
	log.Printf("API endpoint: http://localhost:%d/api/v1/switch/0/", APIPort)
	log.Fatal(http.ListenAndServe(addr, r))
}

// startDiscoveryService starts the UDP discovery service for ASCOM Alpaca
func startDiscoveryService() {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("Discovery service panic recovered: %v", r)
		}
	}()

	addr := net.UDPAddr{
		Port: DiscoveryPort,
		IP:   net.IPv4zero,
	}

	conn, err := net.ListenUDP("udp4", &addr)
	if err != nil {
		log.Printf("Failed to start UDP discovery service: %v", err)
		return
	}
	defer conn.Close()

	// Enable broadcast reception
	err = conn.SetReadBuffer(1024)
	if err != nil {
		log.Printf("Warning: Could not set read buffer: %v", err)
	}

	log.Printf("UDP discovery service listening on port %d (ready for broadcasts)", DiscoveryPort)

	buffer := make([]byte, 1024)
	for {
		n, remoteAddr, err := conn.ReadFromUDP(buffer)
		if err != nil {
			log.Printf("Error reading UDP packet: %v", err)
			continue
		}

		message := string(buffer[:n])
		log.Printf("Received UDP message (%d bytes) from %s: %s", n, remoteAddr.String(), message)

		// Check if this is an Alpaca discovery request
		if strings.Contains(strings.ToLower(message), "alpacadiscovery1") {
			log.Printf("Valid Alpaca discovery request detected")

			// Respond with the Alpaca API port
			response := map[string]int{
				"AlpacaPort": APIPort,
			}

			responseJSON, err := json.Marshal(response)
			if err != nil {
				log.Printf("Error marshaling discovery response: %v", err)
				continue
			}

			log.Printf("Sending response: %s to %s", string(responseJSON), remoteAddr.String())

			// Send response back to requester
			sent, err := conn.WriteToUDP(responseJSON, remoteAddr)
			if err != nil {
				log.Printf("Error sending discovery response: %v", err)
			} else {
				log.Printf("Successfully sent %d bytes to %s", sent, remoteAddr.String())
			}
		} else {
			log.Printf("Not an Alpaca discovery message (ignored)")
		}
	}
}

func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, PUT, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}

		next.ServeHTTP(w, r)
	})
}

func getClientTransactionID(r *http.Request) uint32 {
	if err := r.ParseForm(); err != nil {
		return 0
	}
	clientIDStr := r.FormValue("ClientTransactionID")
	if clientIDStr == "" {
		clientIDStr = r.URL.Query().Get("ClientTransactionID")
	}
	clientID, _ := strconv.ParseUint(clientIDStr, 10, 32)
	return uint32(clientID)
}

func getNextServerTransactionID() uint32 {
	driver.mu.Lock()
	defer driver.mu.Unlock()
	driver.serverTransactionID++
	return driver.serverTransactionID
}

func sendResponse(w http.ResponseWriter, clientTxnID uint32, value interface{}, err error) {
	response := AlpacaResponse{
		ClientTransactionID: clientTxnID,
		ServerTransactionID: getNextServerTransactionID(),
		ErrorNumber:         0,
		ErrorMessage:        "",
		Value:               value,
	}

	if err != nil {
		response.ErrorNumber = 0x500 // Generic error
		response.ErrorMessage = err.Error()
		response.Value = nil
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func validateSwitchID(idStr string) (int, error) {
	id, err := strconv.Atoi(idStr)
	if err != nil {
		return 0, fmt.Errorf("invalid switch ID")
	}
	if id < 0 || id >= MaxSwitch {
		return 0, fmt.Errorf("switch ID out of range (0-%d)", MaxSwitch-1)
	}
	return id, nil
}

// Common device method handlers
func handleConnected(w http.ResponseWriter, r *http.Request) {
	clientTxnID := getClientTransactionID(r)

	if r.Method == "GET" {
		driver.mu.RLock()
		connected := driver.connected
		driver.mu.RUnlock()
		sendResponse(w, clientTxnID, connected, nil)
	} else if r.Method == "PUT" {
		if err := r.ParseForm(); err != nil {
			sendResponse(w, clientTxnID, nil, err)
			return
		}

		connectedStr := r.FormValue("Connected")
		connected, err := strconv.ParseBool(connectedStr)
		if err != nil {
			sendResponse(w, clientTxnID, nil, fmt.Errorf("invalid Connected parameter"))
			return
		}

		driver.mu.Lock()
		driver.connected = connected
		driver.mu.Unlock()

		sendResponse(w, clientTxnID, nil, nil)
	}
}

func handleDescription(w http.ResponseWriter, r *http.Request) {
	sendResponse(w, getClientTransactionID(r), Description, nil)
}

func handleDriverInfo(w http.ResponseWriter, r *http.Request) {
	info := fmt.Sprintf("%s v%s - Manages %d switches", DriverName, DriverVersion, MaxSwitch)
	sendResponse(w, getClientTransactionID(r), info, nil)
}

func handleDriverVersion(w http.ResponseWriter, r *http.Request) {
	sendResponse(w, getClientTransactionID(r), DriverVersion, nil)
}

func handleInterfaceVersion(w http.ResponseWriter, r *http.Request) {
	sendResponse(w, getClientTransactionID(r), 2, nil) // ISwitchV2
}

func handleName(w http.ResponseWriter, r *http.Request) {
	sendResponse(w, getClientTransactionID(r), DriverName, nil)
}

func handleSupportedActions(w http.ResponseWriter, r *http.Request) {
	sendResponse(w, getClientTransactionID(r), []string{}, nil)
}

// Switch-specific method handlers
func handleMaxSwitch(w http.ResponseWriter, r *http.Request) {
	sendResponse(w, getClientTransactionID(r), MaxSwitch, nil)
}

func handleCanWrite(w http.ResponseWriter, r *http.Request) {
	clientTxnID := getClientTransactionID(r)
	idStr := r.URL.Query().Get("Id")
	id, err := validateSwitchID(idStr)
	if err != nil {
		sendResponse(w, clientTxnID, nil, err)
		return
	}

	driver.mu.RLock()
	canWrite := driver.switches[id].CanWrite
	driver.mu.RUnlock()

	sendResponse(w, clientTxnID, canWrite, nil)
}

func handleCanAsync(w http.ResponseWriter, r *http.Request) {
	clientTxnID := getClientTransactionID(r)
	idStr := r.URL.Query().Get("Id")
	id, err := validateSwitchID(idStr)
	if err != nil {
		sendResponse(w, clientTxnID, nil, err)
		return
	}

	driver.mu.RLock()
	canAsync := driver.switches[id].CanAsync
	driver.mu.RUnlock()

	sendResponse(w, clientTxnID, canAsync, nil)
}

func handleGetSwitch(w http.ResponseWriter, r *http.Request) {
	clientTxnID := getClientTransactionID(r)
	idStr := r.URL.Query().Get("Id")
	id, err := validateSwitchID(idStr)
	if err != nil {
		sendResponse(w, clientTxnID, nil, err)
		return
	}

	driver.mu.RLock()
	state := driver.switches[id].State
	driver.mu.RUnlock()

	sendResponse(w, clientTxnID, state, nil)
}

func handleGetSwitchName(w http.ResponseWriter, r *http.Request) {
	clientTxnID := getClientTransactionID(r)
	idStr := r.URL.Query().Get("Id")
	id, err := validateSwitchID(idStr)
	if err != nil {
		sendResponse(w, clientTxnID, nil, err)
		return
	}

	driver.mu.RLock()
	name := driver.switches[id].Name
	driver.mu.RUnlock()

	sendResponse(w, clientTxnID, name, nil)
}

func handleGetSwitchDescription(w http.ResponseWriter, r *http.Request) {
	clientTxnID := getClientTransactionID(r)
	idStr := r.URL.Query().Get("Id")
	id, err := validateSwitchID(idStr)
	if err != nil {
		sendResponse(w, clientTxnID, nil, err)
		return
	}

	driver.mu.RLock()
	description := driver.switches[id].Description
	driver.mu.RUnlock()

	sendResponse(w, clientTxnID, description, nil)
}

func handleGetSwitchValue(w http.ResponseWriter, r *http.Request) {
	clientTxnID := getClientTransactionID(r)
	idStr := r.URL.Query().Get("Id")
	id, err := validateSwitchID(idStr)
	if err != nil {
		sendResponse(w, clientTxnID, nil, err)
		return
	}

	driver.mu.RLock()
	value := driver.switches[id].Value
	driver.mu.RUnlock()

	sendResponse(w, clientTxnID, value, nil)
}

func handleMinSwitchValue(w http.ResponseWriter, r *http.Request) {
	clientTxnID := getClientTransactionID(r)
	idStr := r.URL.Query().Get("Id")
	id, err := validateSwitchID(idStr)
	if err != nil {
		sendResponse(w, clientTxnID, nil, err)
		return
	}

	driver.mu.RLock()
	minValue := driver.switches[id].MinValue
	driver.mu.RUnlock()

	sendResponse(w, clientTxnID, minValue, nil)
}

func handleMaxSwitchValue(w http.ResponseWriter, r *http.Request) {
	clientTxnID := getClientTransactionID(r)
	idStr := r.URL.Query().Get("Id")
	id, err := validateSwitchID(idStr)
	if err != nil {
		sendResponse(w, clientTxnID, nil, err)
		return
	}

	driver.mu.RLock()
	maxValue := driver.switches[id].MaxValue
	driver.mu.RUnlock()

	sendResponse(w, clientTxnID, maxValue, nil)
}

func handleSetSwitch(w http.ResponseWriter, r *http.Request) {
	clientTxnID := getClientTransactionID(r)

	if err := r.ParseForm(); err != nil {
		sendResponse(w, clientTxnID, nil, err)
		return
	}

	idStr := r.FormValue("Id")
	id, err := validateSwitchID(idStr)
	if err != nil {
		sendResponse(w, clientTxnID, nil, err)
		return
	}

	stateStr := r.FormValue("State")
	state, err := strconv.ParseBool(stateStr)
	if err != nil {
		sendResponse(w, clientTxnID, nil, fmt.Errorf("invalid State parameter"))
		return
	}

	driver.mu.Lock()
	if !driver.switches[id].CanWrite {
		driver.mu.Unlock()
		sendResponse(w, clientTxnID, nil, fmt.Errorf("switch is read-only"))
		return
	}
	driver.switches[id].State = state
	if state {
		driver.switches[id].Value = driver.switches[id].MaxValue
	} else {
		driver.switches[id].Value = driver.switches[id].MinValue
	}
	driver.mu.Unlock()

	log.Printf("Switch %d set to %v", id, state)
	sendResponse(w, clientTxnID, nil, nil)
}

func handleSetSwitchName(w http.ResponseWriter, r *http.Request) {
	clientTxnID := getClientTransactionID(r)

	if err := r.ParseForm(); err != nil {
		sendResponse(w, clientTxnID, nil, err)
		return
	}

	idStr := r.FormValue("Id")
	id, err := validateSwitchID(idStr)
	if err != nil {
		sendResponse(w, clientTxnID, nil, err)
		return
	}

	name := r.FormValue("Name")
	if name == "" {
		sendResponse(w, clientTxnID, nil, fmt.Errorf("Name parameter is required"))
		return
	}

	driver.mu.Lock()
	driver.switches[id].Name = name
	driver.mu.Unlock()

	sendResponse(w, clientTxnID, nil, nil)
}

func handleSetSwitchValue(w http.ResponseWriter, r *http.Request) {
	clientTxnID := getClientTransactionID(r)

	if err := r.ParseForm(); err != nil {
		sendResponse(w, clientTxnID, nil, err)
		return
	}

	idStr := r.FormValue("Id")
	id, err := validateSwitchID(idStr)
	if err != nil {
		sendResponse(w, clientTxnID, nil, err)
		return
	}

	valueStr := r.FormValue("Value")
	value, err := strconv.ParseFloat(valueStr, 64)
	if err != nil {
		sendResponse(w, clientTxnID, nil, fmt.Errorf("invalid Value parameter"))
		return
	}

	driver.mu.Lock()
	if !driver.switches[id].CanWrite {
		driver.mu.Unlock()
		sendResponse(w, clientTxnID, nil, fmt.Errorf("switch is read-only"))
		return
	}
	if value < driver.switches[id].MinValue || value > driver.switches[id].MaxValue {
		driver.mu.Unlock()
		sendResponse(w, clientTxnID, nil, fmt.Errorf("value out of range"))
		return
	}
	driver.switches[id].Value = value
	driver.switches[id].State = value > 0
	driver.mu.Unlock()

	log.Printf("Switch %d value set to %f", id, value)
	sendResponse(w, clientTxnID, nil, nil)
}

// Management API handlers
func handleAPIVersions(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode([]int{1})
}

func handleManagementDescription(w http.ResponseWriter, r *http.Request) {
	description := map[string]interface{}{
		"ServerName":          DriverName,
		"Manufacturer":        "Custom",
		"ManufacturerVersion": DriverVersion,
		"Location":            "Local",
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(description)
}

func handleConfiguredDevices(w http.ResponseWriter, r *http.Request) {
	devices := []map[string]interface{}{
		{
			"DeviceName":   DriverName,
			"DeviceType":   DeviceType,
			"DeviceNumber": DeviceNumber,
			"UniqueID":     "12345678-1234-1234-1234-123456789012",
		},
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(devices)
}

// Status page handler
func handleStatusPage(w http.ResponseWriter, r *http.Request) {
	driver.mu.RLock()
	defer driver.mu.RUnlock()

	html := `<!DOCTYPE html>
<html>
<head>
    <title>ASCOM Alpaca Switch Driver</title>
    <meta http-equiv="refresh" content="5">
    <style>
        body {
            font-family: Arial, sans-serif;
            max-width: 1200px;
            margin: 0 auto;
            padding: 20px;
            background-color: #f5f5f5;
        }
        h1 {
            color: #2c3e50;
            border-bottom: 3px solid #3498db;
            padding-bottom: 10px;
        }
        .info {
            background: white;
            padding: 20px;
            border-radius: 5px;
            margin-bottom: 20px;
            box-shadow: 0 2px 4px rgba(0,0,0,0.1);
        }
        .info p {
            margin: 8px 0;
        }
        .switch-grid {
            display: grid;
            grid-template-columns: repeat(auto-fit, minmax(350px, 1fr));
            gap: 15px;
        }
        .switch-card {
            background: white;
            padding: 15px;
            border-radius: 5px;
            box-shadow: 0 2px 4px rgba(0,0,0,0.1);
        }
        .switch-header {
            font-weight: bold;
            font-size: 1.1em;
            color: #2c3e50;
            margin-bottom: 10px;
        }
        .switch-status {
            display: inline-block;
            padding: 5px 15px;
            border-radius: 3px;
            font-weight: bold;
            margin: 10px 0;
        }
        .status-on {
            background-color: #2ecc71;
            color: white;
        }
        .status-off {
            background-color: #95a5a6;
            color: white;
        }
        .property {
            margin: 5px 0;
            color: #555;
        }
        .label {
            font-weight: bold;
            color: #34495e;
        }
        .footer {
            text-align: center;
            margin-top: 30px;
            color: #7f8c8d;
            font-size: 0.9em;
        }
    </style>
</head>
<body>
    <h1>🔌 ASCOM Alpaca Switch Driver</h1>
    
    <div class="info">
        <p><span class="label">Driver:</span> ` + DriverName + ` v` + DriverVersion + `</p>
        <p><span class="label">Discovery Port:</span> ` + fmt.Sprintf("%d (UDP)", DiscoveryPort) + `</p>
        <p><span class="label">API Port:</span> ` + fmt.Sprintf("%d (HTTP)", APIPort) + `</p>
        <p><span class="label">API Endpoint:</span> <code>http://localhost:` + fmt.Sprintf("%d", APIPort) + `/api/v1/switch/0/</code></p>
        <p><span class="label">Connected:</span> ` + fmt.Sprintf("%v", driver.connected) + `</p>
        <p><span class="label">Max Switches:</span> ` + fmt.Sprintf("%d", MaxSwitch) + `</p>
        <p><span class="label">Time:</span> ` + time.Now().Format("2006-01-02 15:04:05") + `</p>
    </div>

    <h2>Switch Status</h2>
    <div class="switch-grid">`

	for i := 0; i < MaxSwitch; i++ {
		sw := driver.switches[i]
		statusClass := "status-off"
		statusText := "OFF"
		if sw.State {
			statusClass = "status-on"
			statusText = "ON"
		}

		html += fmt.Sprintf(`
        <div class="switch-card">
            <div class="switch-header">Switch %d</div>
            <div class="switch-status %s">%s</div>
            <div class="property"><span class="label">Name:</span> %s</div>
            <div class="property"><span class="label">Description:</span> %s</div>
            <div class="property"><span class="label">Value:</span> %.2f</div>
            <div class="property"><span class="label">Range:</span> %.2f - %.2f</div>
            <div class="property"><span class="label">Writable:</span> %v</div>
        </div>`, i, statusClass, statusText, sw.Name, sw.Description, sw.Value, sw.MinValue, sw.MaxValue, sw.CanWrite)
	}

	html += `
    </div>
    
    <div class="footer">
        <p>Page automatically refreshes every 5 seconds</p>
        <p>ASCOM Alpaca Device API v1</p>
    </div>
</body>
</html>`

	w.Header().Set("Content-Type", "text/html")
	w.Write([]byte(html))
}
