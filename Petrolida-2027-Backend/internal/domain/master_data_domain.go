package domain

import (
	MyError "github.com/Petrolida/Petrolida-2027-Backend/internal/pkg/error"
)

const (
	MESSAGE_FAILED_GET_ALL_DIVISIONS         = "failed get all divisions"
	MESSAGE_FAILED_GET_DIVISION              = "failed get division"
	MESSAGE_FAILED_PARSE_DIVISION_ID         = "failed parse division id"
	MESSAGE_FAILED_GET_ALL_SUB_DIVISIONS     = "failed get all sub divisions"
	MESSAGE_FAILED_GET_SUB_DIVISION          = "failed get sub division"
	MESSAGE_FAILED_PARSE_SUB_DIVISION_ID     = "failed parse division id"
	MESSAGE_FAILED_GET_ALL_DEPARTEMENTS      = "failed get all departments"
	MESSAGE_FAILED_GET_DEPARTEMENT           = "failed get department"
	MESSAGE_FAILED_PARSE_DEPARTEMENT_ID      = "failed parse departement id"
	MESSAGE_FAILED_GET_ALL_FACULTIES         = "failed get all faculties"
	MESSAGE_FAILED_GET_FACULTY               = "failed get faculty"
	MESSAGE_FAILED_PARSE_FACULTY_ID          = "failed parse faculty id"
	MESSAGE_FAILED_GET_ALL_PRODUCT_CATEGORY  = "failed get all product category"
	MESSAGE_FAILED_GET_PRODUCT_CATEGORY      = "failed get product category"
	MESSAGE_FAILED_PARSE_PRODUCT_CATEGORY_ID = "failed parse product category id"
	MESSAGE_FAILED_GET_ALL_PBM_BATCH         = "failed get all pbm batch"
	MESSAGE_FAILED_GET_ALL_DIVISIONS_LOVOL   = "failed get all divisions lovol"
	MESSAGE_FAILED_GET_ALL_PETROVIS_SPE      = "failed get all petrovis spe"

	MESSAGE_SUCCESS_GET_ALL_DIVISIONS        = "success get all divisions"
	MESSAGE_SUCCESS_GET_DIVISION             = "success get division"
	MESSAGE_SUCCESS_GET_ALL_SUB_DIVISIONS    = "success get all sub divisions"
	MESSAGE_SUCCESS_GET_SUB_DIVISION         = "success get sub division"
	MESSAGE_SUCCESS_GET_ALL_DEPARTEMENTS     = "success get all departments"
	MESSAGE_SUCCESS_GET_DEPARTEMENT          = "success get department"
	MESSAGE_SUCCESS_PARSE_DEPARTEMENT_ID     = "success parse departement id"
	MESSAGE_SUCCESS_GET_ALL_FACULTIES        = "success get all faculties"
	MESSAGE_SUCCESS_GET_FACULTY              = "success get faculty"
	MESSAGE_SUCCESS_PARSE_FACULTY_ID         = "success parse faculty id"
	MESSAGE_SUCCESS_GET_ALL_PRODUCT_CATEGORY = "success get all product category"
	MESSAGE_SUCCESS_GET_PRODUCT_CATEGORY     = "success get product category"
	MESSAGE_SUCCESS_GET_ALL_PBM_BATCH        = "success get all pbm batch"
	MESSAGE_SUCCESS_GET_ALL_DIVISIONS_LOVOL  = "success get all divisions lovol"
	MESSAGE_SUCCESS_GET_ALL_PETROVIS_SPE     = "success get all petrovis spe"
)

var (
	ErrDivisionIdNotFound   = MyError.New(MyError.TypeValidation, "DIVISION_NOT_FOUND", "division Id not found")
	ErrSubdivsionIdNotFound = MyError.New(MyError.TypeValidation, "SUBDIVISION_NOT_FOUND", "subdivision id not found")
	ErrDepartmentIdNotFound = MyError.New(MyError.TypeValidation, "DEPARTMENT_NOT_FOUND", "department id not found")
	ErrFacultyIdNotFound    = MyError.New(MyError.TypeValidation, "FACULTY_NOT_FOUND", "faculty id not found")
)
