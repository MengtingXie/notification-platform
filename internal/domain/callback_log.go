package domain

type CallbackLogStatus string

const (
	CallbackLogStatusPENDING CallbackLogStatus = "PENDING"
	CallbackLogStatusSuccess CallbackLogStatus = "SUCCESS"
	CallbackLogStatusFAILED  CallbackLogStatus = "FAILED"
)

type CallbackLog struct {
	ID            int64
	Notification  Notification
	RetryCount    int32
	NextRetryTime int64
	Status        CallbackLogStatus
}
