package service

import (
	"github.com/Petrolida/Petrolida-2027-Backend/internal/domain"
	"github.com/Petrolida/Petrolida-2027-Backend/internal/entity"
	myerror "github.com/Petrolida/Petrolida-2027-Backend/internal/pkg/error"
	"github.com/Petrolida/Petrolida-2027-Backend/internal/pkg/utils"
	"github.com/google/uuid"
)

// File ini berisi fungsi yang mengubah input pendaftaran yang sudah divalidasi menjadi entity.

func newRegistrant(validated validatedRegistration, key *uuid.UUID, fingerprint *string) entity.OprecStaff {
	input := validated.Input
	return entity.OprecStaff{
		SubmissionKey:           key,
		SubmissionFingerprint:   fingerprint,
		Nama:                    input.Name,
		Email:                   input.Email,
		NRP:                     input.NRP,
		NomorTelepon:            input.PhoneNumber,
		LineID:                  utils.NullableString(input.LineID),
		FakultasID:              validated.FacultyID,
		DepartemenID:            validated.DepartmentID,
		Angkatan:                input.Batch,
		TentangPetrolida:        input.AboutPetrolida,
		WillingTransferDivision: *input.WillingTransferDivision,
		Status:                  entity.OprecStaffStatusPending,
	}
}

func buildChoices(inputs []domain.RegistrationChoiceInput) []entity.OprecStaffChoice {
	choices := make([]entity.OprecStaffChoice, 0, 2)
	for _, choice := range inputs {
		subdivisionID, _ := uuid.Parse(choice.SubdivisionID)
		choices = append(choices, entity.OprecStaffChoice{
			Pilihan:         choice.Priority,
			SubdivisiID:     subdivisionID,
			PemahamanDivisi: choice.AboutDivision,
			AlasanBergabung: choice.ReasonToJoin,
			IdeInovasi:      choice.InnovationIdea,
			Pengalaman:      utils.NullableString(choice.PreviousExperience),
		})
	}
	return choices
}

// buildDocuments selalu berisi link berkas wajib, ditambah portfolio kalau ada.
func buildDocuments(requiredDocumentURL string, portfolio *entity.OprecStaffDocument) []entity.OprecStaffDocument {
	documents := []entity.OprecStaffDocument{{
		Tipe: domain.DocumentTypeRequiredDocuments,
		URL:  requiredDocumentURL,
	}}
	if portfolio != nil {
		documents = append(documents, *portfolio)
	}
	return documents
}

// skillFields memetakan nama field JSON ke pointer field skill, supaya bisa
// di-trim dan divalidasi lewat satu loop.
func skillFields(input *domain.RegistrationSkillInput) map[string]*string {
	return map[string]*string{
		"english_proficiency": &input.EnglishProficiency,
		"communication_skill": &input.CommunicationSkill,
		"creativity_in_event": &input.CreativityInEvent,
		"fast_response":       &input.FastResponse,
		"graphic_design":      &input.GraphicDesign,
		"photography":         &input.Photography,
		"video_editing":       &input.VideoEditing,
		"web_design":          &input.WebDesign,
		"operating_zoom":      &input.OperatingZoom,
		"social_media_trends": &input.SocialMediaTrends,
	}
}

func buildSkillEntity(input domain.RegistrationSkillInput) *entity.OprecStaffSkill {
	return &entity.OprecStaffSkill{
		EnglishProficiency: entity.OprecStaffSkillLevel(input.EnglishProficiency),
		CommunicationSkill: entity.OprecStaffSkillLevel(input.CommunicationSkill),
		CreativityInEvent:  entity.OprecStaffSkillLevel(input.CreativityInEvent),
		FastResponse:       entity.OprecStaffSkillLevel(input.FastResponse),
		GraphicDesign:      entity.OprecStaffSkillLevel(input.GraphicDesign),
		Photography:        entity.OprecStaffSkillLevel(input.Photography),
		VideoEditing:       entity.OprecStaffSkillLevel(input.VideoEditing),
		WebDesign:          entity.OprecStaffSkillLevel(input.WebDesign),
		OperatingZoom:      entity.OprecStaffSkillLevel(input.OperatingZoom),
		SocialMediaTrends:  entity.OprecStaffSkillLevel(input.SocialMediaTrends),
		DesignApplications: input.DesignApplications,
	}
}

func additionalInformationValues(input domain.RegistrationAdditionalInformationInput) map[string]*bool {
	return map[string]*bool{
		"has_car_in_surabaya":    input.HasCarInSurabaya,
		"has_sponsor_relation":   input.HasSponsorRelation,
		"has_vendor_in_surabaya": input.HasVendorInSurabaya,
		"has_camera":             input.HasCamera,
		"has_laptop":             input.HasLaptop,
		"has_tablet":             input.HasTablet,
	}
}

func buildAdditionalInformationEntity(input domain.RegistrationAdditionalInformationInput) (*entity.OprecStaffAdditionalInformation, error) {
	for _, value := range additionalInformationValues(input) {
		if value == nil {
			return nil, myerror.NewFieldValidation("additional_info", "Additional information", "all questions must be answered")
		}
	}
	return &entity.OprecStaffAdditionalInformation{
		HasCarInSurabaya:    *input.HasCarInSurabaya,
		HasSponsorRelation:  *input.HasSponsorRelation,
		HasVendorInSurabaya: *input.HasVendorInSurabaya,
		HasCamera:           *input.HasCamera,
		HasLaptop:           *input.HasLaptop,
		HasTablet:           *input.HasTablet,
		ConnectedCompany:    input.ConnectedCompany,
	}, nil
}
