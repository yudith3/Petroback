package response

import (
	"errors"
	"net/http"

	myerror "github.com/Petrolida/Petrolida-2027-Backend/internal/pkg/error"
	pagination "github.com/Petrolida/Petrolida-2027-Backend/internal/pkg/meta"
	"github.com/gin-gonic/gin"
)

type Response struct {
	StatusCode int    `json:"-"`
	Status     bool   `json:"status"`
	Message    string `json:"message"`
	Error      any    `json:"error,omitempty"`
	Data       any    `json:"data,omitempty"`
	Meta       any    `json:"meta,omitempty"`
}

type EmptyObj struct{}

var statusByErrorType = map[myerror.Type]int{
	myerror.TypeValidation:     http.StatusBadRequest,
	myerror.TypeAuthentication: http.StatusUnauthorized,
	myerror.TypeAuthorization:  http.StatusForbidden,
	myerror.TypeNotFound:       http.StatusNotFound,
	myerror.TypeConflict:       http.StatusConflict,
	myerror.TypeInternal:       http.StatusInternalServerError,
}

func BuildResponseSuccess(
	message string,
	data any,
	meta ...pagination.Meta,
) Response {
	res := Response{
		StatusCode: http.StatusOK,
		Status:     true,
		Message:    message,
		Data:       data,
	}

	if len(meta) > 0 {
		res.Meta = meta[0]
	}

	return res
}

func BuildResponseFailed(
	message string,
	err error,
) Response {
	return Response{
		StatusCode: getErrorStatus(err),
		Status:     false,
		Message:    message,
		Error:      err,
	}
}

func getErrorStatus(err error) int {
	var myErr myerror.Error

	if !errors.As(err, &myErr) {
		return http.StatusInternalServerError
	}

	if status, ok := statusByErrorType[myErr.Type]; ok {
		return status
	}

	return http.StatusInternalServerError
}

func (r Response) SetStatus(statusCode int) Response {
	r.StatusCode = statusCode
	return r
}

func (r Response) WithMeta(meta any) Response {
	r.Meta = meta
	return r
}

func (r Response) Send(ctx *gin.Context) {
	ctx.JSON(r.StatusCode, r)
}

func (r Response) SendWithAbort(ctx *gin.Context) {
	ctx.AbortWithStatusJSON(r.StatusCode, r)
}
