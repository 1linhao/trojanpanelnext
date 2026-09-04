package clientcompat

import (
	"errors"
	"strings"
	"trojan-panel/model/constant"
)

var supportedClients = []string{
	constant.ClientSingBox,
	constant.ClientClashMeta,
	constant.ClientV2Ray,
	constant.ClientShadowrocket,
}

func DefaultClients() []string {
	return append([]string(nil), supportedClients...)
}

func Decode(value *string) []string {
	if value == nil {
		return DefaultClients()
	}
	if *value == "" {
		return []string{}
	}
	return normalize(strings.Split(*value, ","))
}

func Encode(clients *[]string) (string, error) {
	if clients == nil {
		return constant.DefaultNodeClientTypes, nil
	}
	if !valid(*clients) {
		return "", errors.New(constant.ValidateFailed)
	}
	return strings.Join(normalize(*clients), ","), nil
}

func Includes(value *string, client string) bool {
	for _, item := range Decode(value) {
		if item == client {
			return true
		}
	}
	return false
}

func ValidateNode(nodeTypeId *uint, clients *[]string) error {
	if nodeTypeId == nil {
		return nil
	}
	selected := DefaultClients()
	if clients != nil {
		selected = *clients
	}
	if *nodeTypeId == constant.NaiveProxy && contains(selected, constant.ClientClashMeta) {
		return errors.New(constant.NodeClientUnsupported)
	}
	return nil
}

func valid(clients []string) bool {
	for _, client := range clients {
		if !contains(supportedClients, client) {
			return false
		}
	}
	return true
}

func normalize(clients []string) []string {
	result := make([]string, 0, len(supportedClients))
	for _, supported := range supportedClients {
		if contains(clients, supported) {
			result = append(result, supported)
		}
	}
	return result
}

func contains(values []string, expected string) bool {
	for _, value := range values {
		if value == expected {
			return true
		}
	}
	return false
}
