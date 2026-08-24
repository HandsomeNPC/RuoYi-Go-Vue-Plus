package errs

// ServiceError 业务异常
type ServiceError struct {
	Code   int
	Msg    string
	Detail string
}

// Error 实现 error
func (e *ServiceError) Error() string {
	return e.Msg
}

// New 构造业务异常，传入业务码、提示、明细三段。
func New(code int, msg, detail string) *ServiceError {
	return &ServiceError{Code: code, Msg: msg, Detail: detail}
}
