package domain

import "errors"

// 通用领域错误类型，HTTP handler 根据错误类型返回合适的状态码。
var (
	// ErrNotFound 资源不存在
	ErrNotFound = errors.New("资源不存在")

	// ErrConflict 资源冲突（例如重复创建）
	ErrConflict = errors.New("资源冲突")

	// ErrBadRequest 请求参数不合法
	ErrBadRequest = errors.New("请求参数不合法")
)
