package types

// JobStatus define os estados possíveis de um Job.
type JobStatus string

const (
	JobPending    JobStatus = "PENDING"
	JobInProgress JobStatus = "IN_PROGRESS"
	JobCompleted  JobStatus = "COMPLETED"
	JobFailed     JobStatus = "FAILED"
	JobCanceled   JobStatus = "CANCELED"
)

// JobType define os tipos de ações suportadas.
type JobType string

const (
	JobTypeStateChange    JobType = "state_change"
	JobTypeUpdateLimits   JobType = "update_limits"
	JobTypeCreateInstance JobType = "create_instance"
	JobTypeDeleteInstance JobType = "delete_instance"

	// Snapshot Jobs
	JobTypeCreateSnapshot  JobType = "create_snapshot"
	JobTypeRestoreSnapshot JobType = "restore_snapshot"
	JobTypeDeleteSnapshot  JobType = "delete_snapshot"

	// Port Forwarding Jobs
	JobTypeAddPort    JobType = "add_port"
	JobTypeRemovePort JobType = "remove_port"
)

// Constantes de retry
const (
	MaxRetries = 3
	BaseDelay  = 2 // segundos
)
