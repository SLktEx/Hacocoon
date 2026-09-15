package hostsetup

// NotificationServiceFailure reports only a closed operation name. It does not
// carry subprocess output, Windows paths, credentials or command arguments.
type NotificationServiceFailure struct{ Operation string }

func (e *NotificationServiceFailure) reason() string {
	if e == nil {
		return "failed"
	}
	switch e.Operation {
	case "enable_state", "activity", "disable", "reload", "failure_state", "reset", "enable", "restart":
		return "notification_" + e.Operation + "_failed"
	default:
		return "failed"
	}
}

func (e *NotificationServiceFailure) Error() string { return e.reason() }
