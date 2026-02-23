# ASCOM Alpaca Switch Driver

A fully compliant ASCOM Alpaca switch driver written in Golang that manages 6 independent switches. This driver is compatible with ASCOM-compliant astronomy applications like N.I.N.A. (Nighttime Imaging 'N' Astronomy).

## Features

- **6 Independent Switches**: Manage up to 6 separate switch devices
- **ASCOM Alpaca Protocol**: Full implementation of the ASCOM Alpaca Switch API v1
- **Web Status Page**: Real-time status monitoring on port 9999
- **N.I.N.A. Compatible**: Works seamlessly with N.I.N.A. and other ASCOM clients
- **RESTful API**: Standard HTTP GET/PUT endpoints for all operations
- **Thread-Safe**: Concurrent access protection with mutex locks
- **CORS Enabled**: Cross-origin requests supported

## Requirements

- Go 1.21 or higher
- Network connectivity on port 32227 (UDP for discovery, HTTP for API)

## Installation

1. Clone or download this repository
2. Install dependencies:
   ```bash
   go mod download
   ```

3. Build the driver:
   ```bash
   go build -o alpaca-switch.exe
   ```

## Running the Driver

Start the driver with:
```bash
go run main.go
```

Or run the compiled executable:
```bash
./alpaca-switch.exe
```

The driver will start on port 32227 and display:
```
Starting ASCOM Alpaca Switch Driver
UDP Discovery: port 32227
HTTP API: port 32227
Status page: http://localhost:32227/
API endpoint: http://localhost:32227/api/v1/switch/0/
```

## Usage

### Web Status Page

Open your browser and navigate to:
```
http://localhost:32227/
```

The status page shows:
- Driver information and connection status
- Real-time state of all 6 switches
- Switch names, descriptions, and values
- Auto-refreshes every 5 seconds

### Connecting from N.I.N.A.

1. Open N.I.N.A.
2. Go to Equipment → Switch
3. Click "Choose Switch"
4. Select "ASCOM Alpaca Discovery" or manually add:
   - Host: `localhost` (or your server IP)
   - Port: `32227`
   - Device Type: `switch`
   - Device Number: `0`
5. Click "Connect"
6. You should now see all 6 switches available to control

### API Endpoints

All endpoints follow the ASCOM Alpaca standard format:
```
http://localhost:32227/api/v1/switch/0/{method}
```

#### Common Device Methods (GET)
- `/connected` - Get/Set connection status
- `/description` - Get device description
- `/driverinfo` - Get driver information
- `/driverversion` - Get driver version
- `/interfaceversion` - Get interface version (returns 2)
- `/name` - Get device name
- `/supportedactions` - Get supported actions

#### Switch-Specific Methods

**GET Methods:**
- `/maxswitch` - Returns 6 (number of switches)
- `/canwrite?Id={0-5}` - Check if switch is writable
- `/canasync?Id={0-5}` - Check if switch supports async operations
- `/getswitch?Id={0-5}` - Get switch state (true/false)
- `/getswitchname?Id={0-5}` - Get switch name
- `/getswitchdescription?Id={0-5}` - Get switch description
- `/getswitchvalue?Id={0-5}` - Get switch value (0.0-1.0)
- `/minswitchvalue?Id={0-5}` - Get minimum value (0.0)
- `/maxswitchvalue?Id={0-5}` - Get maximum value (1.0)

**PUT Methods:**
- `/setswitch` - Set switch state
  - Parameters: `Id={0-5}`, `State={true|false}`
- `/setswitchname` - Set switch name
  - Parameters: `Id={0-5}`, `Name={string}`
- `/setswitchvalue` - Set switch value
  - Parameters: `Id={0-5}`, `Value={0.0-1.0}`

#### Management API
- `/management/apiversions` - Supported API versions
- `/management/v1/description` - Server description
- `/management/v1/configureddevices` - List configured devices

### Example API Calls

**Get switch state:**
```bash
curl "http://localhost:9999/api/v1/switch/0/getswitch?Id=0&ClientTransactionID=1"
```

**Turn switch ON:**
```bash
curl -X PUT "http://localhost:9999/api/v1/switch/0/setswitch" \
  -d "Id=0&State=true&ClientTransactionID=1"
```

**Turn switch OFF:**
```bash
curl -X PUT "http://localhost:9999/api/v1/switch/0/setswitch" \
  -d "Id=0&State=false&ClientTransactionID=1"
```

**Set switch name:**
```bash
curl -X PUT "http://localhost:9999/api/v1/switch/0/setswitchname" \
  -d "Id=0&Name=Dew%20Heater&ClientTransactionID=1"
```

## Response Format

All API responses follow the ASCOM Alpaca standard JSON format:

```json
{
  "ClientTransactionID": 1,
  "ServerTransactionID": 42,
  "ErrorNumber": 0,
  "ErrorMessage": "",
  "Value": true
}
```

- `ErrorNumber`: 0 = success, non-zero = error
- `ErrorMessage`: Empty string on success, error description on failure
- `Value`: The requested value (omitted for PUT operations)

## Switch Configuration

By default, all switches are configured as:
- **Name**: "Switch 0" through "Switch 5"
- **Description**: "Switch device number {N}"
- **Initial State**: OFF (false)
- **Value Range**: 0.0 (OFF) to 1.0 (ON)
- **Writable**: Yes
- **Async Support**: No

You can customize switch names via the API or modify the initialization code in [main.go](main.go).

## Customization

To modify switch properties, edit the initialization in `main.go`:

```go
driver.switches[i] = SwitchDevice{
    Name:        fmt.Sprintf("My Switch %d", i),
    Description: "Custom description",
    State:       false,
    Value:       0.0,
    MinValue:    0.0,
    MaxValue:    1.0,
    CanWrite:    true,
    CanAsync:    false,
}
```

## Troubleshooting

**Port already in use:**
- Change the `Port` constant in `main.go` to a different port
- Or stop the application using port 9999

**N.I.N.A. can't discover the driver:**
- Ensure the driver is running
- Check firewall settings allow port 9999
- Manually add the device with host:port information

**Switches not responding:**
- Check the status page to verify driver state
- Ensure you're sending the correct `Id` parameter (0-5)
- Verify switch `CanWrite` property is true

## ASCOM Alpaca Specification

This driver implements:
- ASCOM Alpaca Device API v1
- Switch Interface Version 2 (ISwitchV2)

Full specification: https://ascom-standards.org/api/

## License

MIT License - Feel free to modify and use for your projects.

## Support

For issues or questions:
- Check the status page at http://localhost:9999/
- Review the console logs for error messages
- Verify API responses match ASCOM standards
