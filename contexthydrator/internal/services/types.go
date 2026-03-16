package services

import "encoding/json"

type ServiceName string

const (
	ServiceProfile     ServiceName = "profile"
	ServicePreferences ServiceName = "preferences"
	ServicePermissions ServiceName = "permissions"
	ServiceResources   ServiceName = "resources"
)

var AllServices = []ServiceName{
	ServiceProfile,
	ServicePreferences,
	ServicePermissions,
	ServiceResources,
}

type ServiceResult struct {
	Service ServiceName
	Data    json.RawMessage
	Err     error
}
