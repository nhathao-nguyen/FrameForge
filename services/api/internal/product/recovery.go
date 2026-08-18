package product

import "context"

type RecoveryReport struct {
	ReadyStepsRequeued int `json:"ready_steps_requeued"`
	RetryStepsRequeued int `json:"retry_steps_requeued"`
}

type RecoverySurface interface {
	Reconcile(context.Context) (RecoveryReport, error)
}
