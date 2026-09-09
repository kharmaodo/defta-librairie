package services

import (
	"bytes"
	"context"
	"defta-librairie/internal/auth"
	"defta-librairie/internal/identity"
	"defta-librairie/internal/models"
	"defta-librairie/internal/repositories"
	"encoding/base64"
	"errors"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"net/mail"
	"strings"
	"time"
	"unicode/utf8"
)

var ErrInvalidSettings = errors.New("invalid library settings")

type LibrarySettingsService struct {
	repository *repositories.LibrarySettingsRepository
}

func NewLibrarySettingsService(r *repositories.LibrarySettingsRepository) *LibrarySettingsService {
	return &LibrarySettingsService{repository: r}
}
func (s *LibrarySettingsService) Find(ctx context.Context, c *auth.Claims, id string) (models.LibrarySettings, error) {
	if c == nil {
		return models.LibrarySettings{}, ErrBookForbidden
	}
	scope, err := resolveBookScope(c, strings.TrimSpace(id), true)
	if err != nil {
		return models.LibrarySettings{}, err
	}
	return s.repository.Find(ctx, scope, c.Role == models.RoleOwnerLibrary)
}
func (s *LibrarySettingsService) Update(ctx context.Context, c *auth.Claims, id string, v models.LibrarySettings) error {
	current, err := s.Find(ctx, c, id)
	if err != nil {
		return err
	}
	if c.Subject == "" {
		return ErrBookForbidden
	}
	if v.Version < 1 || v.Currency != "XOF" || v.DefaultLowStockThreshold < 0 || v.DefaultLowStockThreshold > 1000000 {
		return ErrInvalidSettings
	}
	for _, p := range []struct {
		value *string
		max   int
	}{{&v.Address, 500}, {&v.Phone, 40}, {&v.Email, 254}, {&v.PrintFooter, 500}} {
		*p.value = strings.TrimSpace(*p.value)
		if !utf8.ValidString(*p.value) || utf8.RuneCountInString(*p.value) > p.max {
			return ErrInvalidSettings
		}
	}
	if v.Email != "" {
		a, e := mail.ParseAddress(v.Email)
		if e != nil || a.Address != v.Email {
			return ErrInvalidSettings
		}
	}
	if v.LogoData != "" {
		parts := strings.SplitN(v.LogoData, ",", 2)
		if len(parts) != 2 || (parts[0] != "data:image/png;base64" && parts[0] != "data:image/jpeg;base64") {
			return ErrInvalidSettings
		}
		if len(parts[1]) > 180000 {
			return ErrInvalidSettings
		}
		b, e := base64.StdEncoding.DecodeString(parts[1])
		if e != nil || len(b) > 131072 {
			return ErrInvalidSettings
		}
		cfg, format, e := image.DecodeConfig(bytes.NewReader(b))
		if e != nil || cfg.Width < 1 || cfg.Height < 1 || cfg.Width > 1024 || cfg.Height > 1024 || parts[0] != "data:image/"+format+";base64" {
			return ErrInvalidSettings
		}
		if _, _, e = image.Decode(bytes.NewReader(b)); e != nil {
			return ErrInvalidSettings
		}
	}
	v.LibraryID = current.LibraryID
	v.Name = current.Name
	v.UpdatedAt = time.Now().UTC().Format(time.RFC3339Nano)
	auditID, err := identity.NewID()
	if err != nil {
		return err
	}
	return s.repository.Update(ctx, v, c.Subject, auditID, c.Role == models.RoleOwnerLibrary)
}
