package service

import (
	"fmt"

	"github.com/Petrolida/Petrolida-2027-Backend/internal/domain"
	"github.com/Petrolida/Petrolida-2027-Backend/internal/entity"
)

// File ini berisi fungsi yang mengubah entity menjadi response API.

func mapActivityLog(log entity.OprecStaffStatusAudit) domain.ActivityLogResponse {
	result := domain.ActivityLogResponse{
		ID:             log.ID,
		Actor:          domain.ActivityActorResponse{ID: log.ActorID, Role: string(log.ActorRole)},
		Registrant:     domain.ActivityRegistrantResponse{ID: log.OprecStaffID},
		PreviousStatus: string(log.PreviousStatus), NewStatus: string(log.NewStatus),
		RejectionTarget: log.RejectionTarget, PrimaryRejected: log.NewPrimaryRejected, SecondaryRejected: log.NewSecondaryRejected,
		CreatedAt: log.Timestamp.CreatedAt,
	}
	if log.Actor != nil {
		result.Actor.Name = log.Actor.Name
		result.Actor.Email = log.Actor.Email
	}
	if log.ActorDivision != nil {
		result.Actor.Division = &domain.MasterReference{ID: log.ActorDivision.ID, Name: log.ActorDivision.Nama}
	}
	if log.OprecStaff != nil {
		result.Registrant.Name = log.OprecStaff.Nama
		result.Registrant.NRP = log.OprecStaff.NRP
	}
	if log.PreviousDivision != nil {
		result.PreviousDivision = &domain.MasterReference{ID: log.PreviousDivision.ID, Name: log.PreviousDivision.Nama}
	}
	if log.NewDivision != nil {
		result.NewDivision = &domain.MasterReference{ID: log.NewDivision.ID, Name: log.NewDivision.Nama}
	}
	if log.PreviousAcceptedSubdivision != nil {
		result.PreviousAcceptedSubdivision = &domain.MasterReference{
			ID: log.PreviousAcceptedSubdivision.ID, Name: log.PreviousAcceptedSubdivision.Nama,
		}
	}
	if log.NewAcceptedSubdivision != nil {
		result.AcceptedSubdivision = &domain.MasterReference{
			ID: log.NewAcceptedSubdivision.ID, Name: log.NewAcceptedSubdivision.Nama,
		}
	}
	if log.PreviousCurrentSubdivision != nil {
		result.PreviousCurrentSubdivision = &domain.MasterReference{
			ID: log.PreviousCurrentSubdivision.ID, Name: log.PreviousCurrentSubdivision.Nama,
		}
	}
	if log.NewCurrentSubdivision != nil {
		result.CurrentSubdivision = &domain.MasterReference{
			ID: log.NewCurrentSubdivision.ID, Name: log.NewCurrentSubdivision.Nama,
		}
	}
	return result
}

func mapRegistrant(registrant entity.OprecStaff, includeDocuments bool) domain.RegistrantResponse {
	response := domain.RegistrantResponse{
		ID: registrant.ID, Name: registrant.Nama, Email: registrant.Email, NRP: registrant.NRP,
		PhoneNumber: registrant.NomorTelepon, LineID: registrant.LineID,
		FacultyID: registrant.FakultasID, DepartmentID: registrant.DepartemenID,
		Batch: registrant.Angkatan, AboutPetrolida: registrant.TentangPetrolida,
		WillingTransferDivision: registrant.WillingTransferDivision,
		Status:                  string(registrant.Status), CreatedAt: registrant.Timestamp.CreatedAt, UpdatedAt: registrant.Timestamp.UpdatedAt,
		PrimaryRejected: registrant.PrimaryRejected, SecondaryRejected: registrant.SecondaryRejected,
		Choices: make([]domain.ChoiceResponse, 0, len(registrant.Choices)),
	}
	if registrant.Fakultas != nil {
		response.Faculty = &domain.MasterReference{ID: registrant.Fakultas.ID, Name: registrant.Fakultas.Nama}
	}
	if registrant.Departemen != nil {
		response.Department = &domain.MasterReference{ID: registrant.Departemen.ID, Name: registrant.Departemen.Nama}
	}
	if registrant.AcceptedDivisi != nil {
		response.AcceptedDivision = &domain.MasterReference{ID: registrant.AcceptedDivisi.ID, Name: registrant.AcceptedDivisi.Nama}
	}
	if registrant.CurrentSubdivision != nil {
		response.CurrentSubdivision = &domain.MasterReference{ID: registrant.CurrentSubdivision.ID, Name: registrant.CurrentSubdivision.Nama}
	}
	if registrant.Skills != nil {
		response.Skills = &domain.SkillResponse{
			EnglishProficiency: string(registrant.Skills.EnglishProficiency),
			CommunicationSkill: string(registrant.Skills.CommunicationSkill),
			CreativityInEvent:  string(registrant.Skills.CreativityInEvent),
			FastResponse:       string(registrant.Skills.FastResponse),
			GraphicDesign:      string(registrant.Skills.GraphicDesign),
			Photography:        string(registrant.Skills.Photography),
			VideoEditing:       string(registrant.Skills.VideoEditing),
			WebDesign:          string(registrant.Skills.WebDesign),
			OperatingZoom:      string(registrant.Skills.OperatingZoom),
			SocialMediaTrends:  string(registrant.Skills.SocialMediaTrends),
			DesignApplications: registrant.Skills.DesignApplications,
		}
	}
	if registrant.AdditionalInfo != nil {
		response.AdditionalInfo = &domain.AdditionalInformationResponse{
			HasCarInSurabaya:    registrant.AdditionalInfo.HasCarInSurabaya,
			HasSponsorRelation:  registrant.AdditionalInfo.HasSponsorRelation,
			HasVendorInSurabaya: registrant.AdditionalInfo.HasVendorInSurabaya,
			HasCamera:           registrant.AdditionalInfo.HasCamera,
			HasLaptop:           registrant.AdditionalInfo.HasLaptop,
			HasTablet:           registrant.AdditionalInfo.HasTablet,
			ConnectedCompany:    registrant.AdditionalInfo.ConnectedCompany,
		}
	}
	for _, choice := range registrant.Choices {
		item := domain.ChoiceResponse{
			ID: choice.ID, Priority: choice.Pilihan, SubdivisionID: choice.SubdivisiID,
			AboutDivision: choice.PemahamanDivisi, ReasonToJoin: choice.AlasanBergabung,
			InnovationIdea: choice.IdeInovasi, PreviousExperience: choice.Pengalaman,
		}
		if choice.Subdivisi != nil {
			item.Subdivision = &domain.MasterReference{ID: choice.Subdivisi.ID, Name: choice.Subdivisi.Nama}
			if choice.Subdivisi.Divisi != nil {
				item.Division = &domain.MasterReference{ID: choice.Subdivisi.Divisi.ID, Name: choice.Subdivisi.Divisi.Nama}
			}
		}
		response.Choices = append(response.Choices, item)
	}
	if includeDocuments {
		response.Documents = make([]domain.DocumentResponse, 0, len(registrant.Documents))
		for _, document := range registrant.Documents {
			url := document.URL
			if document.Tipe == domain.DocumentTypePortfolio {
				url = fmt.Sprintf("/oprec-staff/%s/documents/%s", registrant.ID, document.ID)
			}
			response.Documents = append(response.Documents, domain.DocumentResponse{
				ID: document.ID, Type: document.Tipe, URL: url, OriginalName: document.OriginalName,
				MimeType: document.MimeType, SizeBytes: document.SizeBytes,
			})
		}
	}
	return response
}
