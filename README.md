# mi-switch

A minimal CLI tool to control Xiaomi Mi Smart Plugs (Wi-Fi power switches) from the command line.

Uses the Xiaomi UDP protocol with AES-CBC encryption — no cloud, no account, no dependencies.

![Xiaomi Wi-Fi Switch](xiaomi-wifi-switch.jpg)

## Requirements

- Go 1.21 or later
- Device IP address and 32-character hex token (obtainable with [python-miio](https://github.com/rytilahti/python-miio))

## Build

```bash
go build -o mi-switch.exe .
```

## Usage

```
mi-switch --host <IP> --token <32-hex-token> --action on|off|status
```

### Flags

| Flag | Description |
|------|-------------|
| `--host` | Device IP address (required) |
| `--token` | 32-character hex authentication token (required) |
| `--action` | `on`, `off`, or `status` (required) |

### Examples

Turn a plug on:
```bash
mi-switch --host 192.168.1.100 --token a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4 --action on
```

Turn a plug off:
```bash
mi-switch --host 192.168.1.100 --token a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4 --action off
```

Query current state:
```bash
mi-switch --host 192.168.1.100 --token a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4 --action status
```

Output:
```
Switch: on
```

## Token extraction

Use [python-miio](https://github.com/rytilahti/python-miio) to extract your device token:

```bash
pip install python-miio
miiocli device --ip 192.168.1.100 --token 00000000000000000000000000000000 info
```

Or follow the [miio token extraction guide](https://python-miio.readthedocs.io/en/latest/discovery.html).

## Protocol

Communicates directly with the device over UDP port 54321 using the Xiaomi LAN protocol:
- Hello packet for device discovery (retrieves device ID and timestamp)
- AES-CBC encrypted JSON commands (`set_power`, `get_prop`)
- MD5-derived encryption key and IV from the device token

## References

- [python-miio](https://github.com/rytilahti/python-miio)
- [Xiaomi Mi Smart Plug protocol](https://github.com/nickel-org/nickel/blob/master/miio-protocol.md)
