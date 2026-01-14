package common

import (
	"common/biz"
	"framework/msError"
	"net/http"

	"github.com/gin-gonic/gin"
)

type Result struct {
	Code int `json:"code"`
	Msg  any `json:"msg"`
}

func Success(ctx *gin.Context, data any) {
	ctx.JSON(http.StatusOK, Result{Code: biz.OK, Msg: data})
}

func Fail(ctx *gin.Context, err *msError.Error) {
	ctx.JSON(http.StatusOK, Result{Code: err.Code, Msg: err.Error()})
}

func SuccessNoCtx(data any) Result {
	return Result{
		Code: biz.OK,
		Msg:  data,
	}
}

func FailNoCtx(err *msError.Error) Result {
	return Result{
		Code: err.Code,
		Msg:  err.Error(),
	}
}
