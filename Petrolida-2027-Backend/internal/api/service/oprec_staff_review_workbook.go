package service

import (
	"strconv"
	"strings"

	"github.com/Petrolida/Petrolida-2027-Backend/internal/entity"
	"github.com/Petrolida/Petrolida-2027-Backend/internal/pkg/export"
	"github.com/google/uuid"
)

var reviewDataColumns = []export.Column{
	{Header: "No."}, {Header: "Nama"}, {Header: "NRP", Text: true},
	{Header: "Pilihan di Divisi Ini"},
	{Header: "Primary Divisi"}, {Header: "Primary Subdivisi"},
	{Header: "Secondary Divisi"}, {Header: "Secondary Subdivisi"},
	{Header: "Status"}, {Header: "Email"}, {Header: "No. Telepon", Text: true},
	{Header: "ID LINE"}, {Header: "Fakultas"}, {Header: "Departemen"},
	{Header: "Angkatan"}, {Header: "Waktu Daftar (WIB)"},
	{Header: "Bersedia Pindah Divisi"}, {Header: "Subdivisi Interview"},
	{Header: "Divisi Penerimaan"}, {Header: "Primary Ditolak"},
	{Header: "Secondary Ditolak"}, {Header: "Link Berkas Wajib", Link: true}, {Header: "Link Portfolio", Link: true},
}

var reviewIdentityColumns = []export.Column{
	{Header: "No."}, {Header: "Nama"}, {Header: "NRP", Text: true},
	{Header: "Pilihan di Divisi Ini"},
	{Header: "Primary Divisi"}, {Header: "Primary Subdivisi"},
	{Header: "Secondary Divisi"}, {Header: "Secondary Subdivisi"},
}

type reviewQuestion struct {
	header     string
	index      int
	scoreGroup string
}

// Identity and choice placement are context, rather than assessment questions.
var reviewQuestions = func() []reviewQuestion {
	questions := []reviewQuestion{
		{header: "Tentang Petrolida", index: 9}, {header: "Bersedia Pindah Divisi", index: 10},
		{header: "Primary: Pemahaman Divisi", index: 13}, {header: "Primary: Alasan Bergabung", index: 14},
		{header: "Primary: Ide Inovasi", index: 15}, {header: "Primary: Pengalaman", index: 16},
		{header: "Secondary: Pemahaman Divisi", index: 19}, {header: "Secondary: Alasan Bergabung", index: 20},
		{header: "Secondary: Ide Inovasi", index: 21}, {header: "Secondary: Pengalaman", index: 22},
	}
	for i, column := range additionalExportColumns {
		group := ""
		switch {
		case i < 10:
			group = "Skills Level"
		case i >= 11 && i < 17:
			group = "Additional Information"
		}
		questions = append(questions, reviewQuestion{header: column.Header, index: 30 + i, scoreGroup: group})
	}
	return questions
}()

var reviewDocumentScores = []string{
	"Portfolio", "Kelengkapan Berkas", "KTM", "CV", "Twibbon",
	"Bukti Follow LinkedIn", "Bukti Follow Instagram", "Bukti Follow TikTok",
}

func reviewResponseColumns(scoring bool) []export.Column {
	columns := append([]export.Column(nil), reviewIdentityColumns...)
	if scoring {
		columns = append(columns, export.Column{Header: "Link Berkas Wajib", Link: true}, export.Column{Header: "Link Portfolio", Link: true}, export.Column{Header: "Penilai", Input: true})
	}
	seenGroups := map[string]bool{}
	for _, question := range reviewQuestions {
		header := question.header
		if scoring {
			if question.scoreGroup != "" {
				if seenGroups[question.scoreGroup] {
					continue
				}
				seenGroups[question.scoreGroup] = true
				header = question.scoreGroup
			}
			header = "Skor " + header
		}
		columns = append(columns, export.Column{Header: header, Input: scoring, Score: scoring})
	}
	if scoring {
		for _, document := range reviewDocumentScores {
			columns = append(columns, export.Column{Header: "Skor " + document, Input: true, Score: true})
		}
		columns = append(columns, export.Column{Header: "Akumulasi Skor", SumScores: true}, export.Column{Header: "Catatan", Input: true})
	}
	return columns
}

func (s *oprecStaffService) divisionWorkbook(id uuid.UUID, group exportDivisionGroup) export.DivisionWorkbook {
	name := group.name
	if strings.TrimSpace(name) == "" {
		name = "Tanpa Divisi"
	}
	book := export.DivisionWorkbook{
		Name:    name,
		Data:    export.Sheet{Columns: reviewDataColumns},
		Answers: export.Sheet{Columns: reviewResponseColumns(false)},
		Scoring: export.Sheet{Columns: reviewResponseColumns(true)},
	}
	for i, registrant := range group.registrants {
		flat := s.registrantExportRow(registrant)
		number := strconv.Itoa(i + 1)
		membership := divisionChoiceLabel(registrant, id)
		data := []string{number, flat[1], flat[2], membership}
		for _, index := range []int{11, 12, 17, 18, 25, 3, 4, 5, 6, 7, 8, 0, 10, 26, 27, 28, 29, 23, 24} {
			data = append(data, flat[index])
		}
		book.Data.Rows = append(book.Data.Rows, data)
		identity := []string{number, registrant.Nama, registrant.NRP, membership, flat[11], flat[12], flat[17], flat[18]}
		answers := append([]string(nil), identity...)
		for _, question := range reviewQuestions {
			answers = append(answers, flat[question.index])
		}
		book.Answers.Rows = append(book.Answers.Rows, answers)
		// The folder contains KTM/CV/Twibbon/follow proofs; individual URLs are
		// not stored. Keep the folder and authenticated portfolio link next to
		// each applicant and leave every question/document assessment blank.
		scores := make([]string, len(book.Scoring.Columns))
		copy(scores, identity)
		scores[8], scores[9] = flat[23], flat[24]
		book.Scoring.Rows = append(book.Scoring.Rows, scores)
	}
	return book
}

func divisionChoiceLabel(registrant entity.OprecStaff, divisionID uuid.UUID) string {
	var primary, secondary bool
	for _, choice := range registrant.Choices {
		if choice.Subdivisi == nil {
			continue
		}
		parentID := choice.Subdivisi.DivisiID
		if choice.Subdivisi.Divisi != nil {
			parentID = choice.Subdivisi.Divisi.ID
		}
		if parentID != divisionID {
			continue
		}
		primary = primary || choice.Pilihan == 1
		secondary = secondary || choice.Pilihan == 2
	}
	switch {
	case primary && secondary:
		return "Primary & Secondary"
	case primary:
		return "Primary"
	case secondary:
		return "Secondary"
	default:
		return "Penempatan / tanpa pilihan"
	}
}

func additionalExportCells(registrant entity.OprecStaff) []string {
	cells := make([]string, len(additionalExportColumns))
	if skill := registrant.Skills; skill != nil {
		for i, level := range []entity.OprecStaffSkillLevel{
			skill.EnglishProficiency, skill.CommunicationSkill, skill.CreativityInEvent,
			skill.FastResponse, skill.GraphicDesign, skill.Photography, skill.VideoEditing,
			skill.WebDesign, skill.OperatingZoom, skill.SocialMediaTrends,
		} {
			cells[i] = string(level)
		}
		cells[10] = skill.DesignApplications
	}
	if info := registrant.AdditionalInfo; info != nil {
		for i, value := range []bool{info.HasCarInSurabaya, info.HasSponsorRelation, info.HasVendorInSurabaya, info.HasCamera, info.HasLaptop, info.HasTablet} {
			cells[11+i] = "Tidak"
			if value {
				cells[11+i] = "Ya"
			}
		}
		cells[17] = info.ConnectedCompany
	}
	return cells
}
