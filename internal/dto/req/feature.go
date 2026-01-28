package req

type CreateFeatureRequest struct {
	Namespace string `json:"namespace" binding:"required"`
	Env       string `json:"env" binding:"required"`
	Key       string `json:"key" binding:"required"`
	Value     string `json:"value" binding:"required"`
	Type      string `json:"type" binding:"required"`
}

type GetFeatureRequest struct {
	Namespace string `form:"namespace" binding:"required"`
	Env       string `form:"env" binding:"required"`
	Key       string `uri:"key" binding:"required"`
}

type RollbackFeatureRequest struct {
	Namespace string `json:"namespace" binding:"required"`
	Env       string `json:"env" binding:"required"`
	AuditID   uint64 `json:"audit_id" binding:"required"`
}
