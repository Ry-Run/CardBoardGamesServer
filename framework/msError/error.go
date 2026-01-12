package msError

import (
	"errors"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type Error struct {
	Code int
	Err  error
}

func (e *Error) Error() string {
	return e.Err.Error()
}

func NewError(code int, err error) *Error {
	return &Error{Code: code, Err: err}
}

// 通过 grpc 提供的 status.Error 转化为 grpc error
func GrpcError(err *Error) error {
	return status.Error(codes.Code(err.Code), err.Error())
}

// 转换 grpc error 到 返回客户端的 error
func ToError(err error) *Error {
	s, _ := status.FromError(err)
	return NewError(int(s.Code()), errors.New(s.Message()))
}
