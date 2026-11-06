package service

import (
	"bytes"
	"errors"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/Petrolida/Petrolida-2027-Backend/internal/api/repository"
	"github.com/Petrolida/Petrolida-2027-Backend/internal/domain"
	"github.com/Petrolida/Petrolida-2027-Backend/internal/entity"
	myerror "github.com/Petrolida/Petrolida-2027-Backend/internal/pkg/error"
	"github.com/Petrolida/Petrolida-2027-Backend/internal/pkg/export"
	"github.com/google/uuid"
	"github.com/xuri/excelize/v2"
)

// exportRepository hanya mengimplementasikan ListForExport. Method lain akan
// panic kalau terpanggil, dan memang tidak boleh terpanggil oleh Export.
type exportRepository struct {
	repository.OprecStaffRepository
	registrants []entity.OprecStaff
	gotParams   domain.RegistrantListParams
	gotDivision *uuid.UUID
}

func (r *exportRepository) ListForExport(params domain.RegistrantListParams, divisionID *uuid.UUID) ([]entity.OprecStaff, error) {
	r.gotParams, r.gotDivision = params, divisionID
	return r.registrants, nil
}

func exportColumnIndex(t *testing.T, header string) int {
	t.Helper()
	for i, column := range registrantExportColumns {
		if column.Header == header {
			return i
		}
	}
	t.Fatalf("export column %q not found", header)
	return -1
}

func TestExportBuildsRegistrantRow(t *testing.T) {
	registrantID, portfolioID := uuid.New(), uuid.New()
	experience := "Panitia PKKMB"
	repo := &exportRepository{registrants: []entity.OprecStaff{{
		ID: registrantID, Nama: "Budi", NRP: "5025251178", Email: "budi@example.com", NomorTelepon: "081234567890",
		Angkatan: 2025, TentangPetrolida: "Kompetisi", WillingTransferDivision: true, Status: entity.OprecStaffStatusAccepted,
		Timestamp:  entity.Timestamp{CreatedAt: time.Date(2026, 9, 30, 17, 30, 0, 0, time.UTC)},
		Fakultas:   &entity.Fakultas{Nama: "FTEIC"},
		Departemen: &entity.Departemen{Nama: "Teknik Informatika"},
		// Pilihan 2 sengaja ditaruh lebih dulu untuk memastikan urutan kolom
		// mengikuti nomor pilihan, bukan urutan slice.
		Choices: []entity.OprecStaffChoice{
			{Pilihan: 2, Subdivisi: &entity.Subdivisi{Nama: "Sponsorship", Divisi: &entity.Divisi{Nama: "External"}}, AlasanBergabung: "alasan 2"},
			{Pilihan: 1, Subdivisi: &entity.Subdivisi{Nama: "Web Development", Divisi: &entity.Divisi{Nama: "IT"}}, AlasanBergabung: "alasan 1", Pengalaman: &experience},
		},
		Documents: []entity.OprecStaffDocument{
			{Tipe: domain.DocumentTypeRequiredDocuments, URL: "https://drive.google.com/drive/folders/abc"},
			{ID: portfolioID, Tipe: domain.DocumentTypePortfolio, URL: "gdrive-secret-file-id"},
		},
		AcceptedDivisi: &entity.Subdivisi{Nama: "Web Development"},
	}}}
	service := &oprecStaffService{repository: repo, publicURL: "https://petrolida.com"}

	table, err := service.Export(domain.AdminProfile{Role: string(entity.RoleSuperAdmin)}, domain.RegistrantListParams{Status: "accepted"})
	if err != nil {
		t.Fatalf("Export() error = %v", err)
	}
	if repo.gotDivision != nil || repo.gotParams.Status != "accepted" {
		t.Fatalf("repository received division %v and params %+v", repo.gotDivision, repo.gotParams)
	}
	if len(table.Rows) != 1 || len(table.Rows[0]) != len(registrantExportColumns) {
		t.Fatalf("rows = %v, want one row with %d cells", table.Rows, len(registrantExportColumns))
	}
	row := table.Rows[0]
	want := map[string]string{
		"Waktu Daftar (WIB)":         "2026-10-01 00:30:00",
		"NRP":                        "5025251178",
		"Fakultas":                   "FTEIC",
		"Angkatan":                   "2025",
		"Bersedia Pindah Divisi":     "Ya",
		"Pilihan 1 Divisi":           "IT",
		"Pilihan 1 Subdivisi":        "Web Development",
		"Pilihan 1 Alasan Bergabung": "alasan 1",
		"Pilihan 1 Pengalaman":       "Panitia PKKMB",
		"Pilihan 2 Subdivisi":        "Sponsorship",
		"Pilihan 2 Pengalaman":       "",
		"Link Berkas Wajib":          "https://drive.google.com/drive/folders/abc",
		"Link Portfolio":             "https://petrolida.com/api/v1/oprec-staff/" + registrantID.String() + "/documents/" + portfolioID.String(),
		"Status":                     "accepted",
		"Divisi Penerimaan":          "Web Development",
	}
	for header, value := range want {
		if got := row[exportColumnIndex(t, header)]; got != value {
			t.Errorf("%s = %q, want %q", header, got, value)
		}
	}
	if slices.ContainsFunc(row, func(cell string) bool { return strings.Contains(cell, "gdrive-secret-file-id") }) {
		t.Fatal("export leaked portfolio storage key")
	}
}

func TestExportLeavesMissingChoiceEmpty(t *testing.T) {
	repo := &exportRepository{registrants: []entity.OprecStaff{{
		Choices: []entity.OprecStaffChoice{{Pilihan: 1, Subdivisi: &entity.Subdivisi{Nama: "Web Development"}}},
	}}}
	table, err := (&oprecStaffService{repository: repo}).Export(domain.AdminProfile{Role: string(entity.RoleSuperAdmin)}, domain.RegistrantListParams{})
	if err != nil {
		t.Fatalf("Export() error = %v", err)
	}
	start := exportColumnIndex(t, "Pilihan 2 Divisi")
	for i, cell := range table.Rows[0][start : start+6] {
		if cell != "" {
			t.Errorf("choice 2 cell %d = %q, want empty", i, cell)
		}
	}
}

func TestExportEmptyDatasetKeepsColumns(t *testing.T) {
	table, err := (&oprecStaffService{repository: &exportRepository{}}).Export(domain.AdminProfile{Role: string(entity.RoleSuperAdmin)}, domain.RegistrantListParams{})
	if err != nil {
		t.Fatalf("Export() error = %v", err)
	}
	if len(table.Rows) != 0 || len(table.Columns) != len(registrantExportColumns) {
		t.Fatalf("table = %+v, want columns without rows", table)
	}
}

func TestExportScopesDivisionAdmin(t *testing.T) {
	divisionID := uuid.New()
	repo := &exportRepository{}
	admin := domain.AdminProfile{Role: string(entity.RoleDivisionAdmin), Division: &domain.MasterReference{ID: divisionID}}
	if _, err := (&oprecStaffService{repository: repo}).Export(admin, domain.RegistrantListParams{}); err != nil {
		t.Fatalf("Export() error = %v", err)
	}
	if repo.gotDivision == nil || *repo.gotDivision != divisionID {
		t.Fatalf("division scope = %v, want %s", repo.gotDivision, divisionID)
	}
}

func TestExportRejectsUnknownRole(t *testing.T) {
	_, err := (&oprecStaffService{repository: &exportRepository{}}).Export(domain.AdminProfile{Role: "participant"}, domain.RegistrantListParams{})
	var typed myerror.Error
	if !errors.As(err, &typed) || typed.Code != domain.ErrForbidden.Code {
		t.Fatalf("Export() error = %v, want ErrForbidden", err)
	}
}

func TestExportGroupsSubdivisionsIntoParentDivisionSheets(t *testing.T) {
	a, b := uuid.New(), uuid.New()
	a1, a2, b1 := uuid.New(), uuid.New(), uuid.New()
	choice := func(priority int16, id, parent uuid.UUID, name string) entity.OprecStaffChoice {
		return entity.OprecStaffChoice{Pilihan: priority, SubdivisiID: id, Subdivisi: &entity.Subdivisi{ID: id, DivisiID: parent, Divisi: &entity.Divisi{ID: parent, Nama: name}}}
	}
	repo := &exportRepository{registrants: []entity.OprecStaff{
		{Nama: "Same parent", Choices: []entity.OprecStaffChoice{choice(1, a1, a, "Creative"), choice(2, a2, a, "Creative")}},
		{Nama: "Different parents", Choices: []entity.OprecStaffChoice{choice(1, a1, a, "Creative"), choice(2, b1, b, "Finance")}},
	}}
	svc := &oprecStaffService{repository: repo}
	admin := domain.AdminProfile{Role: string(entity.RoleSuperAdmin)}
	table, err := svc.Export(admin, domain.RegistrantListParams{})
	if err != nil {
		t.Fatal(err)
	}
	groups := table.Review.Divisions
	if len(table.Rows) != 2 || len(groups) != 2 || groups[0].Name != "Creative" || len(groups[0].Data.Rows) != 2 || groups[1].Name != "Finance" || len(groups[1].Data.Rows) != 1 {
		t.Fatalf("wrong grouping: %+v", groups)
	}
	if groups[0].Data.Rows[0][3] != "Primary & Secondary" || groups[1].Data.Rows[0][3] != "Secondary" {
		t.Fatal("division membership does not distinguish original choice priorities")
	}
	for _, group := range groups {
		if len(group.Answers.Rows) != len(group.Data.Rows) || len(group.Scoring.Rows) != len(group.Data.Rows) {
			t.Fatalf("%s repeats applicants across question rows", group.Name)
		}
	}
	table, err = svc.Export(admin, domain.RegistrantListParams{DivisionID: &a, ChoicePriority: 1})
	if err != nil || len(table.Review.Divisions) != 1 || len(table.Review.Divisions[0].Data.Rows) != 2 {
		t.Fatalf("filter grouping: %+v %v", table.Review, err)
	}
	table, err = svc.Export(domain.AdminProfile{Role: string(entity.RoleDivisionAdmin), Division: &domain.MasterReference{ID: a, Name: "Creative"}}, domain.RegistrantListParams{})
	if err != nil || len(table.Review.Divisions) != 1 || table.Review.Divisions[0].Name != "Creative" {
		t.Fatalf("scope grouping: %+v %v", table.Review, err)
	}
}

func TestReviewWorkbookIncludesBothChoicesAndCompleteAnswers(t *testing.T) {
	creative, finance := uuid.New(), uuid.New()
	registrantID, portfolioID := uuid.New(), uuid.New()
	folder := "https://drive.google.com/drive/folders/required-documents"
	portfolio := "https://petrolida.com/api/v1/oprec-staff/" + registrantID.String() + "/documents/" + portfolioID.String()
	line, experience := "candidate_line", "Panitia acara"
	repo := &exportRepository{registrants: []entity.OprecStaff{{
		ID: registrantID, Nama: "Budi", NRP: "000123456789012345", NomorTelepon: "081234567890", LineID: &line,
		TentangPetrolida: "Kompetisi mahasiswa", WillingTransferDivision: true,
		Choices: []entity.OprecStaffChoice{
			{Pilihan: 2, Subdivisi: &entity.Subdivisi{Nama: "Sponsorship", Divisi: &entity.Divisi{ID: finance, Nama: "Finance"}}, PemahamanDivisi: "Relasi sponsor", AlasanBergabung: "Alasan secondary", IdeInovasi: "Ide secondary"},
			{Pilihan: 1, Subdivisi: &entity.Subdivisi{Nama: "Design", Divisi: &entity.Divisi{ID: creative, Nama: "Creative"}}, PemahamanDivisi: "Desain visual", AlasanBergabung: "Alasan primary", IdeInovasi: "Ide primary", Pengalaman: &experience},
		},
		Skills:         &entity.OprecStaffSkill{EnglishProficiency: entity.OprecStaffSkillLevelGood, GraphicDesign: entity.OprecStaffSkillLevelVeryGood, DesignApplications: "Figma, Photoshop"},
		AdditionalInfo: &entity.OprecStaffAdditionalInformation{HasSponsorRelation: true, HasLaptop: true, ConnectedCompany: "Perusahaan Contoh"},
		Documents: []entity.OprecStaffDocument{
			{Tipe: domain.DocumentTypeRequiredDocuments, URL: folder},
			{ID: portfolioID, Tipe: domain.DocumentTypePortfolio, URL: "private-storage-key"},
		},
	}}}
	table, err := (&oprecStaffService{repository: repo, publicURL: "https://petrolida.com"}).Export(domain.AdminProfile{Role: string(entity.RoleSuperAdmin)}, domain.RegistrantListParams{DivisionID: &creative, ChoicePriority: 1})
	if err != nil {
		t.Fatal(err)
	}
	var buffer bytes.Buffer
	if err := export.WriteXLSX(&buffer, table); err != nil {
		t.Fatal(err)
	}
	file, err := excelize.OpenReader(&buffer)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	if names := file.GetSheetList(); !slices.Equal(names, []string{"Navigasi", "Data - Creative", "Jawaban - Creative", "Scoring - Creative"}) {
		t.Fatalf("unexpected tabs: %v", names)
	}
	for _, division := range []string{"Creative"} {
		for cell, want := range map[string]string{"C2": "000123456789012345", "E2": "Creative", "F2": "Design", "G2": "Finance", "H2": "Sponsorship", "K2": "081234567890", "L2": "candidate_line"} {
			if got, err := file.GetCellValue("Data - "+division, cell); err != nil || got != want {
				t.Errorf("%s %s = %q, %v; want %q", division, cell, got, err, want)
			}
		}
		rows, err := file.GetRows("Jawaban - " + division)
		if err != nil || len(rows) != 2 {
			t.Fatalf("answers must contain one header and one applicant: %v %v", rows, err)
		}
		answers := map[string]string{}
		for i, header := range rows[0] {
			if i < len(rows[1]) {
				answers[header] = rows[1][i]
			}
		}
		for question, want := range map[string]string{
			"Tentang Petrolida": "Kompetisi mahasiswa", "Bersedia Pindah Divisi": "Ya",
			"Primary Divisi": "Creative", "Primary: Pemahaman Divisi": "Desain visual", "Primary: Alasan Bergabung": "Alasan primary", "Primary: Ide Inovasi": "Ide primary", "Primary: Pengalaman": experience,
			"Secondary Divisi": "Finance", "Secondary: Pemahaman Divisi": "Relasi sponsor", "Secondary: Alasan Bergabung": "Alasan secondary", "Secondary: Ide Inovasi": "Ide secondary",
			"Kemampuan Bahasa Inggris": "good", "Desain Grafis": "very_good", "Aplikasi Desain": "Figma, Photoshop",
			"Relasi Sponsor": "Ya", "Memiliki Laptop": "Ya", "Memiliki Kamera": "Tidak", "Perusahaan yang Terhubung": "Perusahaan Contoh",
		} {
			if got := answers[question]; got != want {
				t.Errorf("%s answer %s = %q, want %q", division, question, got, want)
			}
		}
		columns := table.Review.Divisions[0].Scoring.Columns
		for i, column := range columns {
			if !column.Input {
				continue
			}
			cell, _ := excelize.CoordinatesToCellName(i+1, 2)
			value, _ := file.GetCellValue("Scoring - "+division, cell)
			formula, _ := file.GetCellFormula("Scoring - "+division, cell)
			if value != "" || formula != "" {
				t.Errorf("scoring input %s is prefilled: %q %q", cell, value, formula)
			}
		}
		for _, question := range reviewQuestions {
			if question.scoreGroup != "" {
				if slices.ContainsFunc(columns, func(column export.Column) bool { return column.Header == "Skor "+question.header }) {
					t.Errorf("grouped response %s still has an individual score", question.header)
				}
				continue
			}
			if !slices.ContainsFunc(columns, func(column export.Column) bool { return column.Header == "Skor "+question.header && column.Input }) {
				t.Errorf("missing assessment for %s", question.header)
			}
		}
		for _, group := range []string{"Skills Level", "Additional Information"} {
			count := 0
			for _, column := range columns {
				if column.Header == "Skor "+group && column.Input && column.Score {
					count++
				}
			}
			if count != 1 {
				t.Errorf("%s score columns = %d; want one", group, count)
			}
		}
		totalScores := 0
		totalCell := ""
		var scoreCells []string
		for i, column := range columns {
			if column.Score && !column.Input {
				t.Errorf("assessment %s is not editable", column.Header)
			}
			if column.Score {
				cell, _ := excelize.CoordinatesToCellName(i+1, 2)
				scoreCells = append(scoreCells, cell)
			}
			if column.SumScores {
				totalScores++
				if column.Header != "Akumulasi Skor" || column.Input || i+1 >= len(columns) || columns[i+1].Header != "Catatan" {
					t.Errorf("automatic total must appear immediately before Catatan: %+v", column)
				}
				cell, _ := excelize.CoordinatesToCellName(i+1, 2)
				totalCell = cell
				formula, err := file.GetCellFormula("Scoring - "+division, cell)
				if err != nil || formula == "" {
					t.Errorf("missing automatic total at %s: %q %v", cell, formula, err)
				}
			}
		}
		if totalScores != 1 {
			t.Errorf("total columns = %d; want one", totalScores)
		}
		// Ten general/choice scores, two grouped scores, separate design/company
		// scores, and eight document scores must all contribute exactly once.
		if len(scoreCells) != 22 {
			t.Fatalf("score columns = %d; want 22", len(scoreCells))
		}
		for _, cell := range scoreCells {
			if err := file.SetCellInt("Scoring - "+division, cell, 1); err != nil {
				t.Fatal(err)
			}
		}
		if totalCell == "" {
			t.Fatal("missing score total")
		}
		value, err := file.CalcCellValue("Scoring - "+division, totalCell)
		if err != nil || value != "22" {
			t.Errorf("combined score = %q %v; want 22", value, err)
		}
		for _, document := range []string{"Portfolio", "Kelengkapan Berkas", "KTM", "CV", "Twibbon", "Bukti Follow LinkedIn", "Bukti Follow Instagram", "Bukti Follow TikTok"} {
			if !slices.ContainsFunc(columns, func(column export.Column) bool { return column.Header == "Skor "+document && column.Input }) {
				t.Errorf("missing document assessment for %s", document)
			}
		}
		for sheet, cells := range map[string]map[string]string{
			"Data - " + division:    {"V2": folder, "W2": portfolio},
			"Scoring - " + division: {"I2": folder, "J2": portfolio},
		} {
			for cell, want := range cells {
				exists, target, err := file.GetCellHyperLink(sheet, cell)
				if err != nil || !exists || target != want {
					t.Errorf("%s %s document target = %q %v; want %q", sheet, cell, target, err, want)
				}
			}
		}
	}
	if table.Rows[0][exportColumnIndex(t, "Aplikasi Desain")] != "Figma, Photoshop" || table.Rows[0][exportColumnIndex(t, "Memiliki Laptop")] != "Ya" {
		t.Fatal("CSV lost additional answers")
	}
}

func TestReviewMissingResponsesStayBlankAndEmptyDivisionKeepsTemplate(t *testing.T) {
	cells := additionalExportCells(entity.OprecStaff{})
	for _, cell := range cells {
		if cell != "" {
			t.Fatal("missing response was invented", cells)
		}
	}
	id := uuid.New()
	table, err := (&oprecStaffService{repository: &exportRepository{}}).Export(domain.AdminProfile{Role: string(entity.RoleDivisionAdmin), Division: &domain.MasterReference{ID: id, Name: "Creative"}}, domain.RegistrantListParams{})
	if err != nil || len(table.Review.Divisions) != 1 || len(table.Review.Divisions[0].Scoring.Columns) == 0 || len(table.Review.Divisions[0].Scoring.Rows) != 0 {
		t.Fatalf("empty division lost its template: %+v %v", table.Review, err)
	}
}
