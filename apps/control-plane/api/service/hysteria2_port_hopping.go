package service

import "strings"

func stringValue(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

func uintValue(value *uint) uint {
	if value == nil {
		return 0
	}
	return *value
}

func normalizedHysteria2PortHopping(value *string) string {
	parts := strings.Split(strings.TrimSpace(stringValue(value)), ",")
	cleaned := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			cleaned = append(cleaned, part)
		}
	}
	return strings.Join(cleaned, ",")
}

func singBoxHysteria2ServerPorts(value *string) []string {
	portHopping := normalizedHysteria2PortHopping(value)
	if portHopping == "" {
		return nil
	}
	parts := strings.Split(portHopping, ",")
	serverPorts := make([]string, 0, len(parts))
	for _, part := range parts {
		if !strings.Contains(part, "-") {
			serverPorts = append(serverPorts, part+":"+part)
			continue
		}
		serverPorts = append(serverPorts, strings.ReplaceAll(part, "-", ":"))
	}
	return serverPorts
}
