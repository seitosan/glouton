package types

import (
	"crypto/sha256"
	"fmt"
	"time"
)

// AgentFact is an agent facts
type AgentFact struct {
	ID    string
	Key   string
	Value string
}

// Agent is an Agent object on Bleemeo API
type Agent struct {
	ID              string    `json:"id"`
	CreatedAt       time.Time `json:"created_at"`
	AccountID       string    `json:"account"`
	NextConfigAt    time.Time `json:"next_config_at"`
	CurrentConfigID string    `json:"current_config"`
}

// AccountConfig is the configuration used by this agent
type AccountConfig struct {
	ID                    string `json:"id"`
	Name                  string `json:"name"`
	MetricsAgentWhitelist string `json:"metrics_agent_whitelist"`
	MetricAgentResolution int    `json:"metrics_agent_resolution"`
	LiveProcessResolution int    `json:"live_process_resolution"`
	DockerIntegration     bool   `json:"docker_integration"`
}

// Service is a Service object on Bleemeo API
type Service struct {
	ID              string `json:"id"`
	Label           string `json:"label"`
	Instance        string `json:"instance"`
	ListenAddresses string `json:"listen_addresses"`
	ExePath         string `json:"exe_path"`
	Stack           string `json:"stack"`
	Active          bool   `json:"active"`
}

// Container is a Contaier object on Bleemeo API
type Container struct {
	ID                string `json:"id"`
	Name              string `json:"name"`
	DockerID          string `json:"docker_id"`
	DockerInspect     string `json:"docker_inspect"`
	DockerInspectHash string `json:",omitempty"`
}

// Metric is a Metric object on Bleemeo API
type Metric struct {
	ID                     string            `json:"id"`
	Label                  string            `json:"label"`
	Labels                 map[string]string `json:"labels"`
	ServiceID              string            `json:"service,omitempty"`
	ContainerID            string            `json:"container,omitempty"`
	StatusOf               string            `json:"status_of,omitempty"`
	ThresholdLowWarning    float64           `json:"threshold_low_warning,omitempty"`
	ThresholdLowCrictical  float64           `json:"threshold_low_critical,omitempty"`
	ThresholdHighWarning   float64           `json:"threshold_high_warning,omitempty"`
	ThresholdHighCrictical float64           `json:"threshold_high_critical,omitempty"`
	Unit                   int               `json:"unit,omitempty"`
	UnitText               string            `json:"unit_text,omitempty"`
	DeactivatedAt          time.Time         `json:"deactivated_at,omitempty"`
}

// FillInspectHash fill the DockerInspectHash
func (c *Container) FillInspectHash() {
	bin := sha256.Sum256([]byte(c.DockerInspect))
	c.DockerInspectHash = fmt.Sprintf("%x", bin)
}