package models

type AuditLog struct {
	ID            string `json:"id"`
	ActorUserID   string `json:"actorUserId,omitempty"`
	ActorUsername string `json:"actorUsername,omitempty"`
	Action        string `json:"action"`
	ResourceType  string `json:"resourceType"`
	ResourceID    string `json:"resourceId,omitempty"`
	OldValues     string `json:"oldValues,omitempty"`
	NewValues     string `json:"newValues,omitempty"`
	IPAddress     string `json:"ipAddress,omitempty"`
	Success       bool   `json:"success"`
	CreatedAt     string `json:"createdAt"`
}
