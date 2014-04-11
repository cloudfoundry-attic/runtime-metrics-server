package health_check

type HealthCheck struct{}

func New() *HealthCheck {
	return &HealthCheck{}
}

func (h *HealthCheck) Ok() bool {
	return true
}
