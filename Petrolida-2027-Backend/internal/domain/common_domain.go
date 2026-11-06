package domain

import (
	MyError "github.com/Petrolida/Petrolida-2027-Backend/internal/pkg/error"
)

const (
	JWT_EXPIRE_TIME_IN_MINUTES = 180

	CTX_ID_PARAM      = "id"
	CTX_KEY_TOKEN     = "TOKEN"
	CTX_KEY_USER_ID   = "user_id"
	CTX_KEY_ROLE_NAME = "role"

	ROLE_ADMIN = "ADMIN"

	MESSAGE_TO_MANY_REQUEST           = "Too many request"
	MESSAGE_FAILED_PARSE_TIME         = "Failed parse time"
	MESSAGE_INVALID_NAME_FORMAT       = "Invalid name format"
	MESSAGE_FAILED_ID_REQUIRED        = "id is required"
	MESSAGE_FAILED_GET_DATA_FROM_BODY = "failed to get data from body"
	MESSAGE_FAILED_PARSE_PARAM        = "failed parse param"
	MESSAGE_SERVER_PANIC_OCCURED      = "server panic occured"

	MESSAGE_FAILED_CONVERT_STATUS = "failed status invalid"

	PETROVIS_EVENT_ID    = "1e2d6522-8339-4a92-9bf4-7453ddaa7a29"
	OPREC_LOVOL_EVENT_ID = "02bcc63a-d017-4d07-826a-408003de6473"
)

var (
	ErrGeneral            = MyError.New(MyError.TypeInternal, "INTERNAL_SERVER_ERROR", "terjadi kesalahan")
	ErrInvalidInput       = MyError.New(MyError.TypeValidation, "INVALID_INPUT", "input tidak valid")
	ErrMakeMail           = MyError.New(MyError.TypeInternal, "MAKE_MAIL_FAILED", "failed generate email")
	ErrSendMail           = MyError.New(MyError.TypeInternal, "SEND_MAIL_FAILED", "failed send email")
	ErrRouteNotFound      = MyError.New(MyError.TypeNotFound, "ROUTE_NOT_FOUND", "route not found")
	ErrToManyRequest      = MyError.New(MyError.TypeValidation, "TOO_MANY_REQUESTS", "Terlalu banyak permintaan")
	ErrInvalidNameFormat  = MyError.New(MyError.TypeValidation, "INVALID_NAME_FORMAT", MESSAGE_INVALID_NAME_FORMAT)
	ErrIdParamRequired    = MyError.New(MyError.TypeValidation, "ID_PARAM_REQUIRED", "id param is required")
	ErrParseValue         = MyError.New(MyError.TypeValidation, "INVALID_VALUE_FORMAT", "invalid value format")
	ErrInvalidRequestBody = MyError.New(MyError.TypeValidation, "INVALID_REQUEST_BODY", "invalid request body")
	ErrInvalidStatus      = MyError.New(MyError.TypeValidation, "INVALID_STATUS_CODE", "invalid status code")
	ErrUploadFile         = MyError.New(MyError.TypeValidation, "ERROR_UPLOAD_FILE", "error upload file to aws")
)
