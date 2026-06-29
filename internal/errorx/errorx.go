package errorx

type Kind int

const (
	KindUnknown Kind = iota
	KindNotFound
	KindConflict
	KindBadRequest
	KindForbidden
	KindUnauthenticated
	KindServiceDegraded
)

func (k Kind) String() string {
	switch k {
	case KindNotFound:
		return "not_found"
	case KindConflict:
		return "conflict"
	case KindBadRequest:
		return "bad_request"
	case KindForbidden:
		return "forbidden"
	case KindUnauthenticated:
		return "unauthorized"
	case KindServiceDegraded:
		return "service_degraded"
	default:
		return "unknown"
	}
}

type Error interface {
	error
	Kind() Kind
}

type errorx struct {
	kind    Kind
	message string
}

func (e *errorx) Error() string {
	return e.message
}

func (e *errorx) Kind() Kind {
	return e.kind
}

func New(kind Kind, message string) Error {
	return &errorx{
		kind:    kind,
		message: message,
	}
}
