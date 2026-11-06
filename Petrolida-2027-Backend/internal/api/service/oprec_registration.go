package service

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/mail"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strings"
	"time"

	"github.com/Petrolida/Petrolida-2027-Backend/internal/domain"
	"github.com/Petrolida/Petrolida-2027-Backend/internal/entity"
	myerror "github.com/Petrolida/Petrolida-2027-Backend/internal/pkg/error"
	"github.com/Petrolida/Petrolida-2027-Backend/internal/pkg/utils"
	"github.com/google/uuid"
)

// submissionLookupTimeout membatasi pencarian pendaftaran lewat submission key.
const submissionLookupTimeout = 8 * time.Second

var (
	nrpPattern           = regexp.MustCompile(`^[A-Za-z0-9.-]{6,30}$`)
	phonePattern         = regexp.MustCompile(`^\+?[0-9]{9,16}$`)
	portfolioNamePattern = regexp.MustCompile(`[^A-Za-z0-9]+`)
)

// validatedRegistration adalah input pendaftaran yang sudah dinormalisasi dan lolos validasi.
type validatedRegistration struct {
	Input          domain.RegistrationInput
	FacultyID      uuid.UUID
	DepartmentID   uuid.UUID
	SubdivisionIDs []uuid.UUID
}

func (s *oprecStaffService) Register(
	input domain.RegistrationInput,
	portfolio *multipart.FileHeader,
) (domain.RegistrationResult, error) {
	key, fingerprint, err := s.submissionIdentity(input, portfolio)
	if err != nil {
		return domain.RegistrationResult{}, err
	}
	if key != nil {
		if result, found, err := s.recoverRegistration(*key, *fingerprint); found || err != nil {
			return result, err
		}
	}
	if !s.config.RegistrationOpen(s.now()) {
		return domain.RegistrationResult{}, domain.ErrRegistrationClosed
	}

	validated, err := s.validateInput(input)
	if err != nil {
		return domain.RegistrationResult{}, err
	}
	subdivisions, err := s.validateReferences(validated)
	if err != nil {
		return domain.RegistrationResult{}, err
	}
	if s.requiresPortfolio(subdivisions) && portfolio == nil {
		return domain.RegistrationResult{}, myerror.NewFieldValidation("portfolio", "Portfolio", "portfolio PDF is required for the selected subdivision")
	}

	portfolioDocument, err := s.storePortfolio(portfolio, validated.Input)
	if err != nil {
		return domain.RegistrationResult{}, err
	}

	registrant := newRegistrant(validated, key, fingerprint)
	choices := buildChoices(validated.Input.Choices)
	skills := buildSkillEntity(validated.Input.Skills)
	additionalInfo, err := buildAdditionalInformationEntity(validated.Input.AdditionalInfo)
	if err != nil {
		return domain.RegistrationResult{}, err
	}
	documents := buildDocuments(validated.Input.RequiredDocumentURL, portfolioDocument)

	if err := s.repository.Create(&registrant, choices, skills, additionalInfo, documents); err != nil {
		if portfolioDocument != nil {
			_ = s.storage.Delete(portfolioDocument.URL)
		}
		// A concurrent retry may have committed while this request was uploading.
		if key != nil {
			if result, found, recoveryErr := s.recoverRegistration(*key, *fingerprint); found || recoveryErr != nil {
				return result, recoveryErr
			}
		}
		return domain.RegistrationResult{}, err
	}
	return domain.RegistrationResult{RegistrationID: registrant.ID, Status: string(registrant.Status)}, nil
}

// submissionIdentity mengembalikan submission key dan fingerprint pendaftaran.
// Keduanya nil kalau client tidak mengirim submission key.
func (s *oprecStaffService) submissionIdentity(
	input domain.RegistrationInput,
	portfolio *multipart.FileHeader,
) (*uuid.UUID, *string, error) {
	if input.SubmissionKey == "" {
		return nil, nil, nil
	}
	key, err := parseSubmissionKey(input.SubmissionKey)
	if err != nil {
		return nil, nil, err
	}
	fingerprint, err := s.registrationFingerprint(input, portfolio)
	if err != nil {
		return nil, nil, err
	}
	return &key, &fingerprint, nil
}

func parseSubmissionKey(value string) (uuid.UUID, error) {
	key, err := uuid.Parse(value)
	if err != nil || key.Version() != 4 || key.Variant() != uuid.RFC4122 {
		return uuid.Nil, myerror.NewFieldValidation("submission_key", "Submission", "must use a random UUID")
	}
	return key, nil
}

func (s *oprecStaffService) ConfirmRegistration(ctx context.Context, submissionKey string) (domain.RegistrationConfirmation, error) {
	key, err := parseSubmissionKey(submissionKey)
	if err != nil {
		return domain.RegistrationConfirmation{}, err
	}
	ctx, cancel := context.WithTimeout(ctx, submissionLookupTimeout)
	defer cancel()
	registrant, err := s.repository.GetBySubmissionKey(ctx, key)
	if errors.Is(err, domain.ErrRegistrantNotFound) {
		// Absence does not prove failure: the original request may still be running.
		return domain.RegistrationConfirmation{State: "unconfirmed"}, nil
	}
	if err != nil {
		return domain.RegistrationConfirmation{}, err
	}
	return domain.RegistrationConfirmation{State: "confirmed", Registration: &domain.RegistrationResult{
		RegistrationID: registrant.ID, Status: string(entity.OprecStaffStatusPending),
	}}, nil
}

func (s *oprecStaffService) recoverRegistration(key uuid.UUID, fingerprint string) (domain.RegistrationResult, bool, error) {
	ctx, cancel := context.WithTimeout(context.Background(), submissionLookupTimeout)
	defer cancel()
	registrant, err := s.repository.GetBySubmissionKey(ctx, key)
	if errors.Is(err, domain.ErrRegistrantNotFound) {
		return domain.RegistrationResult{}, false, nil
	}
	if err != nil {
		return domain.RegistrationResult{}, false, err
	}
	if registrant.SubmissionFingerprint == nil || *registrant.SubmissionFingerprint != fingerprint {
		return domain.RegistrationResult{}, true, domain.ErrSubmissionKeyConflict
	}
	return domain.RegistrationResult{RegistrationID: registrant.ID, Status: string(entity.OprecStaffStatusPending)}, true, nil
}

func (s *oprecStaffService) registrationFingerprint(input domain.RegistrationInput, portfolio *multipart.FileHeader) (string, error) {
	input.SubmissionKey = ""
	payload, err := json.Marshal(input)
	if err != nil {
		return "", err
	}
	hash := sha256.New()
	_, _ = hash.Write(payload)
	if portfolio == nil {
		_, _ = hash.Write([]byte("\x00no-portfolio"))
	} else {
		if portfolio.Size <= 0 || portfolio.Size > s.config.PortfolioMaxBytes {
			return "", myerror.NewFieldValidation("portfolio", "Portfolio", "file size exceeds the permitted upload limit")
		}
		file, err := portfolio.Open()
		if err != nil {
			return "", err
		}
		defer file.Close()
		_, _ = hash.Write([]byte("\x00portfolio\x00" + portfolio.Filename + "\x00"))
		if _, err := io.Copy(hash, file); err != nil {
			return "", err
		}
	}
	return hex.EncodeToString(hash.Sum(nil)), nil
}

func (s *oprecStaffService) validateInput(input domain.RegistrationInput) (validatedRegistration, error) {
	input.Name = strings.TrimSpace(input.Name)
	input.Email = strings.ToLower(strings.TrimSpace(input.Email))
	input.NRP = strings.ToUpper(strings.TrimSpace(input.NRP))
	input.PhoneNumber = strings.ReplaceAll(strings.TrimSpace(input.PhoneNumber), " ", "")
	input.LineID = strings.TrimSpace(input.LineID)
	input.AboutPetrolida = strings.TrimSpace(input.AboutPetrolida)
	input.RequiredDocumentURL = strings.TrimSpace(input.RequiredDocumentURL)
	input.Skills.DesignApplications = strings.TrimSpace(input.Skills.DesignApplications)
	input.AdditionalInfo.ConnectedCompany = strings.TrimSpace(input.AdditionalInfo.ConnectedCompany)
	for _, value := range skillFields(&input.Skills) {
		*value = strings.TrimSpace(*value)
	}

	fields := make([]myerror.FieldError, 0)
	add := func(field, label, message string) {
		fields = append(fields, myerror.FieldError{Field: field, Label: label, Message: message})
	}
	if len(input.Name) < 3 || len(input.Name) > 150 {
		add("name", "Name", "must contain 3 to 150 characters")
	}
	if address, err := mail.ParseAddress(input.Email); err != nil || !strings.EqualFold(address.Address, input.Email) {
		add("email", "Email", "must be a valid email address")
	}
	if !nrpPattern.MatchString(input.NRP) {
		add("nrp", "NRP", "must contain 6 to 30 letters or numbers")
	}
	if !phonePattern.MatchString(input.PhoneNumber) {
		add("phone_number", "Phone number", "must contain 9 to 16 digits")
	}
	if !slices.Contains(s.config.AllowedBatches, input.Batch) {
		add("batch", "Batch", "is not allowed")
	}
	if input.AboutPetrolida == "" || len(input.AboutPetrolida) > 5000 {
		add("about_petrolida", "About Petrolida", "is required and must not exceed 5000 characters")
	}
	if input.WillingTransferDivision == nil {
		add("willing_transfer_division", "Willing to transfer division", "must be answered with yes or no")
	}
	for field, value := range skillFields(&input.Skills) {
		if !entity.OprecStaffSkillLevel(*value).IsValid() {
			add("skills."+field, "Skill level", "must be one of no_proficiency, poor, decent, good, or very_good")
		}
	}
	if input.Skills.DesignApplications == "" {
		add("skills.design_applications", "Design applications", "is required")
	}
	for field, value := range additionalInformationValues(input.AdditionalInfo) {
		if value == nil {
			add("additional_info."+field, "Additional information", "must be answered")
		}
	}
	if input.AdditionalInfo.ConnectedCompany == "" {
		add("additional_info.connected_company", "Connected company", "is required")
	}
	if !utils.IsGoogleDriveURL(input.RequiredDocumentURL) {
		add("required_document_url", "Required documents", "must be an HTTPS Google Drive folder link")
	}

	facultyID, err := uuid.Parse(strings.TrimSpace(input.FacultyID))
	if err != nil {
		add("faculty_id", "Faculty", "must be a valid selection")
	}
	departmentID, err := uuid.Parse(strings.TrimSpace(input.DepartmentID))
	if err != nil {
		add("department_id", "Department", "must be a valid selection")
	}

	if len(input.Choices) != 2 {
		add("choices", "Division choices", "must contain exactly primary and secondary choices")
	}
	priorities := map[int16]bool{}
	subdivisionSet := map[uuid.UUID]bool{}
	subdivisionIDs := make([]uuid.UUID, 0, 2)
	for index := range input.Choices {
		choice := &input.Choices[index]
		choice.AboutDivision = strings.TrimSpace(choice.AboutDivision)
		choice.ReasonToJoin = strings.TrimSpace(choice.ReasonToJoin)
		choice.InnovationIdea = strings.TrimSpace(choice.InnovationIdea)
		choice.PreviousExperience = strings.TrimSpace(choice.PreviousExperience)
		prefix := fmt.Sprintf("choices.%d", index)
		if choice.Priority != 1 && choice.Priority != 2 {
			add(prefix+".priority", "Choice priority", "must be 1 or 2")
		}
		if priorities[choice.Priority] {
			add(prefix+".priority", "Choice priority", "must be unique")
		}
		priorities[choice.Priority] = true
		id, parseErr := uuid.Parse(strings.TrimSpace(choice.SubdivisionID))
		if parseErr != nil {
			add(prefix+".subdivision_id", "Subdivision", "must be a valid selection")
		} else {
			if subdivisionSet[id] {
				add(prefix+".subdivision_id", "Subdivision", "primary and secondary choices must differ")
			}
			subdivisionSet[id] = true
			subdivisionIDs = append(subdivisionIDs, id)
		}
		if choice.AboutDivision == "" {
			add(prefix+".about_division", "Division understanding", "is required")
		}
		if choice.ReasonToJoin == "" {
			add(prefix+".reason_to_join", "Reason to join", "is required")
		}
		if choice.InnovationIdea == "" {
			add(prefix+".innovation_idea", "Innovation idea", "is required")
		}
	}
	if len(fields) > 0 {
		return validatedRegistration{Input: input}, myerror.NewValidation(fields)
	}
	sort.Slice(input.Choices, func(i, j int) bool { return input.Choices[i].Priority < input.Choices[j].Priority })
	return validatedRegistration{
		Input:          input,
		FacultyID:      facultyID,
		DepartmentID:   departmentID,
		SubdivisionIDs: subdivisionIDs,
	}, nil
}

// validateReferences memastikan departemen dan subdivisi pilihan ada di database,
// lalu menerjemahkan error repository menjadi validation error untuk client.
func (s *oprecStaffService) validateReferences(validated validatedRegistration) (map[uuid.UUID]entity.Subdivisi, error) {
	subdivisions, err := s.repository.ValidateReferences(validated.FacultyID, validated.DepartmentID, validated.SubdivisionIDs)
	switch {
	case errors.Is(err, domain.ErrDepartmentIdNotFound):
		return nil, myerror.NewFieldValidation("department_id", "Department", "does not belong to the selected faculty")
	case errors.Is(err, domain.ErrSubdivsionIdNotFound):
		return nil, myerror.NewFieldValidation("choices", "Division choices", "contain an unavailable subdivision")
	case err != nil:
		return nil, err
	}
	return subdivisions, nil
}

// requiresPortfolio bernilai true kalau salah satu subdivisi pilihan mewajibkan portfolio.
func (s *oprecStaffService) requiresPortfolio(subdivisions map[uuid.UUID]entity.Subdivisi) bool {
	for _, subdivision := range subdivisions {
		if utils.ContainsFold(s.config.PortfolioRequired, subdivision.Nama) {
			return true
		}
	}
	return false
}

// storePortfolio menyimpan file portfolio dan mengembalikan dokumennya.
// Mengembalikan nil kalau pendaftar tidak mengunggah portfolio.
func (s *oprecStaffService) storePortfolio(
	portfolio *multipart.FileHeader,
	input domain.RegistrationInput,
) (*entity.OprecStaffDocument, error) {
	if portfolio == nil {
		return nil, nil
	}
	key, mimeType, err := s.savePortfolio(portfolio, portfolioFileName(input.NRP, input.Name))
	if err != nil {
		return nil, err
	}
	name := filepath.Base(portfolio.Filename)
	size := portfolio.Size
	return &entity.OprecStaffDocument{
		Tipe:         domain.DocumentTypePortfolio,
		URL:          key,
		OriginalName: &name,
		MimeType:     &mimeType,
		SizeBytes:    &size,
	}, nil
}

func (s *oprecStaffService) savePortfolio(fileHeader *multipart.FileHeader, name string) (string, string, error) {
	if fileHeader.Size <= 0 || fileHeader.Size > s.config.PortfolioMaxBytes {
		return "", "", myerror.NewFieldValidation("portfolio", "Portfolio", fmt.Sprintf("must not exceed %d bytes", s.config.PortfolioMaxBytes))
	}
	if !strings.EqualFold(filepath.Ext(fileHeader.Filename), ".pdf") {
		return "", "", myerror.NewFieldValidation("portfolio", "Portfolio", "must be a PDF file")
	}
	file, err := fileHeader.Open()
	if err != nil {
		return "", "", err
	}
	defer file.Close()

	header := make([]byte, 512)
	read, readErr := file.Read(header)
	if readErr != nil && read == 0 {
		return "", "", myerror.NewFieldValidation("portfolio", "Portfolio", "could not be read")
	}
	header = header[:read]
	mimeType := http.DetectContentType(header)
	if !bytes.HasPrefix(header, []byte("%PDF-")) || mimeType != "application/pdf" {
		return "", "", myerror.NewFieldValidation("portfolio", "Portfolio", "file content must be a valid PDF")
	}
	if _, err := file.Seek(0, 0); err != nil {
		return "", "", err
	}
	key, err := s.storage.Save(name, fileHeader.Size, file)
	return key, mimeType, err
}

// portfolioFileName membuat nama berkas yang enak dibaca panitia saat membuka
// folder Drive, karena di sana nama inilah yang terlihat, bukan UUID. Penyimpanan
// lokal mengabaikannya dan tetap memakai nama acak.
func portfolioFileName(nrp, name string) string {
	clean := func(value string) string {
		return strings.Trim(portfolioNamePattern.ReplaceAllString(value, "_"), "_")
	}
	nrp, name = clean(nrp), clean(name)
	if len(name) > 60 {
		name = strings.Trim(name[:60], "_")
	}
	switch {
	case nrp == "" && name == "":
		return "portfolio.pdf"
	case nrp == "":
		return name + ".pdf"
	case name == "":
		return nrp + ".pdf"
	default:
		return nrp + "_" + name + ".pdf"
	}
}
