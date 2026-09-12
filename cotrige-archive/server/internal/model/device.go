package model

type DeviceStatus string

type CollectorStatus string

const (
	DeviceStatusReady    DeviceStatus = "Ready"
	DeviceStatusNotReady DeviceStatus = "NotReady"
	DeviceStatusUnknown  DeviceStatus = "Unknown"
)

const (
	CollectorStatusRunning        CollectorStatus = "Running"
	CollectorStatusPending        CollectorStatus = "Pending"
	CollectorStatusCrashLoop      CollectorStatus = "CrashLoopBackOff"
	CollectorStatusNotFound       CollectorStatus = "NotFound"
	CollectorStatusUnknown        CollectorStatus = "Unknown"
	CollectorStatusImagePullError CollectorStatus = "ImagePullBackOff"
)

type DeviceItem struct {
	NodeName        string          `json:"node_name"`
	Equipment       string          `json:"equipment"`
	Site            string          `json:"site"`
	Status          DeviceStatus    `json:"status"`
	CollectorStatus CollectorStatus `json:"collector_status"`
	LastEventTime   *string         `json:"last_event_time"`
	IngestRate1m    int             `json:"ingest_rate_1m"`
}

type ListDevicesResponse struct {
	Items       []DeviceItem `json:"items"`
	Total       int          `json:"total"`
	GeneratedAt string       `json:"generated_at"`
}
