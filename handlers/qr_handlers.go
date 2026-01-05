package handlers

import (
	"bytes"
	"fmt"
	"image"
	"image/jpeg"
	"image/png"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/fogleman/gg"
	"github.com/labstack/echo/v4"
	"github.com/nfnt/resize"
	"github.com/skip2/go-qrcode"
)

// GenerateQRHandler generates a QR code image for receiving payments
func GenerateQRHandler(c echo.Context) error {
	var req struct {
		Name            string  `json:"name"`
		AccountNoMasked string  `json:"account_no_masked"`
		QRPayload       string  `json:"qr_payload"`
		Amount          float64 `json:"amount,omitempty"`
		Title           string  `json:"title,omitempty"`
	}
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Invalid request body"})
	}

	// 1. Generate QR Code Image
	qr, err := qrcode.New(req.QRPayload, qrcode.Medium)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to generate QR code"})
	}
	// Background white for QR, but we will draw it on our own canvas
	qrImg := qr.Image(600) 

	// 2. Setup GG Context (Layout)
	const width = 800
	const height = 1100
	dc := gg.NewContext(width, height)

	// Background - white
	dc.SetRGB(1, 1, 1)
	dc.Clear()

	// Load Fonts
	fontPath := "./assets/fonts/Sarabun.ttf"
	if _, err := os.Stat(fontPath); os.IsNotExist(err) {
		fontPath = "/app/assets/fonts/Sarabun.ttf"
	}
	if _, err := os.Stat(fontPath); os.IsNotExist(err) {
		fontPath = "/System/Library/Fonts/Supplemental/Arial Unicode.ttf"
	}
	
	boldFontPath := "./assets/fonts/Sarabun Bold.ttf"
	if _, err := os.Stat(boldFontPath); os.IsNotExist(err) {
		boldFontPath = "/app/assets/fonts/Sarabun Bold.ttf"
	}
	if _, err := os.Stat(boldFontPath); os.IsNotExist(err) {
		boldFontPath = fontPath
	}

	drawTextCentered := func(text string, y float64, size float64, colorRGB [3]float64, bold bool) {
		usePath := fontPath
		if bold {
			usePath = boldFontPath
		}
		dc.LoadFontFace(usePath, size)
		dc.SetRGB(colorRGB[0], colorRGB[1], colorRGB[2])
		tw, _ := dc.MeasureString(text)
		dc.DrawString(text, (float64(width)-tw)/2, y)
	}

	primaryGreen := [3]float64{0.0, 0.424, 0.278}   // #006C47
	textPrimary := [3]float64{0.129, 0.129, 0.129}  // #212121
	textSecondary := [3]float64{0.459, 0.459, 0.459} // #757575

	// 3. Draw Header Logo
	yPos := 60.0
	logoPath := "./assets/pic/logoCoop.jpg"
	if _, err := os.Stat(logoPath); os.IsNotExist(err) {
		logoPath = "/app/assets/pic/logoCoop.jpg"
	}
	
	logoSize := uint(100)
	logoLoaded := false
	if logoFile, err := os.Open(logoPath); err == nil {
		defer logoFile.Close()
		if logoImg, err := jpeg.Decode(logoFile); err == nil {
			// Get original dimensions
			bounds := logoImg.Bounds()
			imgW := bounds.Dx()
			imgH := bounds.Dy()
			
			// Crop to square (center crop)
			var squareImg image.Image
			if imgW > imgH {
				// Landscape - crop width
				offset := (imgW - imgH) / 2
				squareImg = logoImg.(interface {
					SubImage(r image.Rectangle) image.Image
				}).SubImage(image.Rect(offset, 0, offset+imgH, imgH))
			} else if imgH > imgW {
				// Portrait - crop height
				offset := (imgH - imgW) / 2
				squareImg = logoImg.(interface {
					SubImage(r image.Rectangle) image.Image
				}).SubImage(image.Rect(0, offset, imgW, offset+imgW))
			} else {
				// Already square
				squareImg = logoImg
			}

			// Resize to exact square size
			resizedLogo := resize.Resize(logoSize, logoSize, squareImg, resize.Lanczos3)
			
			// Create circular mask
			logoCtx := gg.NewContext(int(logoSize), int(logoSize))
			logoCtx.DrawCircle(float64(logoSize)/2.0, float64(logoSize)/2.0, float64(logoSize)/2.0)
			logoCtx.Clip()
			logoCtx.DrawImage(resizedLogo, 0, 0)
			
			dc.DrawImage(logoCtx.Image(), (width-int(logoSize))/2, int(yPos))
			logoLoaded = true
		}
	}
	if !logoLoaded {
		dc.SetRGB(primaryGreen[0], primaryGreen[1], primaryGreen[2])
		dc.DrawCircle(float64(width)/2, yPos+float64(logoSize)/2, float64(logoSize)/2)
		dc.Fill()
	}

	yPos += float64(logoSize) + 40
	title := req.Title
	if title == "" {
		title = "QR รับเงิน"
	}
	drawTextCentered(title, yPos, 38, primaryGreen, true)

	// 4. Draw QR Code
	yPos += 40
	dc.DrawImage(qrImg, (width-600)/2, int(yPos))
	yPos += 600 + 40

	// 5. Draw Account Info
	drawTextCentered(req.Name, yPos, 34, textPrimary, true)
	yPos += 45
	drawTextCentered(req.AccountNoMasked, yPos, 28, textSecondary, false)

	// 6. Draw Amount (if exists)
	if req.Amount > 0 {
		yPos += 80
		// Rounded Box for Amount
		dc.SetRGB(0.96, 0.96, 0.96)
		boxW := 400.0
		boxH := 100.0
		dc.DrawRoundedRectangle((float64(width)-boxW)/2, yPos-65, boxW, boxH, 15)
		dc.Fill()
		
		amountStr := fmt.Sprintf("%.2f บาท", req.Amount)
		drawTextCentered(amountStr, yPos, 42, primaryGreen, true)
	}

	// 7. Save to Storage
	storageDir := os.Getenv("STORAGE_DIR")
	if storageDir == "" {
		storageDir = "./storage"
	}
	qrsDir := fmt.Sprintf("%s/qrs", storageDir)
	if err := os.MkdirAll(qrsDir, 0755); err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to create storage dir"})
	}

	filename := fmt.Sprintf("qr_%d.png", time.Now().UnixNano())
	filepath := fmt.Sprintf("%s/%s", qrsDir, filename)

	var buf bytes.Buffer
	if err := png.Encode(&buf, dc.Image()); err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to encode PNG"})
	}
	
	if err := os.WriteFile(filepath, buf.Bytes(), 0644); err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to write file"})
	}

	// 8. Return URL
	publicUrlBase := os.Getenv("STORAGE_PUBLIC_URL")
	if publicUrlBase == "" {
		// Fallback for local dev
		publicUrlBase = "http://localhost:8080/storage"
	}
	// Ensure no double slash
	publicUrlBase = strings.TrimSuffix(publicUrlBase, "/")
	publicUrl := fmt.Sprintf("%s/qrs/%s", publicUrlBase, filename)

	return c.JSON(http.StatusOK, map[string]interface{}{
		"status": "success",
		"url":    publicUrl,
	})
}

// DeleteQRHandler deletes a generated QR code
func DeleteQRHandler(c echo.Context) error {
	var req struct {
		URL string `json:"url"`
	}
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Invalid request"})
	}

	if req.URL == "" {
		return c.JSON(http.StatusOK, map[string]string{"status": "nothing_to_delete"})
	}

	// Extract filename from URL
	parts := strings.Split(req.URL, "/")
	if len(parts) == 0 {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Invalid URL"})
	}
	filename := parts[len(parts)-1]

	// Security check: ensure it's a qr_*.png file
	if !strings.HasPrefix(filename, "qr_") || !strings.HasSuffix(filename, ".png") {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Unauthorized delete request"})
	}

	storageDir := os.Getenv("STORAGE_DIR")
	if storageDir == "" {
		storageDir = "./storage"
	}
	filepath := fmt.Sprintf("%s/qrs/%s", storageDir, filename)

	if err := os.Remove(filepath); err != nil {
		return c.JSON(http.StatusOK, map[string]string{"status": "deleted_or_not_found", "error": err.Error()})
	}

	return c.JSON(http.StatusOK, map[string]string{"status": "success"})
}
