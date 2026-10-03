package core

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

type HostRemoval struct {
	NodeID      uint   `json:"nodeId"`
	Purge       bool   `json:"purge"`
	Receipt     string `json:"receipt,omitempty"`
	Success     bool   `json:"success,omitempty"`
	CallbackURL string `json:"callbackUrl,omitempty"`
}

// HostRemovalCallbackURL comes from the Web deployment, never browser input.
func HostRemovalCallbackURL() (string, error) {
	raw := os.Getenv("TP_HOST_REMOVAL_CALLBACK_URL")
	u, err := url.Parse(raw)
	if err != nil || len(raw) > 2048 || u.Scheme != "https" || u.Hostname() == "" || u.User != nil || u.Path != "/api/nodeServer/completeHostRemoval" || u.RawQuery != "" || u.Fragment != "" {
		return "", errors.New("TP_HOST_REMOVAL_CALLBACK_URL must be the Web HTTPS /api/nodeServer/completeHostRemoval endpoint")
	}
	return raw, nil
}

func RemoveHost(ip string, port uint, transport NodeTransport, request HostRemoval) (*HostRemoval, error) {
	return hostRemovalCall(ip, port, transport, "/remove", request)
}

func FinalizeHostRemoval(ip string, port uint, transport NodeTransport, request HostRemoval) error {
	_, err := hostRemovalCall(ip, port, transport, "/finalize", request)
	return err
}

func hostRemovalCall(ip string, port uint, transport NodeTransport, path string, request HostRemoval) (*HostRemoval, error) {
	if transport.Mode != "mtls" || request.NodeID == 0 || port == 0 || port > 65535 {
		return nil, errors.New("host removal requires mTLS, node ID and a valid port")
	}
	tlsConfig, err := clientTLSConfig(transport.ServerName)
	if err != nil {
		return nil, err
	}
	clientTransport := &http.Transport{TLSClientConfig: tlsConfig, Proxy: nil, DialContext: (&net.Dialer{Timeout: 10 * time.Second}).DialContext, TLSHandshakeTimeout: 10 * time.Second}
	defer clientTransport.CloseIdleConnections()
	timeout := 190 * time.Second
	if path == "/finalize" {
		timeout = 15 * time.Second
	}
	client := &http.Client{Transport: clientTransport, Timeout: timeout, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	request.Success = false
	body, err := json.Marshal(struct {
		NodeID      uint   `json:"nodeId"`
		Purge       bool   `json:"purge"`
		Receipt     string `json:"receipt,omitempty"`
		CallbackURL string `json:"callbackUrl,omitempty"`
	}{request.NodeID, request.Purge, request.Receipt, request.CallbackURL})
	if err != nil {
		return nil, err
	}
	response, err := client.Post("https://"+net.JoinHostPort(ip, strconv.Itoa(int(port)))+path, "application/json", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("host removal endpoint unavailable (upgrade Node and allow its gRPC port + 1 from Web): %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		if path == "/remove" && response.StatusCode == http.StatusConflict {
			data, readErr := io.ReadAll(io.LimitReader(response.Body, 129))
			if readErr == nil && len(data) <= 128 && strings.TrimSpace(string(data)) == "container_update_active" {
				return nil, ErrHostContainerUpdateActive
			}
		}
		return nil, fmt.Errorf("host removal rejected the request (HTTP %d); Web registration retained", response.StatusCode)
	}
	var result HostRemoval
	if err = json.NewDecoder(io.LimitReader(response.Body, 4096)).Decode(&result); err != nil {
		return nil, err
	}
	if !result.Success || result.NodeID != request.NodeID || result.Purge != request.Purge || len(result.Receipt) != 64 || (request.Receipt != "" && request.Receipt != result.Receipt) {
		return nil, errors.New("invalid host removal result")
	}
	return &result, nil
}
