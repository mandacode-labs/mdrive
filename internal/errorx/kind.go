package errorx

type Kind int

const (
	Unknown Kind = iota
	NotFound
	Conflict
	BadRequest
	Forbidden
	Unauthenticated
	ServiceDegraded
)

func (k Kind) String() string {
	switch k {
	case NotFound:
		return "not_found"
	case Conflict:
		return "conflict"
	case BadRequest:
		return "bad_request"
	case Forbidden:
		return "forbidden"
	case Unauthenticated:
		return "unauthenticated"
	case ServiceDegraded:
		return "service_degraded"
	default:
		return "unknown"
	}
}

type DomainError interface {
	error
	Kind() Kind
}
