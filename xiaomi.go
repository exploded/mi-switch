package main

// xiaomi.go implements the low-level Xiaomi Mi Smart Plug UDP protocol.
// Packets use AES-CBC encryption with an MD5-derived key and IV.

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net"
	"time"
)

// setSwitch turns a Xiaomi Mi Smart Plug on or off.
func setSwitch(host, token string, on bool) error {
	tokenBytes, err := hex.DecodeString(token)
	if err != nil {
		return fmt.Errorf("decoding token: %w", err)
	}
	deviceID, stamp, err := discoverDevice(host)
	if err != nil {
		return fmt.Errorf("discovery: %w", err)
	}
	return setPower(host, tokenBytes, deviceID, stamp, on)
}

// getSwitch returns the live on/off state of a Xiaomi Mi Smart Plug.
func getSwitch(host, token string) (bool, error) {
	tokenBytes, err := hex.DecodeString(token)
	if err != nil {
		return false, fmt.Errorf("decoding token: %w", err)
	}
	deviceID, stamp, err := discoverDevice(host)
	if err != nil {
		return false, fmt.Errorf("discovery: %w", err)
	}

	command := map[string]interface{}{
		"id":     1,
		"method": "get_prop",
		"params": []interface{}{"power"},
	}
	jsonData, err := json.Marshal(command)
	if err != nil {
		return false, err
	}
	encrypted, err := encryptPayload(jsonData, tokenBytes)
	if err != nil {
		return false, err
	}
	packet := buildPacket(tokenBytes, deviceID, stamp, encrypted)

	conn, err := net.DialTimeout("udp", fmt.Sprintf("%s:54321", host), 5*time.Second)
	if err != nil {
		return false, err
	}
	defer conn.Close()
	conn.SetDeadline(time.Now().Add(3 * time.Second))

	if _, err = conn.Write(packet); err != nil {
		return false, err
	}
	buf := make([]byte, 1024)
	n, err := conn.Read(buf)
	if err != nil {
		return false, fmt.Errorf("reading response: %w", err)
	}
	if n < 32 {
		return false, fmt.Errorf("response too short (%d bytes)", n)
	}
	decrypted, err := decryptPayload(buf[32:n], tokenBytes)
	if err != nil {
		return false, fmt.Errorf("decrypting response: %w", err)
	}
	var resp struct {
		Result []string `json:"result"`
	}
	if err := json.Unmarshal(decrypted, &resp); err != nil {
		return false, fmt.Errorf("parsing response: %w", err)
	}
	if len(resp.Result) > 0 {
		return resp.Result[0] == "on", nil
	}
	return false, fmt.Errorf("no power state in response")
}

// discoverDevice sends a hello packet and returns (deviceID, stamp).
func discoverDevice(ipAddress string) ([]byte, []byte, error) {
	hello := make([]byte, 32)
	hello[0] = 0x21
	hello[1] = 0x31
	hello[2] = 0x00
	hello[3] = 0x20
	for i := 4; i < 32; i++ {
		hello[i] = 0xFF
	}
	conn, err := net.DialTimeout("udp", fmt.Sprintf("%s:54321", ipAddress), 5*time.Second)
	if err != nil {
		return nil, nil, err
	}
	defer conn.Close()
	conn.SetDeadline(time.Now().Add(5 * time.Second))
	if _, err = conn.Write(hello); err != nil {
		return nil, nil, err
	}
	buf := make([]byte, 1024)
	n, err := conn.Read(buf)
	if err != nil {
		return nil, nil, err
	}
	if n < 16 {
		return nil, nil, fmt.Errorf("hello response too short")
	}
	return buf[8:12], buf[12:16], nil
}

// setPower sends a set_power command to ipAddress.
func setPower(ipAddress string, token, deviceID, stamp []byte, powerOn bool) error {
	state := "off"
	if powerOn {
		state = "on"
	}
	command := map[string]interface{}{
		"id":     1,
		"method": "set_power",
		"params": []interface{}{state},
	}
	jsonData, err := json.Marshal(command)
	if err != nil {
		return err
	}
	encrypted, err := encryptPayload(jsonData, token)
	if err != nil {
		return err
	}
	packet := buildPacket(token, deviceID, stamp, encrypted)

	conn, err := net.DialTimeout("udp", fmt.Sprintf("%s:54321", ipAddress), 5*time.Second)
	if err != nil {
		return err
	}
	defer conn.Close()
	conn.SetDeadline(time.Now().Add(3 * time.Second))
	if _, err = conn.Write(packet); err != nil {
		return err
	}
	// Response is optional; ignore timeout
	buf := make([]byte, 1024)
	_, _ = conn.Read(buf)
	return nil
}

// buildPacket frames an encrypted payload into a Xiaomi protocol packet.
func buildPacket(token, deviceID, stamp, encryptedData []byte) []byte {
	pkt := make([]byte, 32+len(encryptedData))
	pkt[0] = 0x21
	pkt[1] = 0x31
	length := uint16(len(pkt))
	pkt[2] = byte(length >> 8)
	pkt[3] = byte(length)
	copy(pkt[8:12], deviceID)
	copy(pkt[12:16], stamp)
	copy(pkt[32:], encryptedData)

	// MD5 checksum over header + token + payload
	checksumInput := make([]byte, 16+len(token)+len(encryptedData))
	copy(checksumInput[0:16], pkt[0:16])
	copy(checksumInput[16:], token)
	copy(checksumInput[16+len(token):], encryptedData)
	sum := md5.Sum(checksumInput)
	copy(pkt[16:32], sum[:])
	return pkt
}

// encryptPayload encrypts data with AES-CBC using an MD5-derived key and IV.
func encryptPayload(data, token []byte) ([]byte, error) {
	key := md5sum(token)
	iv := md5sum(append(key, token...))

	// PKCS7 padding
	padding := aes.BlockSize - len(data)%aes.BlockSize
	padded := make([]byte, len(data)+padding)
	copy(padded, data)
	for i := len(data); i < len(padded); i++ {
		padded[i] = byte(padding)
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	ciphertext := make([]byte, len(padded))
	cipher.NewCBCEncrypter(block, iv).CryptBlocks(ciphertext, padded)
	return ciphertext, nil
}

// decryptPayload reverses encryptPayload.
func decryptPayload(encryptedData, token []byte) ([]byte, error) {
	key := md5sum(token)
	iv := md5sum(append(key, token...))

	if len(encryptedData)%aes.BlockSize != 0 {
		return nil, fmt.Errorf("encrypted data length is not a multiple of block size")
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	plaintext := make([]byte, len(encryptedData))
	cipher.NewCBCDecrypter(block, iv).CryptBlocks(plaintext, encryptedData)

	// Strip PKCS7 padding
	if len(plaintext) > 0 {
		pad := int(plaintext[len(plaintext)-1])
		if pad > 0 && pad <= aes.BlockSize && pad <= len(plaintext) {
			plaintext = plaintext[:len(plaintext)-pad]
		}
	}
	return plaintext, nil
}

func md5sum(data []byte) []byte {
	s := md5.Sum(data)
	return s[:]
}
