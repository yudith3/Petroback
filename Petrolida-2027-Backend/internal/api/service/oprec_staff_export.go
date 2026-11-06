package service

import (
	"fmt"
	"sort"
	"strconv"

	"github.com/Petrolida/Petrolida-2027-Backend/internal/domain"
	"github.com/Petrolida/Petrolida-2027-Backend/internal/entity"
	"github.com/Petrolida/Petrolida-2027-Backend/internal/pkg/export"
	"github.com/google/uuid"
)

var registrantExportColumns = []export.Column{
	{Header: "Waktu Daftar (WIB)"},
	{Header: "Nama"},
	{Header: "NRP", Text: true},
	{Header: "Email"},
	{Header: "No. Telepon", Text: true},
	{Header: "ID LINE"},
	{Header: "Fakultas"},
	{Header: "Departemen"},
	{Header: "Angkatan"},
	{Header: "Tentang Petrolida"},
	{Header: "Bersedia Pindah Divisi"},
	{Header: "Pilihan 1 Divisi"},
	{Header: "Pilihan 1 Subdivisi"},
	{Header: "Pilihan 1 Pemahaman Divisi"},
	{Header: "Pilihan 1 Alasan Bergabung"},
	{Header: "Pilihan 1 Ide Inovasi"},
	{Header: "Pilihan 1 Pengalaman"},
	{Header: "Pilihan 2 Divisi"},
	{Header: "Pilihan 2 Subdivisi"},
	{Header: "Pilihan 2 Pemahaman Divisi"},
	{Header: "Pilihan 2 Alasan Bergabung"},
	{Header: "Pilihan 2 Ide Inovasi"},
	{Header: "Pilihan 2 Pengalaman"},
	{Header: "Link Berkas Wajib"},
	{Header: "Link Portfolio"},
	{Header: "Status"},
	{Header: "Subdivisi Interview"},
	{Header: "Divisi Penerimaan"},
	{Header: "Primary Ditolak"},
	{Header: "Secondary Ditolak"},
}

var additionalExportColumns = []export.Column{
	{Header: "Kemampuan Bahasa Inggris"}, {Header: "Kemampuan Komunikasi"},
	{Header: "Kreativitas Acara"}, {Header: "Respons Cepat"},
	{Header: "Desain Grafis"}, {Header: "Fotografi"}, {Header: "Video Editing"},
	{Header: "Web Design"}, {Header: "Mengoperasikan Zoom"}, {Header: "Tren Media Sosial"},
	{Header: "Aplikasi Desain"}, {Header: "Mobil di Surabaya"},
	{Header: "Relasi Sponsor"}, {Header: "Vendor di Surabaya"},
	{Header: "Memiliki Kamera"}, {Header: "Memiliki Laptop"}, {Header: "Memiliki Tablet"},
	{Header: "Perusahaan yang Terhubung"},
}

func init() {
	registrantExportColumns = append(registrantExportColumns, additionalExportColumns...)
}

type exportDivisionGroup struct {
	name        string
	registrants []entity.OprecStaff
}

func (s *oprecStaffService) Export(admin domain.AdminProfile, params domain.RegistrantListParams) (export.Table, error) {
	divisionID, err := adminScope(admin)
	if err != nil {
		return export.Table{}, err
	}
	registrants, err := s.repository.ListForExport(params, divisionID)
	if err != nil {
		return export.Table{}, err
	}
	table := export.Table{
		Columns: registrantExportColumns, Rows: make([][]string, 0, len(registrants)),
		Review: &export.ReviewWorkbook{RegistrantCount: len(registrants)},
	}
	sheets := map[uuid.UUID]*exportDivisionGroup{}
	for _, registrant := range registrants {
		row := s.registrantExportRow(registrant)
		table.Rows = append(table.Rows, row)
		seen := map[uuid.UUID]bool{}
		for _, choice := range registrant.Choices {
			if choice.Subdivisi == nil || choice.Subdivisi.Divisi == nil {
				continue
			}
			division := choice.Subdivisi.Divisi
			if divisionID != nil && division.ID != *divisionID {
				continue
			}
			if params.DivisionID != nil && division.ID != *params.DivisionID {
				continue
			}
			if params.SubdivisionID != nil && choice.SubdivisiID != *params.SubdivisionID {
				continue
			}
			if params.ChoicePriority != 0 && choice.Pilihan != params.ChoicePriority {
				continue
			}
			if seen[division.ID] {
				continue
			}
			seen[division.ID] = true
			if sheets[division.ID] == nil {
				sheets[division.ID] = &exportDivisionGroup{name: division.Nama}
			}
			sheets[division.ID].registrants = append(sheets[division.ID].registrants, registrant)
		}
		if len(seen) == 0 {
			id, name := uuid.Nil, "Tanpa Divisi"
			if divisionID != nil {
				id, name = *divisionID, admin.Division.Name
			}
			if sheets[id] == nil {
				sheets[id] = &exportDivisionGroup{name: name}
			}
			sheets[id].registrants = append(sheets[id].registrants, registrant)
		}
	}
	if len(registrants) == 0 && divisionID != nil {
		sheets[*divisionID] = &exportDivisionGroup{name: admin.Division.Name}
	}
	ids := make([]uuid.UUID, 0, len(sheets))
	for id := range sheets {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool {
		if sheets[ids[i]].name == sheets[ids[j]].name {
			return ids[i].String() < ids[j].String()
		}
		return sheets[ids[i]].name < sheets[ids[j]].name
	})
	for _, id := range ids {
		table.Review.Divisions = append(table.Review.Divisions, s.divisionWorkbook(id, *sheets[id]))
	}
	return table, nil
}

func (s *oprecStaffService) registrantExportRow(registrant entity.OprecStaff) []string {
	var requiredDocumentsURL, portfolioURL string
	for _, document := range registrant.Documents {
		switch document.Tipe {
		case domain.DocumentTypeRequiredDocuments:
			requiredDocumentsURL = document.URL
		case domain.DocumentTypePortfolio:
			// URL portfolio di database berisi storage key, jadi yang diexport
			// adalah endpoint admin yang tetap butuh sesi untuk dibuka.
			portfolioURL = fmt.Sprintf("%s/api/v1/oprec-staff/%s/documents/%s", s.publicURL, registrant.ID, document.ID)
		}
	}

	var faculty, department string
	if registrant.Fakultas != nil {
		faculty = registrant.Fakultas.Nama
	}
	if registrant.Departemen != nil {
		department = registrant.Departemen.Nama
	}
	willingTransfer := "Tidak"
	if registrant.WillingTransferDivision {
		willingTransfer = "Ya"
	}

	row := []string{
		registrant.Timestamp.CreatedAt.In(export.Jakarta).Format("2006-01-02 15:04:05"),
		registrant.Nama,
		registrant.NRP,
		registrant.Email,
		registrant.NomorTelepon,
		stringValue(registrant.LineID),
		faculty,
		department,
		strconv.Itoa(int(registrant.Angkatan)),
		registrant.TentangPetrolida,
		willingTransfer,
	}

	// Dipetakan berdasarkan nomor pilihan, bukan posisi slice, supaya pilihan
	// yang tidak ada tetap menghasilkan sel kosong tanpa menggeser kolom.
	choices := make(map[int16]entity.OprecStaffChoice, len(registrant.Choices))
	for _, choice := range registrant.Choices {
		choices[choice.Pilihan] = choice
	}
	for _, priority := range []int16{1, 2} {
		row = append(row, choiceExportCells(choices[priority])...)
	}

	row = append(row,
		requiredDocumentsURL,
		portfolioURL,
		string(registrant.Status),
		subdivisionName(registrant.CurrentSubdivision),
		subdivisionName(registrant.AcceptedDivisi),
		strconv.FormatBool(registrant.PrimaryRejected),
		strconv.FormatBool(registrant.SecondaryRejected),
	)
	return append(row, additionalExportCells(registrant)...)
}

func choiceExportCells(choice entity.OprecStaffChoice) []string {
	var division, subdivision string
	if choice.Subdivisi != nil {
		subdivision = choice.Subdivisi.Nama
		if choice.Subdivisi.Divisi != nil {
			division = choice.Subdivisi.Divisi.Nama
		}
	}
	return []string{
		division,
		subdivision,
		choice.PemahamanDivisi,
		choice.AlasanBergabung,
		choice.IdeInovasi,
		stringValue(choice.Pengalaman),
	}
}

func subdivisionName(subdivision *entity.Subdivisi) string {
	if subdivision == nil {
		return ""
	}
	return subdivision.Nama
}

func stringValue(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}
