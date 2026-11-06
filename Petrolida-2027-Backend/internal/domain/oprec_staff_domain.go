package domain

import (
	"time"

	myerror "github.com/Petrolida/Petrolida-2027-Backend/internal/pkg/error"
	"github.com/google/uuid"
)

const (
	DocumentTypeRequiredDocuments = "required_documents"
	DocumentTypePortfolio         = "portfolio"
)

var (
	ErrRegistrationClosed      = myerror.New(myerror.TypeAuthorization, "REGISTRATION_CLOSED", "registration is not open")
	ErrSelectionFinalized      = myerror.New(myerror.TypeConflict, "OPREC_SELECTION_FINALIZED", "staff selection is finalized; registration data is read-only")
	ErrRegistrationExists      = myerror.New(myerror.TypeConflict, "REGISTRATION_EXISTS", "email or NRP is already registered")
	ErrSubmissionKeyConflict   = myerror.New(myerror.TypeConflict, "SUBMISSION_KEY_CONFLICT", "this submission was already saved with different answers; check its confirmation before submitting again")
	ErrRegistrantNotFound      = myerror.New(myerror.TypeNotFound, "REGISTRANT_NOT_FOUND", "registrant not found")
	ErrDocumentNotFound        = myerror.New(myerror.TypeNotFound, "DOCUMENT_NOT_FOUND", "document not found")
	ErrInvalidCredentials      = myerror.New(myerror.TypeAuthentication, "INVALID_CREDENTIALS", "email or password is incorrect")
	ErrUnauthenticated         = myerror.New(myerror.TypeAuthentication, "UNAUTHENTICATED", "admin session is required")
	ErrForbidden               = myerror.New(myerror.TypeAuthorization, "FORBIDDEN", "admin access is required")
	ErrInvalidRegistrantStatus = myerror.New(myerror.TypeValidation, "INVALID_REGISTRANT_STATUS", "registrant status is invalid")
	ErrInvalidStatusTransition = myerror.New(myerror.TypeValidation, "INVALID_STATUS_TRANSITION", "registrant status transition is not allowed")
	ErrDivisionNotFound        = myerror.New(myerror.TypeNotFound, "DIVISION_NOT_FOUND", "accepted division not found or inactive")
)

type RegistrationChoiceInput struct {
	Priority           int16  `json:"priority"`
	SubdivisionID      string `json:"subdivision_id"`
	AboutDivision      string `json:"about_division"`
	ReasonToJoin       string `json:"reason_to_join"`
	InnovationIdea     string `json:"innovation_idea"`
	PreviousExperience string `json:"previous_experience"`
}

type RegistrationSkillInput struct {
	EnglishProficiency string `json:"english_proficiency"`
	CommunicationSkill string `json:"communication_skill"`
	CreativityInEvent  string `json:"creativity_in_event"`
	FastResponse       string `json:"fast_response"`
	GraphicDesign      string `json:"graphic_design"`
	Photography        string `json:"photography"`
	VideoEditing       string `json:"video_editing"`
	WebDesign          string `json:"web_design"`
	OperatingZoom      string `json:"operating_zoom"`
	SocialMediaTrends  string `json:"social_media_trends"`
	DesignApplications string `json:"design_applications"`
}

type RegistrationAdditionalInformationInput struct {
	HasCarInSurabaya    *bool  `json:"has_car_in_surabaya"`
	HasSponsorRelation  *bool  `json:"has_sponsor_relation"`
	HasVendorInSurabaya *bool  `json:"has_vendor_in_surabaya"`
	HasCamera           *bool  `json:"has_camera"`
	HasLaptop           *bool  `json:"has_laptop"`
	HasTablet           *bool  `json:"has_tablet"`
	ConnectedCompany    string `json:"connected_company"`
}

type RegistrationInput struct {
	SubmissionKey           string                                 `json:"submission_key,omitempty"`
	Name                    string                                 `json:"name"`
	Email                   string                                 `json:"email"`
	NRP                     string                                 `json:"nrp"`
	PhoneNumber             string                                 `json:"phone_number"`
	LineID                  string                                 `json:"line_id"`
	FacultyID               string                                 `json:"faculty_id"`
	DepartmentID            string                                 `json:"department_id"`
	Batch                   int16                                  `json:"batch"`
	AboutPetrolida          string                                 `json:"about_petrolida"`
	WillingTransferDivision *bool                                  `json:"willing_transfer_division"`
	Choices                 []RegistrationChoiceInput              `json:"choices"`
	Skills                  RegistrationSkillInput                 `json:"skills"`
	AdditionalInfo          RegistrationAdditionalInformationInput `json:"additional_info"`
	RequiredDocumentURL     string                                 `json:"required_document_url"`
}

type RegistrationResult struct {
	RegistrationID uuid.UUID `json:"registration_id"`
	Status         string    `json:"status"`
}

type RegistrationConfirmation struct {
	State        string              `json:"state"`
	Registration *RegistrationResult `json:"registration,omitempty"`
}

type UpdateRegistrantStatusInput struct {
	Status               string  `json:"status"`
	RejectionTarget      string  `json:"rejection_target"`
	AcceptedDivisionID   *string `json:"accepted_division_id"`
	CurrentSubdivisionID *string `json:"current_subdivision_id"`
}

type UpdateRegistrantStatusResult struct {
	RegistrationID               uuid.UUID  `json:"registration_id"`
	PreviousStatus               string     `json:"previous_status"`
	Status                       string     `json:"status"`
	PrimaryRejected              bool       `json:"primary_rejected"`
	SecondaryRejected            bool       `json:"secondary_rejected"`
	PreviousDivisionID           *uuid.UUID `json:"previous_division_id,omitempty"`
	AcceptedDivisionID           *uuid.UUID `json:"accepted_division_id,omitempty"`
	PreviousCurrentSubdivisionID *uuid.UUID `json:"previous_current_subdivision_id,omitempty"`
	CurrentSubdivisionID         *uuid.UUID `json:"current_subdivision_id,omitempty"`
}

type OprecConfigResponse struct {
	SelectionFinalized bool       `json:"selection_finalized"`
	RegistrationOpen   bool       `json:"registration_open"`
	RegistrationStart  *time.Time `json:"registration_start_at"`
	RegistrationEnd    *time.Time `json:"registration_end_at"`
	AnnouncementOpen   bool       `json:"announcement_open"`
	AnnouncementAt     *time.Time `json:"announcement_at"`
	ServerTime         time.Time  `json:"server_time"`
	PortfolioMaxBytes  int64      `json:"portfolio_max_bytes"`
	AllowedBatches     []int16    `json:"allowed_batches"`
	RequiredDocuments  []string   `json:"required_document_types"`
	PortfolioRequired  []string   `json:"portfolio_required_subdivisions"`
}

type MasterReference struct {
	ID   uuid.UUID `json:"id"`
	Name string    `json:"name"`
}

type ChoiceResponse struct {
	ID                 uuid.UUID        `json:"id"`
	Priority           int16            `json:"priority"`
	SubdivisionID      uuid.UUID        `json:"subdivision_id"`
	Subdivision        *MasterReference `json:"subdivision,omitempty"`
	Division           *MasterReference `json:"division,omitempty"`
	AboutDivision      string           `json:"about_division"`
	ReasonToJoin       string           `json:"reason_to_join"`
	InnovationIdea     string           `json:"innovation_idea"`
	PreviousExperience *string          `json:"previous_experience,omitempty"`
}

type SkillResponse struct {
	EnglishProficiency string `json:"english_proficiency"`
	CommunicationSkill string `json:"communication_skill"`
	CreativityInEvent  string `json:"creativity_in_event"`
	FastResponse       string `json:"fast_response"`
	GraphicDesign      string `json:"graphic_design"`
	Photography        string `json:"photography"`
	VideoEditing       string `json:"video_editing"`
	WebDesign          string `json:"web_design"`
	OperatingZoom      string `json:"operating_zoom"`
	SocialMediaTrends  string `json:"social_media_trends"`
	DesignApplications string `json:"design_applications"`
}

type AdditionalInformationResponse struct {
	HasCarInSurabaya    bool   `json:"has_car_in_surabaya"`
	HasSponsorRelation  bool   `json:"has_sponsor_relation"`
	HasVendorInSurabaya bool   `json:"has_vendor_in_surabaya"`
	HasCamera           bool   `json:"has_camera"`
	HasLaptop           bool   `json:"has_laptop"`
	HasTablet           bool   `json:"has_tablet"`
	ConnectedCompany    string `json:"connected_company"`
}

type DocumentResponse struct {
	ID           uuid.UUID `json:"id"`
	Type         string    `json:"type"`
	URL          string    `json:"url"`
	OriginalName *string   `json:"original_name,omitempty"`
	MimeType     *string   `json:"mime_type,omitempty"`
	SizeBytes    *int64    `json:"size_bytes,omitempty"`
}

type RegistrantResponse struct {
	ID                      uuid.UUID                      `json:"id"`
	Name                    string                         `json:"name"`
	Email                   string                         `json:"email"`
	NRP                     string                         `json:"nrp"`
	PhoneNumber             string                         `json:"phone_number"`
	LineID                  *string                        `json:"line_id,omitempty"`
	FacultyID               uuid.UUID                      `json:"faculty_id"`
	Faculty                 *MasterReference               `json:"faculty,omitempty"`
	DepartmentID            uuid.UUID                      `json:"department_id"`
	Department              *MasterReference               `json:"department,omitempty"`
	Batch                   int16                          `json:"batch"`
	AboutPetrolida          string                         `json:"about_petrolida"`
	WillingTransferDivision bool                           `json:"willing_transfer_division"`
	Status                  string                         `json:"status"`
	PrimaryRejected         bool                           `json:"primary_rejected"`
	SecondaryRejected       bool                           `json:"secondary_rejected"`
	AcceptedDivision        *MasterReference               `json:"accepted_division,omitempty"`
	CurrentSubdivision      *MasterReference               `json:"current_subdivision,omitempty"`
	Choices                 []ChoiceResponse               `json:"choices"`
	Skills                  *SkillResponse                 `json:"skills,omitempty"`
	AdditionalInfo          *AdditionalInformationResponse `json:"additional_info,omitempty"`
	Documents               []DocumentResponse             `json:"documents,omitempty"`
	CreatedAt               time.Time                      `json:"created_at"`
	UpdatedAt               time.Time                      `json:"updated_at"`
}

type RegistrantListParams struct {
	Page           int
	PageSize       int
	Search         string
	Status         string
	FacultyID      *uuid.UUID
	DepartmentID   *uuid.UUID
	SubdivisionID  *uuid.UUID
	DivisionID     *uuid.UUID
	ChoicePriority int16
	Batch          *int16
	SortBy         string
	SortDirection  string
}

type ActivityLogListParams struct {
	Page       int
	PageSize   int
	ActorID    *uuid.UUID
	DivisionID *uuid.UUID
}

type RegistrantListMeta struct {
	Page       int   `json:"page"`
	PageSize   int   `json:"page_size"`
	TotalItems int64 `json:"total_items"`
	TotalPages int   `json:"total_pages"`
}

type RegistrantCountResponse struct {
	Register           int64 `json:"register"`
	Pending            int64 `json:"pending"`
	ScreeningPrimary   int64 `json:"screening_primary"`
	ScreeningSecondary int64 `json:"screening_secondary"`
	Interviewed        int64 `json:"interviewed"`
	Accepted           int64 `json:"accepted"`
	Rejected           int64 `json:"rejected"`
}

type SubdivisionCountResponse struct {
	SubdivisionID uuid.UUID `json:"subdivision_id"`
	Subdivision   string    `json:"subdivision"`
	Count         int64     `json:"count"`
}

type AdminProfile struct {
	ID       uuid.UUID        `json:"id"`
	Name     string           `json:"name"`
	Email    string           `json:"email"`
	Role     string           `json:"role"`
	Division *MasterReference `json:"division,omitempty"`
}

func (p AdminProfile) IsSuperAdmin() bool {
	return p.Role == "superadmin"
}

func (p AdminProfile) DivisionID() *uuid.UUID {
	if p.Division == nil {
		return nil
	}
	id := p.Division.ID
	return &id
}

type ActivityActorResponse struct {
	ID       uuid.UUID        `json:"id"`
	Name     string           `json:"name"`
	Email    string           `json:"email"`
	Role     string           `json:"role"`
	Division *MasterReference `json:"division,omitempty"`
}

type ActivityRegistrantResponse struct {
	ID   uuid.UUID `json:"id"`
	Name string    `json:"name"`
	NRP  string    `json:"nrp"`
}

type ActivityLogResponse struct {
	ID                          uuid.UUID                  `json:"id"`
	Actor                       ActivityActorResponse      `json:"actor"`
	Registrant                  ActivityRegistrantResponse `json:"registrant"`
	PreviousStatus              string                     `json:"previous_status"`
	NewStatus                   string                     `json:"new_status"`
	RejectionTarget             string                     `json:"rejection_target,omitempty"`
	PrimaryRejected             bool                       `json:"primary_rejected"`
	SecondaryRejected           bool                       `json:"secondary_rejected"`
	PreviousDivision            *MasterReference           `json:"previous_division,omitempty"`
	NewDivision                 *MasterReference           `json:"new_division,omitempty"`
	PreviousAcceptedSubdivision *MasterReference           `json:"previous_accepted_subdivision,omitempty"`
	AcceptedSubdivision         *MasterReference           `json:"accepted_subdivision,omitempty"`
	PreviousCurrentSubdivision  *MasterReference           `json:"previous_current_subdivision,omitempty"`
	CurrentSubdivision          *MasterReference           `json:"current_subdivision,omitempty"`
	CreatedAt                   time.Time                  `json:"created_at"`
}
