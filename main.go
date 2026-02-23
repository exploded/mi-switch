package main

import (
	"flag"
	"fmt"
	"os"
)

func main() {
	host  := flag.String("host",   "",      "Device IP address (required)")
	token := flag.String("token",  "",      "32-character hex token (required)")
	action := flag.String("action", "",     "Action: on | off | status (required)")
	flag.Parse()

	if *host == "" || *token == "" || *action == "" {
		fmt.Fprintf(os.Stderr, "Usage: mi-switch --host <IP> --token <32-hex-token> --action on|off|status\n")
		os.Exit(1)
	}

	switch *action {
	case "on":
		if err := setSwitch(*host, *token, true); err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		}
		fmt.Println("Switch: on")

	case "off":
		if err := setSwitch(*host, *token, false); err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		}
		fmt.Println("Switch: off")

	case "status":
		on, err := getSwitch(*host, *token)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		}
		if on {
			fmt.Println("Switch: on")
		} else {
			fmt.Println("Switch: off")
		}

	default:
		fmt.Fprintf(os.Stderr, "unknown action %q — must be on, off, or status\n", *action)
		os.Exit(1)
	}
}
