package core

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"
)

const HostContainerTimeout = 5 * time.Second
const hostContainerResponseLimit = 64 * 1024
const hostContainerImageRepository = "ghcr.io/1linhao/trojanpanelnext-node-agent:"

var ErrHostContainerUnsupported = errors.New("Node container management is unavailable on this host; upgrade Node and the host maintenance service using CLI first")
var ErrHostContainerUpdateActive = errors.New("Node container update is active; wait for it to finish before managing this server")
var containerVersionPattern = regexp.MustCompile(`^[1-9][0-9]*\.(0|[1-9][0-9]*)(\.(0|[1-9][0-9]*))?(-rc\.[1-9][0-9]*)?$`)
var containerJobIDPattern = regexp.MustCompile(`^[a-f0-9]{64}$`)

type HostContainerJob struct {
	ID            string `json:"id"`
	FromVersion   string `json:"fromVersion"`
	TargetVersion string `json:"targetVersion"`
	Status        string `json:"status"`
	Error         string `json:"error"`
	StartedAt     string `json:"startedAt"`
	FinishedAt    string `json:"finishedAt"`
}

type HostContainerInventory struct {
	NodeID          uint              `json:"nodeId"`
	CurrentVersion  string            `json:"currentVersion"`
	Image           string            `json:"image"`
	UpdateSupported bool              `json:"updateSupported"`
	Job             *HostContainerJob `json:"job"`
}

func ValidContainerVersion(version string) bool {
	return len(version) <= 64 && containerVersionPattern.MatchString(version)
}

func ValidateHostContainerJob(job *HostContainerJob) error {
	if job == nil || !containerJobIDPattern.MatchString(job.ID) || !ValidContainerVersion(job.FromVersion) || !ValidContainerVersion(job.TargetVersion) || len(job.Error) > 8192 {
		return errors.New("invalid Node container job")
	}
	switch job.Status {
	case "queued", "running", "succeeded", "failed":
	default:
		return errors.New("invalid Node container job status")
	}
	for _, value := range []string{job.StartedAt, job.FinishedAt} {
		if value != "" {
			if _, err := time.Parse(time.RFC3339, value); err != nil {
				return errors.New("invalid Node container job timestamp")
			}
		}
	}
	if job.Status == "running" && job.StartedAt == "" || (job.Status == "succeeded" || job.Status == "failed") && job.FinishedAt == "" {
		return errors.New("incomplete Node container job timestamps")
	}
	return nil
}

func GetHostContainerInventory(ctx context.Context, ip string, port uint, transport NodeTransport, nodeID uint) (*HostContainerInventory, error) {
	var result HostContainerInventory
	if err := hostContainerCall(ctx, ip, port, transport, "/container/inventory", struct {
		NodeID uint `json:"nodeId"`
	}{nodeID}, &result); err != nil {
		return nil, err
	}
	if result.NodeID != nodeID || !ValidContainerVersion(result.CurrentVersion) || result.Image != hostContainerImageRepository+result.CurrentVersion || !result.UpdateSupported {
		return nil, errors.New("invalid Node container inventory")
	}
	if result.Job != nil {
		if err := ValidateHostContainerJob(result.Job); err != nil {
			return nil, err
		}
	}
	return &result, nil
}

func UpdateHostContainer(ctx context.Context, ip string, port uint, transport NodeTransport, nodeID uint, version string) (*HostContainerJob, error) {
	if !ValidContainerVersion(version) {
		return nil, errors.New("invalid Node container target version")
	}
	var result HostContainerJob
	request := struct {
		NodeID  uint   `json:"nodeId"`
		Version string `json:"version"`
	}{nodeID, version}
	if err := hostContainerCall(ctx, ip, port, transport, "/container/update", request, &result); err != nil {
		return nil, err
	}
	if err := ValidateHostContainerJob(&result); err != nil {
		return nil, err
	}
	if result.TargetVersion != version {
		return nil, errors.New("Node container job target does not match the Web release")
	}
	return &result, nil
}

func hostContainerCall(ctx context.Context, ip string, port uint, transport NodeTransport, path string, body any, result any) error {
	if transport.Mode != "mtls" || transport.ServerName == "" || ip == "" || strings.ContainsAny(ip, "/@?#\\ \t\x00\r\n") || port == 0 || port > 65535 {
		return errors.New("Node container management requires a registered mTLS endpoint")
	}
	payload, err := json.Marshal(body)
	if err != nil {
		return err
	}
	var identity struct {
		NodeID uint `json:"nodeId"`
	}
	if json.Unmarshal(payload, &identity) != nil || identity.NodeID == 0 {
		return errors.New("a valid Node ID is required")
	}
	tlsConfig, err := clientTLSConfig(transport.ServerName)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(ctx, HostContainerTimeout)
	defer cancel()
	clientTransport := &http.Transport{TLSClientConfig: tlsConfig, Proxy: nil, DialContext: (&net.Dialer{Timeout: HostContainerTimeout}).DialContext, TLSHandshakeTimeout: HostContainerTimeout}
	defer clientTransport.CloseIdleConnections()
	client := &http.Client{Transport: clientTransport, Timeout: HostContainerTimeout, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://"+net.JoinHostPort(ip, strconv.Itoa(int(port)))+path, bytes.NewReader(payload))
	if err != nil {
		return err
	}
	request.Header.Set("Content-Type", "application/json")
	response, err := client.Do(request)
	if err != nil {
		return fmt.Errorf("Node host maintenance endpoint unavailable (check its gRPC port + 1 and mTLS): %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode == http.StatusNotFound || response.StatusCode == http.StatusMethodNotAllowed {
		return ErrHostContainerUnsupported
	}
	if response.StatusCode != http.StatusOK && !(path == "/container/update" && response.StatusCode == http.StatusAccepted) {
		return fmt.Errorf("Node container request rejected (HTTP %d); check active operations and the host maintenance service", response.StatusCode)
	}
	data, err := io.ReadAll(io.LimitReader(response.Body, hostContainerResponseLimit+1))
	if err != nil {
		return err
	}
	if len(data) > hostContainerResponseLimit {
		return errors.New("Node container response exceeds its size limit")
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(result); err != nil {
		return errors.New("invalid Node container response")
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return errors.New("invalid trailing Node container response")
	}
	return nil
}
