package api

import (
	"aexon/internal/db"
	"aexon/internal/types"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

// ensure we have uploads directory
func init() {
	os.MkdirAll("uploads/logos", 0755)
}

func RegisterBrandingRoutes(r *gin.RouterGroup) {
	branding := r.Group("/branding")
	branding.Use(checkProPlanMiddleware())
	{
		branding.POST("/upload-logo", UploadLogo)
		branding.GET("/settings", GetBranding)
		branding.PUT("/settings", UpdateBranding)
	}
}

func checkProPlanMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Mock entitlement check
		// In a real app, we check c.Get("role") or query DB for subscription status
		// For MVP: allow all authenticated users (or just check existence of user_id)
		userID := c.GetString("user_id")
		if userID == "" {
			c.AbortWithStatusJSON(401, gin.H{"error": "Unauthorized"})
			return
		}
		// Assume "Pro"
		c.Next()
	}
}

// Helper to reliably get integer user ID (mocked for MVP since auth uses string IDs)
func getUserIDInt(c *gin.Context) int {
	// For MVP, we map admin-001 to 1, or just hash it.
	// Since the DB requires INT, and Auth uses UUID/String, we face a mismatch.
	// We'll hardcode 1 for the demo purpose if ID is "admin-001".
	uidStr := c.GetString("user_id")
	if uidStr == "admin-001" {
		return 1
	}
	// Fallback or hash implementation could go here
	return 1
}

func UploadLogo(c *gin.Context) {
	file, err := c.FormFile("logo")
	if err != nil {
		c.JSON(400, gin.H{"error": "No file uploaded"})
		return
	}

	// Validation
	if file.Size > 2*1024*1024 {
		c.JSON(400, gin.H{"error": "File size exceeds 2MB"})
		return
	}
	
	ext := strings.ToLower(filepath.Ext(file.Filename))
	validExt := map[string]bool{".png": true, ".jpg": true, ".jpeg": true, ".svg": true}
	if !validExt[ext] {
		c.JSON(400, gin.H{"error": "Invalid file type"})
		return
	}

	userID := getUserIDInt(c)
	filename := fmt.Sprintf("logo_%d_%d%s", userID, time.Now().Unix(), ext)
	// Be careful with Windows paths, but internal references should be relative to CWD
	path := filepath.Join("uploads", "logos", filename)

	if err := c.SaveUploadedFile(file, path); err != nil {
		c.JSON(500, gin.H{"error": "Failed to save file"})
		return
	}

	// Update DB
	// First get existing to keep other fields
	settings, err := db.GetBrandingSettings(userID)
	if err != nil {
		// If fails, maybe creates new with defaults
		settings = &types.BrandingSettings{UserID: userID}
	}
	// Normalize path for URL serving (forward slashes)
	settings.LogoURL = "/uploads/logos/" + filename
	
	if err := db.UpsertBrandingSettings(settings); err != nil {
		c.JSON(500, gin.H{"error": "Database error"})
		return
	}

	c.JSON(200, gin.H{"url": settings.LogoURL})
}

func GetBranding(c *gin.Context) {
	userID := getUserIDInt(c)
	settings, err := db.GetBrandingSettings(userID)
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	c.JSON(200, settings)
}

func UpdateBranding(c *gin.Context) {
	var req struct {
		PrimaryColor  string `json:"primary_color"`
		HidePoweredBy bool   `json:"hide_powered_by"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": "Invalid JSON"})
		return
	}

	userID := getUserIDInt(c)
	settings, err := db.GetBrandingSettings(userID)
	if err != nil {
		settings = &types.BrandingSettings{UserID: userID} // Reset if new
	}

	settings.PrimaryColor = req.PrimaryColor
	settings.HidePoweredBy = req.HidePoweredBy
	settings.UserID = userID // ensure ID is set
	
	if err := db.UpsertBrandingSettings(settings); err != nil {
		c.JSON(500, gin.H{"error": "Database error"})
		return
	}

	c.JSON(200, settings)
}
