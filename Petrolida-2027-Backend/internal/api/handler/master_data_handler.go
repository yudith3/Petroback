package handler

import (
	"github.com/Petrolida/Petrolida-2027-Backend/internal/api/service"
	"github.com/Petrolida/Petrolida-2027-Backend/internal/domain"
	pagination "github.com/Petrolida/Petrolida-2027-Backend/internal/pkg/meta"
	"github.com/Petrolida/Petrolida-2027-Backend/internal/pkg/response"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type (
	MasterDataHandler interface {
		GetsDivisions(ctx *gin.Context)
		GetDivisionByID(ctx *gin.Context)
		GetSubDivisionByID(ctx *gin.Context)
		GetSubDivisions(ctx *gin.Context)
		GetDepartements(ctx *gin.Context)
		GetDepartmentByID(ctx *gin.Context)
		GetFaculties(ctx *gin.Context)
		GetFacultyByID(ctx *gin.Context)
	}

	masterDataHandler struct {
		masterDataService service.MasterDataService
	}
)

func NewMasterData(masterDataService service.MasterDataService) MasterDataHandler {
	return &masterDataHandler{
		masterDataService: masterDataService,
	}
}

func (h *masterDataHandler) GetsDivisions(ctx *gin.Context) {
	res, err := h.masterDataService.GetsDivisions()
	if err != nil {
		response.BuildResponseFailed(domain.MESSAGE_FAILED_GET_ALL_DIVISIONS, err).Send(ctx)
		return
	}

	response.BuildResponseSuccess(domain.MESSAGE_SUCCESS_GET_ALL_DIVISIONS, res, pagination.Meta{}).Send(ctx)
}

func (h *masterDataHandler) GetDivisionByID(ctx *gin.Context) {
	idParam := ctx.Param(domain.CTX_ID_PARAM)
	if idParam == "" {
		response.BuildResponseFailed(domain.MESSAGE_FAILED_ID_REQUIRED, domain.ErrIdParamRequired).Send(ctx)
		return
	}

	id, err := uuid.Parse(idParam)
	if err != nil {
		response.BuildResponseFailed(domain.MESSAGE_FAILED_GET_DIVISION, domain.ErrParseValue).Send(ctx)
		return
	}

	res, err := h.masterDataService.GetDivisionByID(id)
	if err != nil {
		response.BuildResponseFailed(domain.MESSAGE_FAILED_GET_DIVISION, err).Send(ctx)
		return
	}

	response.BuildResponseSuccess(domain.MESSAGE_SUCCESS_GET_DIVISION, res, pagination.Meta{}).Send(ctx)
}

func (h *masterDataHandler) GetSubDivisionByID(ctx *gin.Context) {
	idParam := ctx.Param(domain.CTX_ID_PARAM)
	if idParam == "" {
		response.BuildResponseFailed(domain.MESSAGE_FAILED_ID_REQUIRED, domain.ErrIdParamRequired).Send(ctx)
		return
	}

	id, err := uuid.Parse(idParam)
	if err != nil {
		response.BuildResponseFailed(domain.MESSAGE_FAILED_GET_SUB_DIVISION, err).Send(ctx)
		return
	}

	res, err := h.masterDataService.GetSubDivisionByID(id)
	if err != nil {
		response.BuildResponseFailed(domain.MESSAGE_FAILED_GET_SUB_DIVISION, err).Send(ctx)
		return
	}

	response.BuildResponseSuccess(domain.MESSAGE_SUCCESS_GET_SUB_DIVISION, res, pagination.Meta{}).Send(ctx)
}

func (h *masterDataHandler) GetSubDivisions(ctx *gin.Context) {
	res, err := h.masterDataService.GetSubDivisions()
	if err != nil {
		response.BuildResponseFailed(domain.MESSAGE_FAILED_GET_ALL_SUB_DIVISIONS, err).Send(ctx)
		return
	}

	response.BuildResponseSuccess(domain.MESSAGE_SUCCESS_GET_ALL_SUB_DIVISIONS, res, pagination.Meta{}).Send(ctx)
}

func (h *masterDataHandler) GetDepartements(ctx *gin.Context) {
	res, err := h.masterDataService.GetDepartments()
	if err != nil {
		response.BuildResponseFailed(domain.MESSAGE_FAILED_GET_ALL_DEPARTEMENTS, err).Send(ctx)
		return
	}

	response.BuildResponseSuccess(domain.MESSAGE_SUCCESS_GET_ALL_DEPARTEMENTS, res, pagination.Meta{}).Send(ctx)
}

func (h *masterDataHandler) GetDepartmentByID(ctx *gin.Context) {
	idParam := ctx.Param(domain.CTX_ID_PARAM)
	if idParam == "" {
		response.BuildResponseFailed(domain.MESSAGE_FAILED_ID_REQUIRED, domain.ErrIdParamRequired).Send(ctx)
		return
	}

	id, err := uuid.Parse(idParam)
	if err != nil {
		response.BuildResponseFailed(domain.MESSAGE_FAILED_PARSE_PARAM, domain.ErrParseValue).Send(ctx)
		return
	}

	res, err := h.masterDataService.GetDepartmentByID(id)
	if err != nil {
		response.BuildResponseFailed(domain.MESSAGE_FAILED_GET_DEPARTEMENT, err).Send(ctx)
		return
	}

	response.BuildResponseSuccess(domain.MESSAGE_FAILED_GET_DEPARTEMENT, res, pagination.Meta{}).Send(ctx)
}

func (h *masterDataHandler) GetFaculties(ctx *gin.Context) {
	res, err := h.masterDataService.GetFaculties()
	if err != nil {
		response.BuildResponseFailed(domain.MESSAGE_FAILED_GET_ALL_FACULTIES, err).Send(ctx)
		return
	}

	response.BuildResponseSuccess(domain.MESSAGE_SUCCESS_GET_ALL_FACULTIES, res, pagination.Meta{}).Send(ctx)
}

func (h *masterDataHandler) GetFacultyByID(ctx *gin.Context) {
	idParam := ctx.Param(domain.CTX_ID_PARAM)
	if idParam == "" {
		response.BuildResponseFailed(domain.MESSAGE_FAILED_ID_REQUIRED, domain.ErrIdParamRequired).Send(ctx)
		return
	}

	id, err := uuid.Parse(idParam)
	if err != nil {
		response.BuildResponseFailed(domain.MESSAGE_FAILED_PARSE_PARAM, domain.ErrParseValue).Send(ctx)
		return
	}

	res, err := h.masterDataService.GetFacultyByID(id)
	if err != nil {
		response.BuildResponseFailed(domain.MESSAGE_FAILED_GET_FACULTY, err).Send(ctx)
		return
	}

	response.BuildResponseSuccess(domain.MESSAGE_SUCCESS_GET_FACULTY, res, pagination.Meta{}).Send(ctx)
}
