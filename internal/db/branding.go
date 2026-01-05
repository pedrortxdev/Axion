package db

import (
	"aexon/internal/types"
	"context"
	"database/sql"
	"errors"
)

func GetBrandingSettings(userID int) (*types.BrandingSettings, error) {
	query := `
		SELECT id, user_id, logo_url, primary_color, hide_powered_by, created_at, updated_at
		FROM branding_settings
		WHERE user_id = $1
	`
	var s types.BrandingSettings
	err := GetService().QueryRowContext(context.Background(), query, userID).Scan(
		&s.ID, &s.UserID, &s.LogoURL, &s.PrimaryColor, &s.HidePoweredBy, &s.CreatedAt, &s.UpdatedAt,
	)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			// Return default settings if none exist
			return &types.BrandingSettings{
				UserID:       userID,
				PrimaryColor: "#3B82F6",
			}, nil
		}
		return nil, err
	}
	return &s, nil
}

func UpsertBrandingSettings(settings *types.BrandingSettings) error {
	query := `
		INSERT INTO branding_settings (user_id, logo_url, primary_color, hide_powered_by, updated_at)
		VALUES ($1, $2, $3, $4, NOW())
		ON CONFLICT (user_id) DO UPDATE SET
			logo_url = EXCLUDED.logo_url,
			primary_color = EXCLUDED.primary_color,
			hide_powered_by = EXCLUDED.hide_powered_by,
			updated_at = NOW()
	`
	_, err := GetService().ExecContext(context.Background(), query, settings.UserID, settings.LogoURL, settings.PrimaryColor, settings.HidePoweredBy)

	return err
}
